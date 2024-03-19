/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pricing

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Node struct {
	Version        string          `json:"version"`
	Type           string          `json:"type"`
	MasterNodeInfo MasterNodeInfo  `json:"masterNodeInfo"`
	Capacity       v1.ResourceList `json:"capacity"`
	NodeGroup      string          `json:"nodeGroup"`
}

type MasterNodeInfo struct {
	IsDedicated bool               `json:"isDedicated"`
	CPU         *resource.Quantity `json:"cpu"`
	Memory      *resource.Quantity `json:"memory"`
}

type NodeStats struct {
	StaticNodesCount      int64  `json:"staticNodesCount"`
	MastersCount          int64  `json:"mastersCount"`
	MasterIsDedicated     bool   `json:"masterIsDedicated"`
	MasterMinCPU          int64  `json:"masterMinCPU"`
	MasterMinMemory       int64  `json:"masterMinMemory"`
	MinimalKubeletVersion string `json:"minimalKubeletVersion"`
}

type NodeGroupCapacity struct {
	CPU    *resource.Quantity `json:"CPU"`
	Memory *resource.Quantity `json:"memory"`
}

type NodeGroupCapacityInt64 struct {
	CPU    int64 `json:"CPU"`
	Memory int64 `json:"memory"`
}

func ApplyPricingNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	n := &Node{}
	n.Version = node.Status.NodeInfo.KubeletVersion[1:]

	if t, ok := node.ObjectMeta.Labels["node.deckhouse.io/type"]; ok {
		n.Type = t
	}

	if nodeGroup, ok := node.ObjectMeta.Labels["node.deckhouse.io/group"]; !ok || nodeGroup != "master" {
		return n, nil
	}

	for _, taint := range node.Spec.Taints {
		if taint.Key == "node-role.kubernetes.io/control-plane" ||
			taint.Key == "node-role.kubernetes.io/master" {
			n.MasterNodeInfo.IsDedicated = true
			break
		}
	}

	n.MasterNodeInfo.CPU = node.Status.Allocatable.Cpu()
	n.MasterNodeInfo.Memory = node.Status.Allocatable.Memory()

	n.Capacity = node.Status.Capacity
	n.NodeGroup = node.Labels["node.deckhouse.io/group"]

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
			FilterFunc: ApplyPricingNodeFilter,
		},
	},
}, nodeHandler)

func nodeHandler(input *go_hook.HookInput) error {
	snaps, ok := input.Snapshots["node"]
	if !ok {
		input.LogEntry.Info("No Nodes received, skipping setting values")
		return nil
	}

	var minNodeVersion *semver.Version
	stats := NodeStats{}

	ngc := map[string]NodeGroupCapacity{}
	nodeGroupsCapacity := map[string]NodeGroupCapacityInt64{}

	for _, s := range snaps {
		node := s.(*Node)

		nodeVersion, err := semver.NewVersion(node.Version)
		if err != nil {
			return fmt.Errorf("can't parse Node version: %v", err)
		}
		if minNodeVersion == nil || nodeVersion.LessThan(minNodeVersion) {
			minNodeVersion = nodeVersion
		}

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

		if node.Type == "Static" {
			stats.StaticNodesCount++
		}

		if node.MasterNodeInfo == (MasterNodeInfo{}) {
			continue
		}

		stats.MastersCount++

		if node.MasterNodeInfo.IsDedicated {
			stats.MasterIsDedicated = true
		}

		nodeCPU := node.MasterNodeInfo.CPU.Value()
		if stats.MasterMinCPU == 0 || nodeCPU < stats.MasterMinCPU {
			stats.MasterMinCPU = node.MasterNodeInfo.CPU.Value()
		}

		nodeMemory := node.MasterNodeInfo.Memory.Value()
		if stats.MasterMinMemory == 0 || nodeMemory < stats.MasterMinMemory {
			stats.MasterMinMemory = nodeMemory
		}
	}

	stats.MinimalKubeletVersion = fmt.Sprintf("%d.%d", minNodeVersion.Major(), minNodeVersion.Minor())

	for k, v := range ngc {
		nodeGroupsCapacity[k] = NodeGroupCapacityInt64{
			CPU:    v.CPU.Value(),
			Memory: v.Memory.Value(),
		}
	}

	input.Values.Set("flantIntegration.internal.nodeStats", stats)
	input.Values.Set("flantIntegration.internal.nodeGroupsCapacity", nodeGroupsCapacity)

	return nil
}
