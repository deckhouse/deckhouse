/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/metallb/discovery",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "l2lb_services",
			ApiVersion: "internal.network.deckhouse.io/v1alpha1",
			Kind:       "SDNInternalL2LBService",
			FilterFunc: applyL2LBServiceFilter,
		},
		{
			Name:       "services",
			ApiVersion: "v1",
			Kind:       "Service",
			FilterFunc: applyServiceFilterForStatusUpdater,
		},
	},
}, handleL2LBServices)

func applyServiceFilterForStatusUpdater(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var service v1.Service
	err := sdk.FromUnstructured(obj, &service)
	if err != nil {
		return nil, err
	}

	if service.Spec.Type != v1.ServiceTypeLoadBalancer {
		// we only need service of LoadBalancer type
		return nil, nil
	}

	return ServiceUpdaterInfo{
		Name:       service.GetName(),
		Namespace:  service.GetNamespace(),
		Conditions: service.Status.Conditions,
	}, nil
}

func applyL2LBServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var l2LBService SDNInternalL2LBService

	err := sdk.FromUnstructured(obj, &l2LBService)
	if err != nil {
		return nil, err
	}

	ip := "unknown"
	if len(l2LBService.Status.LoadBalancer.Ingress) > 0 {
		ip = l2LBService.Status.LoadBalancer.Ingress[0].IP
	}

	return L2LBServiceStatusInfo{
		Namespace: l2LBService.Spec.ServiceRef.Namespace,
		Name:      l2LBService.Spec.ServiceRef.Name,
		IP:        ip,
	}, nil
}

func handleL2LBServices(_ context.Context, input *go_hook.HookInput) error {
	namespacedServicesWithIPs := getNamespacedNameOfServicesWithIPs(input.Snapshots.Get("l2lb_services"))
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

		var service *ServiceUpdaterInfo
		for svc, err := range sdkobjectpatch.SnapshotIter[ServiceUpdaterInfo](input.Snapshots.Get("services")) {
			if err != nil {
				continue
			}
			if namespacedName.Name == svc.Name && namespacedName.Namespace == svc.Namespace {
				service = &svc
				break
			}
		}
		if service == nil {
			return nil
		}

		conditionStatus := metav1.ConditionFalse
		reason := "NotAllIPsAssigned"
		if totalIPs == assignedIPs {
			conditionStatus = metav1.ConditionTrue
			reason = "AllIPsAssigned"
		}
		conditions := updateCondition(service.Conditions, metav1.Condition{
			Status:  conditionStatus,
			Type:    "AllPublicIPsAssigned",
			Message: fmt.Sprintf("%d of %d public IPs were assigned", assignedIPs, totalIPs),
			Reason:  reason,
		})
		patch := map[string]any{
			"status": map[string]any{
				"loadBalancer": map[string]any{
					"ingress": IPsForStatus,
				},
				"conditions": conditions,
			},
		}

		input.PatchCollector.PatchWithMerge(patch,
			"v1",
			"Service",
			namespacedName.Namespace,
			namespacedName.Name,
			object_patch.WithSubresource("/status"))
		input.Logger.Info("Service status updated",
			"name", namespacedName.Name,
			"Namespace", namespacedName.Name,
		)
	}
	return nil
}

func getNamespacedNameOfServicesWithIPs(snapshots []sdkpkg.Snapshot) map[types.NamespacedName][]string {
	result := make(map[types.NamespacedName][]string)
	for service, err := range sdkobjectpatch.SnapshotIter[L2LBServiceStatusInfo](snapshots) {
		if err != nil {
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

func updateCondition(conditions []metav1.Condition, newCondition metav1.Condition) []metav1.Condition {
	for i, condition := range conditions {
		if condition.Type == newCondition.Type {
			conditions[i] = newCondition
			return conditions
		}
	}
	return append(conditions, newCondition)
}
