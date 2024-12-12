/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	dataDeviceReadyNodeLabel = "node.deckhouse.io/registry-data-device-ready"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/system-registry/check-data-devices-bofore-helm",
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

func handleRegistryDataDeviceNodes(input *go_hook.HookInput) error {
	clusterType, ok := input.Values.GetOk("global.clusterConfiguration.clusterType")
	if !ok {
		return fmt.Errorf("Cluster type 'global.clusterConfiguration.clusterType' not found")
	}

	if clusterType.String() == "Static" {
		return nil
	}

	nodes := input.Snapshots["nodes_with_data_device"]

	if len(nodes) == 0 {
		return fmt.Errorf("No nodes with registry data devices found in the cloud cluster")
	}

	return nil
}
