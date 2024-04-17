/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/l2-load-balancer/status",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "l2loadbalancers",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "L2LoadBalancer",
			FilterFunc: applyLoadBalancerStatusFilter,
		},
		{
			Name:       "services",
			ApiVersion: "v1",
			Kind:       "Service",
			FilterFunc: applyServiceFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"heritage": "deckhouse",
					"app":      "l2-load-balancer",
				},
			},
		},
	},
}, handleLoadBalancerStatus)

func applyServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var service corev1.Service

	err := sdk.FromUnstructured(obj, &service)
	if err != nil {
		return nil, err
	}

	externalIP := "pending"
	if len(service.Status.LoadBalancer.Ingress) > 0 {
		externalIP = service.Status.LoadBalancer.Ingress[0].IP
	}
	loadBalancerName := service.Labels["instance"]

	return ServiceInfo{
		ExternalIP:       externalIP,
		LoadBalancerName: loadBalancerName,
	}, nil
}

func applyLoadBalancerStatusFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var lb L2LoadBalancer
	err := sdk.FromUnstructured(obj, &lb)
	if err != nil {
		return nil, err
	}

	return L2LoadBalancerInfo{
		Name:      lb.Name,
		Namespace: lb.Namespace,
	}, nil
}

func handleLoadBalancerStatus(input *go_hook.HookInput) error {
	servicesGroupedByLoadBalancerName := servicesSnapshotsToMap(input.Snapshots["services"])

	for _, lb := range input.Snapshots["l2loadbalancers"] {
		loadBalancer := lb.(L2LoadBalancerInfo)
		if externalIPs, ok := servicesGroupedByLoadBalancerName[loadBalancer.Name]; ok {
			patchLoadBalancerStatus(input.PatchCollector, loadBalancer.Name, loadBalancer.Namespace, externalIPs)
		}
	}
	return nil
}

func patchLoadBalancerStatus(pc *object_patch.PatchCollector, name, namespace string, ips []string) {
	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"publicAddresses": ips,
		},
	}

	pc.MergePatch(patch, "deckhouse.io/v1alpha1", "L2LoadBalancer", namespace, name, object_patch.WithSubresource("/status"))
}

func servicesSnapshotsToMap(servicesInfo []go_hook.FilterResult) (result map[string][]string) {
	result = make(map[string][]string)
	for _, si := range servicesInfo {
		serviceInfo := si.(ServiceInfo)
		if _, exists := result[serviceInfo.LoadBalancerName]; !exists {
			result[serviceInfo.LoadBalancerName] = make([]string, 0, 4)
		}
		result[serviceInfo.LoadBalancerName] = append(result[serviceInfo.LoadBalancerName], serviceInfo.ExternalIP)
	}
	return
}

type ServiceInfo struct {
	ExternalIP       string
	LoadBalancerName string
}
