/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	capiMachineDeploymentAPIVersion = "cluster.x-k8s.io/v1beta2"
	capiInfrastructureAPIGroup      = "infrastructure.cluster.x-k8s.io"
)

var capiMachineTemplateAPIGroups = map[string]string{
	"DynamixMachineTemplate":     capiInfrastructureAPIGroup,
	"HuaweiCloudMachineTemplate": capiInfrastructureAPIGroup,
	"VCDMachineTemplate":         capiInfrastructureAPIGroup,
	"ZvirtMachineTemplate":       capiInfrastructureAPIGroup,
	"DeckhouseMachineTemplate":   capiInfrastructureAPIGroup,
	"StaticMachineTemplate":      capiInfrastructureAPIGroup,
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/set_api_version_on_machine_deployment",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "capi_mds",
			ApiVersion:             capiMachineDeploymentAPIVersion,
			Kind:                   "MachineDeployment",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: capiInfrastructureRefFilter,
		},
	},
}, handleSetInfrastructureAPIVersion)

type machineDeploymentInfrastructureRef struct {
	Name      string
	Namespace string
	Kind      string
	APIGroup  string
}

func capiInfrastructureRefFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	kind, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "infrastructureRef", "kind")
	apiGroup, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "infrastructureRef", "apiGroup")

	return machineDeploymentInfrastructureRef{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Kind:      kind,
		APIGroup:  apiGroup,
	}, nil
}

func handleSetInfrastructureAPIVersion(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("capi_mds")

	for md, err := range sdkobjectpatch.SnapshotIter[machineDeploymentInfrastructureRef](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over MachineDeployment snapshots: %w", err)
		}

		apiGroup, ok := capiMachineTemplateAPIGroups[md.Kind]
		if !ok {
			input.Logger.Warn("unknown infrastructure template kind", slog.String("machinedeployment", md.Name), slog.String("kind", md.Kind))
			continue
		}

		if md.APIGroup == apiGroup {
			continue
		}

		if md.APIGroup != "" {
			input.Logger.Debug("infrastructureRef.apiGroup already set", slog.String("machinedeployment", md.Name), slog.String("apiGroup", md.APIGroup))
			continue
		}

		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"infrastructureRef": map[string]interface{}{
							"apiGroup": apiGroup,
						},
					},
				},
			},
		}

		input.PatchCollector.PatchWithMerge(patch, capiMachineDeploymentAPIVersion, "MachineDeployment", md.Namespace, md.Name)
	}

	return nil
}
