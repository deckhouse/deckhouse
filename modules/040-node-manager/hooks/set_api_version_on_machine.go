/*
Copyright 2024 Flant JSC

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
	capiMachineAPIVersion = "cluster.x-k8s.io/v1beta2"
)

var capiMachineAPIGroups = map[string]string{
	"DynamixMachine":     capiInfrastructureAPIGroup,
	"HuaweiCloudMachine": capiInfrastructureAPIGroup,
	"VCDMachine":         capiInfrastructureAPIGroup,
	"ZvirtMachine":       capiInfrastructureAPIGroup,
	"DeckhouseMachine":   capiInfrastructureAPIGroup,
	"StaticMachine":      capiInfrastructureAPIGroup,
}

type machineInfrastructureRef struct {
	Name      string
	Namespace string
	Kind      string
	APIGroup  string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/set_api_version_on_machine",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "capi_machines",
			ApiVersion:             capiMachineAPIVersion,
			Kind:                   "Machine",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: capiMachineInfrastructureRefFilter,
		},
	},
}, handleSetMachineInfrastructureAPIVersion)

func capiMachineInfrastructureRefFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	kind, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "kind")
	apiGroup, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "apiGroup")

	return machineInfrastructureRef{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Kind:      kind,
		APIGroup:  apiGroup,
	}, nil
}

func handleSetMachineInfrastructureAPIVersion(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("capi_machines")

	for machine, err := range sdkobjectpatch.SnapshotIter[machineInfrastructureRef](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over Machine snapshots: %w", err)
		}

		apiGroup, ok := capiMachineAPIGroups[machine.Kind]
		if !ok {
			input.Logger.Warn("unknown infrastructure machine kind", slog.String("machine", machine.Name), slog.String("kind", machine.Kind))
			continue
		}

		if machine.APIGroup == apiGroup {
			continue
		}

		if machine.APIGroup != "" {
			input.Logger.Debug("infrastructureRef.apiGroup already set", slog.String("machine", machine.Name), slog.String("apiGroup", machine.APIGroup))
			continue
		}

		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"infrastructureRef": map[string]interface{}{
					"apiGroup": apiGroup,
				},
			},
		}

		input.PatchCollector.PatchWithMerge(patch, capiMachineAPIVersion, "Machine", machine.Namespace, machine.Name)
	}

	return nil
}
