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

	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/capi/v1beta1"
)

const (
	capiMachineDeploymentAPIVersion = "cluster.x-k8s.io/v1beta1"
	capiInfrastructureAPIGroup     = "infrastructure.cluster.x-k8s.io/"
)

var capiMachineTemplateVersions = map[string]string{
	"DynamixMachineTemplate":     "v1alpha1",
	"HuaweiCloudMachineTemplate": "v1alpha1",
	"VCDMachineTemplate":         "v1beta2",
	"ZvirtMachineTemplate":       "v1",
	"DeckhouseMachineTemplate":   "v1alpha1",
	"StaticMachineTemplate":      "v1alpha1",
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/set_api_version_on_machine_deployment",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "capi_mds",
			ApiVersion:                   capiMachineDeploymentAPIVersion,
			Kind:                         "MachineDeployment",
			WaitForSynchronization:       ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(true),
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
	Name       string
	Namespace  string
	Kind       string
	APIVersion string
}

func capiInfrastructureRefFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var md v1beta1.MachineDeployment

	if err := sdk.FromUnstructured(obj, &md); err != nil {
		return nil, err
	}

	infra := md.Spec.Template.Spec.InfrastructureRef

	return machineDeploymentInfrastructureRef{
		Name:       md.Name,
		Namespace:  md.Namespace,
		Kind:       infra.Kind,
		APIVersion: infra.APIVersion,
	}, nil
}

func handleSetInfrastructureAPIVersion(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("capi_mds")

	for md, err := range sdkobjectpatch.SnapshotIter[machineDeploymentInfrastructureRef](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over MachineDeployment snapshots: %w", err)
		}

		apiVersion, ok := capiMachineTemplateVersions[md.Kind]
		if !ok {
			input.Logger.Warn("unknown infrastructure template kind", slog.String("machinedeployment", md.Name), slog.String("kind", md.Kind))
			continue
		}

		expectedAPIVersion := capiInfrastructureAPIGroup + apiVersion

		if md.APIVersion == expectedAPIVersion {
			continue
		}

		if md.APIVersion != "" {
			input.Logger.Debug("infrastructureRef.apiVersion already set", slog.String("machinedeployment", md.Name), slog.String("apiVersion", md.APIVersion))
			continue
		}

		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"infrastructureRef": map[string]interface{}{
							"apiVersion": expectedAPIVersion,
						},
					},
				},
			},
		}

		input.PatchCollector.PatchWithMerge(patch, capiMachineDeploymentAPIVersion, "MachineDeployment", md.Namespace, md.Name)
	}

	return nil
}
