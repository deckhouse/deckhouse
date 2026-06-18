// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// On upgrade from a release where Cluster / MachineHealthCheck / MachineDeployment
// were rendered by the node-manager helm chart to this branch where they are owned
// by node-controller, helm sees the resources are missing from the new manifest and
// schedules them for deletion — which cascades into capi-controller-manager tearing
// down dependent Machines / Nodes. Disastrous.
//
// Detach them from helm ownership by stamping `helm.sh/resource-policy: keep`.
// helm honours this annotation during upgrade and skips the orphaned-resource
// deletion. The hook runs OnBeforeHelm to ensure it fires before helm install/upgrade.

const helmResourcePolicyAnnotation = "helm.sh/resource-policy"

type capiResourceMeta struct {
	APIVersion        string
	Kind              string
	Name              string
	Namespace         string
	HasHelmOwnership  bool
	HasKeepAnnotation bool
}

func filterCapiResourceMeta(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	annotations := obj.GetAnnotations()
	_, hasHelm := annotations["meta.helm.sh/release-name"]
	_, hasKeep := annotations[helmResourcePolicyAnnotation]
	return capiResourceMeta{
		APIVersion:        obj.GetAPIVersion(),
		Kind:              obj.GetKind(),
		Name:              obj.GetName(),
		Namespace:         obj.GetNamespace(),
		HasHelmOwnership:  hasHelm,
		HasKeepAnnotation: hasKeep,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/node-manager/set-keep-policy-on-capi-resources",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "capi_cluster",
			ApiVersion:                   "cluster.x-k8s.io/v1beta1",
			Kind:                         "Cluster",
			ExecuteHookOnSynchronization: go_hook.Bool(false),
			WaitForSynchronization:       go_hook.Bool(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{capiNamespace}},
			},
			FilterFunc: filterCapiResourceMeta,
		},
		{
			Name:                         "capi_machine_health_check",
			ApiVersion:                   "cluster.x-k8s.io/v1beta1",
			Kind:                         "MachineHealthCheck",
			ExecuteHookOnSynchronization: go_hook.Bool(false),
			WaitForSynchronization:       go_hook.Bool(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{capiNamespace}},
			},
			FilterFunc: filterCapiResourceMeta,
		},
		{
			Name:                         "capi_machine_deployment",
			ApiVersion:                   "cluster.x-k8s.io/v1beta1",
			Kind:                         "MachineDeployment",
			ExecuteHookOnSynchronization: go_hook.Bool(false),
			WaitForSynchronization:       go_hook.Bool(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{capiNamespace}},
			},
			FilterFunc: filterCapiResourceMeta,
		},
	},
}, migrateCapiClusterHelmOwnership)

func migrateCapiClusterHelmOwnership(_ context.Context, input *go_hook.HookInput) error {
	for _, snap := range []string{"capi_cluster", "capi_machine_health_check", "capi_machine_deployment"} {
		metas, err := sdkobjectpatch.UnmarshalToStruct[capiResourceMeta](input.Snapshots, snap)
		if err != nil {
			return fmt.Errorf("unmarshal %s snapshot: %w", snap, err)
		}
		for _, m := range metas {
			if !m.HasHelmOwnership || m.HasKeepAnnotation {
				continue
			}
			patch := map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						helmResourcePolicyAnnotation: "keep",
					},
				},
			}
			input.PatchCollector.PatchWithMerge(patch, m.APIVersion, m.Kind, m.Namespace, m.Name)
		}
	}
	return nil
}
