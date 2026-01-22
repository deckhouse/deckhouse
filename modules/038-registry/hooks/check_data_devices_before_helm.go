/*
Copyright 2025 Flant JSC

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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

const (
	dataDeviceReadyNodeLabel = "node.deckhouse.io/registry-data-device-ready"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/registry/check-data-devices-bofore-helm",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodes_with_data_device",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					dataDeviceReadyNodeLabel: "true",
				},
			},
			FilterFunc: filterRegistryDataDeviceNodes,
		},
	},
}, handleRegistryDataDeviceNodes)

func filterRegistryDataDeviceNodes(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node v1core.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return "", fmt.Errorf("Failed to convert node to struct: %v", err)
	}

	return node.Name, nil
}

func handleRegistryDataDeviceNodes(_ context.Context, input *go_hook.HookInput) error {
	orchestratorTargetMode := registry_const.ToModeType(
		input.Values.Get("registry.internal.orchestrator.state.target_mode").String())

	// If the orchestrator is in direct or unmanaged mode, we do not need to check for data devices.
	// In these modes, the there is no registry instance, so no data devices are required.
	if orchestratorTargetMode == registry_const.ModeDirect || orchestratorTargetMode == registry_const.ModeUnmanaged {
		return nil
	}

	clusterType, ok := input.Values.GetOk("global.clusterConfiguration.clusterType")
	if !ok {
		return fmt.Errorf("Cluster type 'global.clusterConfiguration.clusterType' not found")
	}

	if clusterType.String() == "Static" {
		return nil
	}

	nodes := input.Snapshots.Get("nodes_with_data_device")

	if len(nodes) == 0 {
		return fmt.Errorf("No nodes with registry data devices found in the cloud cluster")
	}

	return nil
}
