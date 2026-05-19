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
	capiMachineSetAPIVersion = "cluster.x-k8s.io/v1beta1"
)

type machineSetInfrastructureRef struct {
	Name       string
	Namespace  string
	Kind       string
	APIVersion string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/set_api_version_on_machine_set",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "capi_machinesets",
			ApiVersion:             capiMachineSetAPIVersion,
			Kind:                   "MachineSet",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: capiMachineSetInfrastructureRefFilter,
		},
	},
}, handleSetMachineSetInfrastructureAPIVersion)

func capiMachineSetInfrastructureRefFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	kind, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "infrastructureRef", "kind")
	apiVersion, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "infrastructureRef", "apiVersion")

	return machineSetInfrastructureRef{
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
		Kind:       kind,
		APIVersion: apiVersion,
	}, nil
}

func handleSetMachineSetInfrastructureAPIVersion(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("capi_machinesets")

	for ms, err := range sdkobjectpatch.SnapshotIter[machineSetInfrastructureRef](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over MachineSet snapshots: %w", err)
		}

		apiVersion, ok := capiMachineTemplateAPIVersions[ms.Kind]
		if !ok {
			input.Logger.Warn("unknown infrastructure template kind", slog.String("machineset", ms.Name), slog.String("kind", ms.Kind))
			continue
		}

		if ms.APIVersion == apiVersion {
			continue
		}

		if ms.APIVersion != "" {
			input.Logger.Debug("infrastructureRef.apiVersion already set", slog.String("machineset", ms.Name), slog.String("apiVersion", ms.APIVersion))
			continue
		}

		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"infrastructureRef": map[string]interface{}{
							"apiVersion": apiVersion,
						},
					},
				},
			},
		}

		input.PatchCollector.PatchWithMerge(patch, capiMachineSetAPIVersion, "MachineSet", ms.Namespace, ms.Name)
	}

	return nil
}
