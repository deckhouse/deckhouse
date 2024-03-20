/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pricing

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type NodeCapacity struct {
	Capacity  v1.ResourceList `json:"capacity"`
	NodeGroup string          `json:"nodeGroup"`
}

type NodeGroupCapacity struct {
	CPU    *resource.Quantity `json:"CPU"`
	Memory *resource.Quantity `json:"memory"`
}

type NodeGroupCapacityInt64 struct {
	CPU    int64 `json:"CPU"`
	Memory int64 `json:"memory"`
}

func ApplyNodeCapacityFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	n := &NodeCapacity{}

	if _, ok := node.ObjectMeta.Labels["node.deckhouse.io/group"]; !ok {
		return n, nil
	}

	n.Capacity = node.Status.Capacity
	n.NodeGroup = node.ObjectMeta.Labels["node.deckhouse.io/group"]

	return n, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 19,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "node",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: ApplyNodeCapacityFilter,
		},
	},
}, nodeCapacityHandler)

func nodeCapacityHandler(input *go_hook.HookInput) error {
	snaps, ok := input.Snapshots["node"]
	if !ok {
		input.LogEntry.Info("No Nodes received, skipping setting values")
		return nil
	}

	ngc := map[string]NodeGroupCapacity{}
	nodeGroupsCapacity := map[string]NodeGroupCapacityInt64{}

	for _, s := range snaps {
		node := s.(*NodeCapacity)

		if node.NodeGroup != "" {
			if _, ok := ngc[node.NodeGroup]; !ok {
				ngc[node.NodeGroup] = NodeGroupCapacity{
					CPU:    node.Capacity.Cpu(),
					Memory: node.Capacity.Memory(),
				}
			} else {
				ngc[node.NodeGroup].CPU.Add(*node.Capacity.Cpu())
				ngc[node.NodeGroup].Memory.Add(*node.Capacity.Memory())
			}
		}
	}

	for k, v := range ngc {
		nodeGroupsCapacity[k] = NodeGroupCapacityInt64{
			CPU:    v.CPU.Value(),
			Memory: v.Memory.Value(),
		}
	}

	input.Values.Set("flantIntegration.internal.nodeGroupsCapacity", nodeGroupsCapacity)

	return nil
}
