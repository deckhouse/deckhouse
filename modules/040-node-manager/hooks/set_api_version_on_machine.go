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
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/capi/v1beta1"
)

const (
	capiMachineAPIVersion = "cluster.x-k8s.io/v1beta1"
)

var capiMachineVersions = map[string]string{
	"DynamixMachine":     "v1alpha1",
	"HuaweiCloudMachine": "v1alpha1",
	"VCDMachine":         "v1beta2",
	"ZvirtMachine":       "v1",
	"DeckhouseMachine":   "v1alpha1",
	"StaticMachine":      "v1alpha1",
}


type machineInfrastructureRef struct {
	Name       string
	Namespace  string
	Kind       string
	APIVersion string
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
	var machine v1beta1.Machine

	if err := sdk.FromUnstructured(obj, &machine); err != nil {
		return nil, err
	}

	infra := machine.Spec.InfrastructureRef

	return machineInfrastructureRef{
		Name:       machine.Name,
		Namespace:  machine.Namespace,
		Kind:       infra.Kind,
		APIVersion: infra.APIVersion,
	}, nil
}

func handleSetMachineInfrastructureAPIVersion(input *go_hook.HookInput) error {
	snaps := input.NewSnapshots.Get("capi_machines")

	for machine, err := range sdkobjectpatch.SnapshotIter[machineInfrastructureRef](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over Machine snapshots: %w", err)
		}

		apiVersion, ok := capiMachineVersions[machine.Kind]
		if !ok {
			input.Logger.Warn("unknown infrastructure template kind", slog.String("machine", machine.Name), slog.String("kind", machine.Kind))
			continue
		}

		expectedAPIVersion := capiInfrastructureAPIGroup + apiVersion
		if machine.APIVersion == expectedAPIVersion {
			continue
		}

		if machine.APIVersion != "" {
			input.Logger.Debug("infrastructureRef.apiVersion already set", slog.String("machine", machine.Name), slog.String("apiVersion", machine.APIVersion))
			continue
		}

		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"infrastructureRef": map[string]interface{}{
					"apiVersion": expectedAPIVersion,
				},
			},
		}

		input.PatchCollector.PatchWithMerge(patch, capiMachineAPIVersion, "Machine", machine.Namespace, machine.Name)
	}

	return nil
}
