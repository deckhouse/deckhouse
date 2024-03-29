/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/testing"
)

const (
	memberLabelKey = "l2-load-balancer.network.deckhouse.io/member"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/l2-load-balancer/discovery",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "l2loadbalancers",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "L2LoadBalancer",
			FilterFunc: applyLoadBalancerFilter,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyNodeFilter,
		},
	},
}, handleLoadBalancers)

func applyNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	var isLabeled bool
	if _, exists := node.Labels[memberLabelKey]; exists {
		isLabeled = true
	}

	return NodeInfo{
		Name:      node.Name,
		Labels:    node.Labels,
		IsLabeled: isLabeled,
	}, nil
}

func applyLoadBalancerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var lb L2LoadBalancer
	err := sdk.FromUnstructured(obj, &lb)
	if err != nil {
		return nil, err
	}

	return L2LoadBalancerInfo{
		Name:         lb.Name,
		Namespace:    lb.Namespace,
		AddressPool:  lb.Spec.AddressPool,
		NodeSelector: lb.Spec.NodeSelector,
		Selector:     lb.Spec.Service.Selector,
		Ports:        lb.Spec.Service.Ports,
		SourceRanges: lb.Spec.Service.SourceRanges,
	}, nil
}

func handleLoadBalancers(input *go_hook.HookInput) error {
	l2LBsSnapshot := input.Snapshots["l2loadbalancers"]
	speakerNodes := NewNodeSet()
	loadBalancers := make([]L2LoadBalancerInfo, 0, 8)

	for _, lb := range l2LBsSnapshot {
		loadBalancer := lb.(L2LoadBalancerInfo)
		nodeSelector := labelSelectorFromMap(loadBalancer.NodeSelector)
		nodeSelectorLabels, _, _ := testing.ExtractFromListOptions(metav1.ListOptions{LabelSelector: nodeSelector})
		if nodeSelectorLabels == nil {
			nodeSelectorLabels = labels.Everything() // by default, the selector is "every node"
		}

		// nodes[0]["name"] = "kube-front-0"
		lbNodes := make([]map[string]string, 0, 4)

		nodesSnapshot := input.Snapshots["nodes"]
		for _, n := range nodesSnapshot {
			nodeInfo := n.(NodeInfo)

			if nodeSelectorLabels.Matches(labels.Set(nodeInfo.Labels)) {
				speakerNodes.Put(nodeInfo.Name)
				lbNodes = append(
					lbNodes,
					map[string]string{"name": nodeInfo.Name},
				)
			}
		}
		loadBalancer.Nodes = lbNodes
		loadBalancers = append(loadBalancers, loadBalancer)
	}

	// Set label or remove label from node if needed
	nodesSnapshot := input.Snapshots["nodes"]
	for _, n := range nodesSnapshot {
		nodeInfo := n.(NodeInfo)

		// Node is in internal Speaker Nodes array but without label. Need to set
		if !nodeInfo.IsLabeled && speakerNodes.Contains(nodeInfo.Name) {
			input.PatchCollector.Filter(appendL2LabelsToNode, "v1", "Node", "", nodeInfo.Name)
		}

		// Node is not in internal Speaker Nodes array but with label. Need to delete label
		if nodeInfo.IsLabeled && !speakerNodes.Contains(nodeInfo.Name) {
			input.PatchCollector.Filter(removeL2LabelsFromNode, "v1", "Node", "", nodeInfo.Name)
		}
	}

	input.Values.Set("l2LoadBalancer.internal.speakerNodes", speakerNodes.GetNames())
	input.Values.Set("l2LoadBalancer.internal.l2LoadBalancers", loadBalancers)
	return nil
}

func removeL2LabelsFromNode(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	var node *corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	nodeLabels := make(map[string]string)

	for key, value := range node.Labels {
		if key != memberLabelKey {
			nodeLabels[key] = value
		}
	}

	node.Labels = nodeLabels
	return sdk.ToUnstructured(node)
}

func appendL2LabelsToNode(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	var node *corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	nodeLabels := make(map[string]string)

	isNeedToAddLabel := true
	for key, value := range node.Labels {
		if key == memberLabelKey {
			isNeedToAddLabel = false
		}
		nodeLabels[key] = value
	}

	if isNeedToAddLabel {
		nodeLabels[memberLabelKey] = ""
	}

	node.Labels = nodeLabels
	return sdk.ToUnstructured(node)
}
