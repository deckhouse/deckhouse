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
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// On upgrade from a release where Cluster / MachineHealthCheck / MachineDeployment
// were rendered by the node-manager helm chart to this branch where they are owned
// by node-controller, helm sees the resources are missing from the new manifest and
// schedules them for deletion — which cascades into capi-controller-manager tearing
// down dependent Machines / Nodes.
//
// Detach them from helm ownership by stamping `helm.sh/resource-policy: keep`.
// The hook runs OnBeforeHelm to ensure it fires before helm install/upgrade.
//
// No Kubernetes bindings: on bootstrap, CRDs don't exist yet and addon-operator
// would fail at discovery. Direct API calls handle missing CRDs gracefully.
//
// TODO(v1beta2): GVR uses v1beta1 for rolling upgrade from CSE 1.73 which still
// serves v1beta1. Switch to v1beta2 once v1beta1 is no longer served.

const (
	helmResourcePolicyAnnotation = "helm.sh/resource-policy"
	capiNamespace                = "d8-cloud-instance-manager"
)

var capiResources = []schema.GroupVersionResource{
	{Group: "cluster.x-k8s.io", Version: "v1beta1", Resource: "clusters"},
	{Group: "cluster.x-k8s.io", Version: "v1beta1", Resource: "machinehealthchecks"},
	{Group: "cluster.x-k8s.io", Version: "v1beta1", Resource: "machinedeployments"},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/node-manager/set-keep-policy-on-capi-resources",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, dependency.WithExternalDependencies(setKeepPolicyOnCapiResources))

func setKeepPolicyOnCapiResources(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("get k8s client: %w", err)
	}
	dynClient := k8sClient.Dynamic()

	patch, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				helmResourcePolicyAnnotation: "keep",
			},
		},
	})

	for _, gvr := range capiResources {
		list, err := dynClient.Resource(gvr).Namespace(capiNamespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			// CRD doesn't exist (bootstrap) or API not available — skip.
			input.Logger.Info("skipping resource", slog.String("resource", gvr.Resource), slog.Any("error", err))
			continue
		}

		for _, item := range list.Items {
			annotations := item.GetAnnotations()
			if annotations == nil {
				continue
			}
			// Only patch resources owned by helm that don't have keep annotation yet.
			if _, hasHelm := annotations["meta.helm.sh/release-name"]; !hasHelm {
				continue
			}
			if annotations[helmResourcePolicyAnnotation] == "keep" {
				continue
			}

			_, err := dynClient.Resource(gvr).Namespace(item.GetNamespace()).Patch(
				context.TODO(),
				item.GetName(),
				types.MergePatchType,
				patch,
				metav1.PatchOptions{},
			)
			if err != nil {
				return fmt.Errorf("patch %s/%s: %w", gvr.Resource, item.GetName(), err)
			}
			input.Logger.Info("stamped keep policy", slog.String("resource", gvr.Resource), slog.String("name", item.GetName()))
		}
	}

	return nil
}
