/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 15},
	Queue:        "/modules/metallb/node-labels-update",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "mlbc",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "MetalLoadBalancerClass",
			FilterFunc: applyMetalLoadBalancerClassLabelFilter,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyNodeLabelFilter,
		},
	},
}, handleLabelsUpdate)

func applyNodeLabelFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node v1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	_, isLabeled := node.Labels[memberLabelKey]

	return NodeInfo{
		Name:      node.Name,
		Labels:    node.Labels,
		IsLabeled: isLabeled,
	}, nil
}

func applyMetalLoadBalancerClassLabelFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var metalLoadBalancerClass MetalLoadBalancerClass

	err := sdk.FromUnstructured(obj, &metalLoadBalancerClass)
	if err != nil {
		return nil, err
	}

	return MetalLoadBalancerClassInfo{
		Name:         metalLoadBalancerClass.Name,
		AddressPool:  metalLoadBalancerClass.Spec.AddressPool,
		NodeSelector: metalLoadBalancerClass.Spec.NodeSelector,
	}, nil
}

func handleLabelsUpdate(_ context.Context, input *go_hook.HookInput) error {
	actualLabeledNodes := getLabeledNodes(input.Snapshots.Get("nodes"))
	desiredLabeledNodes := make([]NodeInfo, 0, 4)

	for mlbcInfo, err := range sdkobjectpatch.SnapshotIter[MetalLoadBalancerClassInfo](input.Snapshots.Get("mlbc")) {
		if err != nil {
			continue
		}

		nodes := getNodesByMLBC(mlbcInfo, input.Snapshots.Get("nodes"))
		if len(nodes) == 0 {
			// There is no node that matches the specified node selector.
			continue
		}
		desiredLabeledNodes = appendUniq(desiredLabeledNodes, nodes...)
	}

	nodesToUnLabel, nodesToLabel := calcDifferenceForNodes(actualLabeledNodes, desiredLabeledNodes)

	for _, node := range nodesToUnLabel {
		labelsPatch := map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					memberLabelKey: nil,
				},
			},
		}
		input.PatchCollector.PatchWithMerge(labelsPatch, "v1", "Node", "", node.Name)
	}

	for _, node := range nodesToLabel {
		labelsPatch := map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					memberLabelKey: "",
				},
			},
		}
		input.PatchCollector.PatchWithMerge(labelsPatch, "v1", "Node", "", node.Name)
	}

	return nil
}

func getLabeledNodes(snapshots []sdkpkg.Snapshot) []NodeInfo {
	result := make([]NodeInfo, 0, 4)
	for nodeInfo, err := range sdkobjectpatch.SnapshotIter[NodeInfo](snapshots) {
		if err != nil {
			continue
		}

		if nodeInfo.IsLabeled {
			result = append(result, nodeInfo)
		}
	}

	return result
}

func getNodesByMLBC(lb MetalLoadBalancerClassInfo, snapshots []sdkpkg.Snapshot) []NodeInfo {
	nodes := make([]NodeInfo, 0, 4)
	for node, err := range sdkobjectpatch.SnapshotIter[NodeInfo](snapshots) {
		if err != nil {
			continue
		}

		if nodeMatchesNodeSelector(node.Labels, lb.NodeSelector) {
			nodes = append(nodes, node)
		}
	}

	// Sort using hashing and the LoadBalancer name to avoid always occupying the first node in the usual order.
	// For example: 5 frontend-nodes sorted in alphabet order, 10 LB with number of IPs equal 1, and frontend-0 will be busy
	sort.Slice(nodes, func(i, j int) bool {
		hi := sha256.Sum256([]byte(lb.Name + "#" + nodes[i].Name))
		hj := sha256.Sum256([]byte(lb.Name + "#" + nodes[j].Name))
		return bytes.Compare(hi[:], hj[:]) < 0
	})
	return nodes
}

func nodeMatchesNodeSelector(nodeLabels, selectorLabels map[string]string) bool {
	for selectorKey, selectorValue := range selectorLabels {
		nodeLabelValue, exists := nodeLabels[selectorKey]
		if !exists {
			return false
		}
		if selectorValue != nodeLabelValue {
			return false
		}
	}
	return true
}

func calcDifferenceForNodes(nodesLabeled, nodesNeeded []NodeInfo) ([]NodeInfo, []NodeInfo) {
	nodesToUnLabel := []NodeInfo{}
	nodesToLabel := []NodeInfo{}

	actualLabeledNodesMap := map[string]struct{}{}
	desiredLabeledNodesMap := map[string]struct{}{}

	for _, node := range nodesLabeled {
		actualLabeledNodesMap[node.Name] = struct{}{}
	}
	for _, node := range nodesNeeded {
		desiredLabeledNodesMap[node.Name] = struct{}{}
	}
	for _, node := range nodesLabeled {
		if _, exists := desiredLabeledNodesMap[node.Name]; !exists {
			nodesToUnLabel = append(nodesToUnLabel, node)
		}
	}

	for _, node := range nodesNeeded {
		if _, exists := actualLabeledNodesMap[node.Name]; !exists {
			nodesToLabel = append(nodesToLabel, node)
		}
	}
	return nodesToUnLabel, nodesToLabel
}

func appendUniq(existingNodes []NodeInfo, nodes ...NodeInfo) []NodeInfo {
	existingNodesMap := make(map[string]struct{})
	result := existingNodes
	for _, node := range existingNodes {
		existingNodesMap[node.Name] = struct{}{}
	}

	for _, node := range nodes {
		if _, exists := existingNodesMap[node.Name]; !exists {
			result = append(result, node)
			existingNodesMap[node.Name] = struct{}{}
		}
	}
	return result
}
