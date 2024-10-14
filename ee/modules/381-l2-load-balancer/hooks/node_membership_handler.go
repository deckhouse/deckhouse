/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"bytes"
	"crypto/sha256"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 8},
	Queue:        "/modules/l2-load-balancer/node-labels-update",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "l2loadbalancers",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "L2LoadBalancer",
			FilterFunc: applyLoadBalancerLabelFilter,
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

func applyLoadBalancerLabelFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var l2loadbalancer L2LoadBalancer

	err := sdk.FromUnstructured(obj, &l2loadbalancer)
	if err != nil {
		return nil, err
	}

	return L2LoadBalancerInfo{
		Name:         l2loadbalancer.Name,
		AddressPool:  l2loadbalancer.Spec.AddressPool,
		NodeSelector: l2loadbalancer.Spec.NodeSelector,
	}, nil
}

func handleLabelsUpdate(input *go_hook.HookInput) error {
	actualLabeledNodes := getLabeledNodes(input.Snapshots["nodes"])
	desiredLabeledNodes := make([]NodeInfo, 0, 4)

	for _, l2lbSnap := range input.Snapshots["l2loadbalancers"] {
		l2lbInfo, ok := l2lbSnap.(L2LoadBalancerInfo)
		if !ok {
			continue
		}

		nodes := getNodesByLoadBalancer(l2lbInfo, input.Snapshots["nodes"])
		if len(nodes) == 0 {
			// There is no node that matches the specified node selector.
			continue
		}
		desiredLabeledNodes = appendUniq(desiredLabeledNodes, nodes...)
	}

	nodesToUnlabel, nodesToLabel := calcDifferenceForNodes(actualLabeledNodes, desiredLabeledNodes)

	for _, node := range nodesToUnlabel {
		labelsPatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					memberLabelKey: nil,
				},
			},
		}
		input.PatchCollector.MergePatch(labelsPatch, "v1", "Node", "", node.Name)
	}

	for _, node := range nodesToLabel {
		node.Labels[memberLabelKey] = ""
		labelsPatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": node.Labels,
			},
		}
		input.PatchCollector.MergePatch(labelsPatch, "v1", "Node", "", node.Name)
	}

	return nil
}

func getLabeledNodes(snapshot []go_hook.FilterResult) []NodeInfo {
	result := make([]NodeInfo, 0, 4)
	for _, nodeSnap := range snapshot {
		nodeInfo, ok := nodeSnap.(NodeInfo)
		if !ok {
			continue
		}

		if nodeInfo.IsLabeled {
			result = append(result, nodeInfo)
		}
	}
	return result
}

func getNodesByLoadBalancer(lb L2LoadBalancerInfo, snapshot []go_hook.FilterResult) []NodeInfo {
	nodes := make([]NodeInfo, 0, 4)
	for _, nodeSnap := range snapshot {
		node := nodeSnap.(NodeInfo)
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
	nodesToUnlabel := []NodeInfo{}
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
			nodesToUnlabel = append(nodesToUnlabel, node)
		}
	}

	for _, node := range nodesNeeded {
		if _, exists := actualLabeledNodesMap[node.Name]; !exists {
			nodesToLabel = append(nodesToLabel, node)
		}
	}
	return nodesToUnlabel, nodesToLabel
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
