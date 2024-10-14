/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/l2-load-balancer/service-update",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "l2lbservices",
			ApiVersion: "internal.network.deckhouse.io/v1alpha1",
			Kind:       "SDNInternalL2LBService",
			FilterFunc: applyL2LBServiceFilter,
		},
	},
}, handleL2LBServices)

func applyL2LBServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var l2lbservice SDNInternalL2LBService

	err := sdk.FromUnstructured(obj, &l2lbservice)
	if err != nil {
		return nil, err
	}

	ip := "unknown"
	if len(l2lbservice.Status.LoadBalancer.Ingress) > 0 {
		ip = l2lbservice.Status.LoadBalancer.Ingress[0].IP
	}

	return L2LBServiceStatusInfo{
		Namespace: l2lbservice.Spec.ServiceRef.Namespace,
		Name:      l2lbservice.Spec.ServiceRef.Name,
		IP:        ip,
	}, nil
}

func handleL2LBServices(input *go_hook.HookInput) error {
	namespacedServicesWithIPs := getNamespacedNameOfServicesWithIPs(input.Snapshots["l2lbservices"])
	for namespacedName, ips := range namespacedServicesWithIPs {
		IPsForStatus := make([]map[string]string, 0, len(ips))
		totalIPs := len(ips)
		assignedIPs := 0
		for _, ip := range ips {
			if ip == "unknown" {
				continue
			}
			assignedIPs++
			IPsForStatus = append(IPsForStatus, map[string]string{"ip": ip})
		}
		conditionStatus := metav1.ConditionFalse
		reason := "NotAllIPsAssigned"
		if totalIPs == assignedIPs {
			conditionStatus = metav1.ConditionTrue
			reason = "AllIPsAssigned"
		}
		patch := map[string]interface{}{
			"status": map[string]interface{}{
				"loadBalancer": map[string]interface{}{
					"ingress": IPsForStatus,
				},
				"conditions": []metav1.Condition{
					{
						Status:  conditionStatus,
						Type:    "AllPublicIPsAssigned",
						Message: fmt.Sprintf("%d of %d public IPs were assigned", assignedIPs, totalIPs),
						Reason:  reason,
					},
				},
			},
		}

		input.PatchCollector.MergePatch(patch,
			"v1",
			"Service",
			namespacedName.Namespace,
			namespacedName.Name,
			object_patch.WithSubresource("/status"))
	}
	return nil
}

func getNamespacedNameOfServicesWithIPs(snapshot []go_hook.FilterResult) map[types.NamespacedName][]string {
	result := make(map[types.NamespacedName][]string)
	for _, serviceSnap := range snapshot {
		service, ok := serviceSnap.(L2LBServiceStatusInfo)
		if !ok {
			continue
		}
		namespacedNameKey := types.NamespacedName{Name: service.Name, Namespace: service.Namespace}
		ips, exists := result[namespacedNameKey]
		if !exists {
			ips = make([]string, 0, 2)
		}
		ips = append(ips, service.IP)

		result[namespacedNameKey] = ips
	}
	return result
}
