/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	keyAnnotationL2BalancerName   = "network.deckhouse.io/l2-load-balancer-name"
	keyAnnotationExternalIPsCount = "network.deckhouse.io/l2-load-balancer-external-ips-count"
	memberLabelKey                = "l2-load-balancer.network.deckhouse.io/member"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/l2-load-balancer/discovery",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "l2loadbalancers",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "L2LoadBalancer",
			FilterFunc: applyLoadBalancerFilter,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyNodeFilter,
		},
		{
			Name:       "services",
			ApiVersion: "v1",
			Kind:       "Service",
			FilterFunc: applyServiceFilter,
		},
	},
}, handleLoadBalancers)

func applyNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
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

func applyServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var service v1.Service

	err := sdk.FromUnstructured(obj, &service)
	if err != nil {
		return nil, err
	}

	if service.Spec.Type != v1.ServiceTypeLoadBalancer {
		// we only need service of LoadBalancer type
		return nil, nil
	}

	var l2LBName string
	l2LBName, ok := service.Annotations[keyAnnotationL2BalancerName]
	if !ok {
		// L2LoadBalancer name must be specified
		return ServiceInfo{Name: service.Name, Namespace: service.Namespace, AnnotationIsMissed: true}, nil
	}

	var externalIPsCount = 1
	if externalIPsCountStr, ok := service.Annotations[keyAnnotationExternalIPsCount]; ok {
		if externalIP, err := strconv.Atoi(externalIPsCountStr); err == nil {
			if externalIP > 1 {
				externalIPsCount = externalIP
			}
		}
	}

	var loadBalancerClass string
	if service.Spec.LoadBalancerClass != nil {
		loadBalancerClass = *service.Spec.LoadBalancerClass
	}

	return ServiceInfo{
		AnnotationIsMissed: false,
		Name:               service.GetName(),
		Namespace:          service.GetNamespace(),
		L2LoadBalancerName: l2LBName,
		LoadBalancerClass:  loadBalancerClass,
		ExternalIPsCount:   externalIPsCount,
		Ports:              service.Spec.Ports,
		Selector:           service.Spec.Selector,
		ClusterIP:          service.Spec.ClusterIP,
	}, nil
}

func applyLoadBalancerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
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

func handleLoadBalancers(input *go_hook.HookInput) error {
	l2lbservices := make([]L2LBServiceConfig, 0, 4)
	l2loadbalancers := makeL2LoadBalancersMapFromSnapshot(input.Snapshots["l2loadbalancers"])

	alreadyLabeledNodes := getLabeledNodes(input.Snapshots["nodes"])
	needToBeLabeledNodes := make([]NodeInfo, 0, 4)

	for _, serviceSnap := range input.Snapshots["services"] {
		service, ok := serviceSnap.(ServiceInfo)
		if !ok {
			continue
		}

		if service.AnnotationIsMissed {
			input.LogEntry.Warnf("Annotation with L2LoadBalancer is missed for service %s in namespace %s", service.Name, service.Namespace)
			continue
		}

		l2lb, exists := l2loadbalancers[service.L2LoadBalancerName]
		if !exists {
			// L2LoadBalancer is not founded by name
			continue
		}

		nodes := getNodesByNodeSelector(l2lb.NodeSelector, input.Snapshots["nodes"])
		if len(nodes) == 0 {
			// There is no node that matches the specified node selector.
			continue
		}

		needToBeLabeledNodes = append(needToBeLabeledNodes, nodes...)

		for i := 1; i <= service.ExternalIPsCount; i++ {
			nodeIndex := i % len(nodes)
			l2lbservices = append(l2lbservices, L2LBServiceConfig{
				Name:              fmt.Sprintf("%s-%s-%d", service.Name, l2lb.Name, i),
				Namespace:         service.Namespace,
				ServiceName:       service.Name,
				ServiceNamespace:  service.Namespace,
				PreferredNode:     nodes[nodeIndex].Name,
				LoadBalancerClass: service.LoadBalancerClass,
				ClusterIP:         service.ClusterIP,
				Ports:             service.Ports,
				Selector:          service.Selector,
			})
		}
	}

	nodesToUnlabel, nodesToLabel := calcDifferenceForNodes(alreadyLabeledNodes, needToBeLabeledNodes)

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

	// L2 Load Balncers are sorted before saving
	l2loadbalancersInternal := make([]L2LoadBalancerInfo, 0, len(l2loadbalancers))
	for _, value := range l2loadbalancers {
		l2loadbalancersInternal = append(l2loadbalancersInternal, value)
	}
	sort.Slice(l2loadbalancersInternal, func(i, j int) bool {
		return l2loadbalancersInternal[i].Name < l2loadbalancersInternal[j].Name
	})
	input.Values.Set("l2LoadBalancer.internal.l2loadbalancers", l2loadbalancersInternal)

	// L2 Load Balancer Services are sorted by Namespace and then Name before saving
	sort.Slice(l2lbservices, func(i, j int) bool {
		if l2lbservices[i].Namespace == l2lbservices[j].Namespace {
			return l2lbservices[i].Name < l2lbservices[j].Name
		}
		return l2lbservices[i].Namespace < l2lbservices[j].Namespace
	})
	input.Values.Set("l2LoadBalancer.internal.l2lbservices", l2lbservices)
	return nil
}

func makeL2LoadBalancersMapFromSnapshot(snapshot []go_hook.FilterResult) map[string]L2LoadBalancerInfo {
	l2lbMap := make(map[string]L2LoadBalancerInfo)
	for _, l2lbSnap := range snapshot {
		if l2lb, ok := l2lbSnap.(L2LoadBalancerInfo); ok {
			l2lbMap[l2lb.Name] = l2lb
		}
	}
	return l2lbMap
}

func getNodesByNodeSelector(nodeSelector map[string]string, snapshot []go_hook.FilterResult) []NodeInfo {
	nodes := make([]NodeInfo, 0, 4)
	for _, nodeSnap := range snapshot {
		node := nodeSnap.(NodeInfo)
		if nodeMatchesNodeSelector(node.Labels, nodeSelector) {
			nodes = append(nodes, node)
		}
	}
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

func getLabeledNodes(snapshot []go_hook.FilterResult) []NodeInfo {
	result := make([]NodeInfo, 0, 4)
	for _, l2lbSnap := range snapshot {
		nodeInfo, ok := l2lbSnap.(NodeInfo)
		if !ok {
			continue
		}

		if nodeInfo.IsLabeled {
			result = append(result, nodeInfo)
		}
	}
	return result
}

func calcDifferenceForNodes(nodesLabeled, nodesNeeded []NodeInfo) ([]NodeInfo, []NodeInfo) {
	left := []NodeInfo{}
	right := []NodeInfo{}

	seenLeft := map[string]struct{}{}
	seenRight := map[string]struct{}{}

	for _, node := range nodesLabeled {
		seenLeft[node.Name] = struct{}{}
	}
	for _, node := range nodesNeeded {
		seenRight[node.Name] = struct{}{}
	}
	for _, node := range nodesLabeled {
		if _, exists := seenRight[node.Name]; !exists {
			left = append(left, node)
		}
	}

	for _, node := range nodesNeeded {
		if _, exists := seenLeft[node.Name]; !exists {
			right = append(right, node)
		}
	}
	return left, right
}
