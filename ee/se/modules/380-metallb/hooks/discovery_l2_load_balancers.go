/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// Depends on 'migration-adopt-old-fashioned-l2-lbs.go' hook
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Queue:        "/modules/metallb/discovery",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "mlbc",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "MetalLoadBalancerClass",
			FilterFunc: applyMetalLoadBalancerClassFilter,
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
}, handleL2LoadBalancers)

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

	var externalIPsCount = 1
	if externalIPsCountStr, ok := service.Annotations[keyAnnotationExternalIPsCount]; ok {
		if externalIP, err := strconv.Atoi(externalIPsCountStr); err == nil {
			if externalIP > 1 {
				externalIPsCount = externalIP
			}
		}
	}

	var desiredIPs []string
	if DesiredIPsStr, ok := service.Annotations[l2LoadBalancerIPsAnnotate]; ok {
		desiredIPs = strings.Split(DesiredIPsStr, ",")
	}

	var lbAllowSharedIP string
	if lbAllowSharedIPStr, ok := service.Annotations[lbAllowSharedIPAnnotate]; ok {
		lbAllowSharedIP = lbAllowSharedIPStr
	}

	var mlbcAnnotation string
	if mlbcAnnotationStr, ok := service.Annotations[mlbcAnnotate]; ok {
		mlbcAnnotation = mlbcAnnotationStr
	}

	var loadBalancerClass string
	if service.Spec.LoadBalancerClass != nil {
		loadBalancerClass = *service.Spec.LoadBalancerClass
	}

	var assignedMLBC string
	for _, condition := range service.Status.Conditions {
		if condition.Type == "network.deckhouse.io/load-balancer-class" {
			assignedMLBC = condition.Message
			break
		}
	}

	internalTrafficPolicy := v1.ServiceInternalTrafficPolicyCluster
	if service.Spec.InternalTrafficPolicy != nil {
		internalTrafficPolicy = *service.Spec.InternalTrafficPolicy
	}

	return ServiceInfo{
		Name:                      service.GetName(),
		Namespace:                 service.GetNamespace(),
		LoadBalancerClass:         loadBalancerClass,
		AssignedLoadBalancerClass: assignedMLBC,
		ExternalIPsCount:          externalIPsCount,
		Ports:                     service.Spec.Ports,
		ExternalTrafficPolicy:     service.Spec.ExternalTrafficPolicy,
		InternalTrafficPolicy:     internalTrafficPolicy,
		Selector:                  service.Spec.Selector,
		ClusterIP:                 service.Spec.ClusterIP,
		PublishNotReadyAddresses:  service.Spec.PublishNotReadyAddresses,
		DesiredIPs:                desiredIPs,
		LBAllowSharedIP:           lbAllowSharedIP,
		AnnotationMLBC:            mlbcAnnotation,
		Conditions:                service.Status.Conditions,
	}, nil
}

func applyMetalLoadBalancerClassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var metalLoadBalancerClass MetalLoadBalancerClass

	err := sdk.FromUnstructured(obj, &metalLoadBalancerClass)
	if err != nil {
		return nil, err
	}

	interfaces := metalLoadBalancerClass.Spec.L2.Interfaces
	if interfaces == nil {
		interfaces = []string{}
	}

	nodeSelector := metalLoadBalancerClass.Spec.NodeSelector
	if nodeSelector == nil {
		nodeSelector = make(map[string]string)
	}

	addressPool := metalLoadBalancerClass.Spec.AddressPool
	if addressPool == nil {
		addressPool = []string{}
	}

	return MetalLoadBalancerClassInfo{
		Name:         metalLoadBalancerClass.Name,
		AddressPool:  addressPool,
		Interfaces:   interfaces,
		NodeSelector: nodeSelector,
		IsDefault:    metalLoadBalancerClass.Spec.IsDefault,
	}, nil
}

func handleL2LoadBalancers(_ context.Context, input *go_hook.HookInput) error {
	if value, ok := input.Values.GetOk("metallb.internal.migrationOfOldFashionedLBsAdoptionComplete"); ok {
		if !value.Bool() {
			return nil
		}
	}

	l2LBServices := make([]L2LBServiceConfig, 0, 4)
	mlbcMap, mlbcDefaultName := makeMLBCMapFromSnapshot(input.Snapshots.Get("mlbc"))

	for service, err := range sdkobjectpatch.SnapshotIter[ServiceInfo](input.Snapshots.Get("services")) {
		if err != nil {
			continue
		}

		patchStatusInformation := true
		var mlbcForUse MetalLoadBalancerClassInfo
		if mlbcTemp, ok := mlbcMap[mlbcDefaultName]; ok {
			// Use default MLBC (and add to status)
			mlbcForUse = mlbcTemp
		}
		if mlbcTemp, ok := mlbcMap[service.AnnotationMLBC]; ok {
			// Use MLBC from annotation (and add to status)
			mlbcForUse = mlbcTemp
		}
		if mlbcTemp, ok := mlbcMap[service.LoadBalancerClass]; ok {
			// Else use the MLBC that exists in the cluster (and add to status)
			mlbcForUse = mlbcTemp
		}
		if service.AssignedLoadBalancerClass != "" {
			if mlbcTemp, ok := mlbcMap[service.AssignedLoadBalancerClass]; ok {
				// Else use the MLBC associated earlier
				mlbcForUse = mlbcTemp
				patchStatusInformation = false
			} else {
				// MLBC is not among clustered MLBCs, but it is associated (in status)
				continue
			}
		}
		if mlbcForUse.Name == "" {
			continue
		}

		nodes := getNodesByMLBC(mlbcForUse, input.Snapshots.Get("nodes"))
		if len(nodes) == 0 {
			// There is no node that matches the specified node selector.
			continue
		}

		conditions := updateCondition(service.Conditions, metav1.Condition{
			Type:    "network.deckhouse.io/load-balancer-class",
			Message: mlbcForUse.Name,
			Status:  "True",
			Reason:  "LoadBalancerClassBound",
		})
		if patchStatusInformation {
			patch := map[string]any{
				"status": map[string]any{
					"conditions": conditions,
				},
			}

			input.PatchCollector.PatchWithMerge(patch,
				"v1",
				"Service",
				service.Namespace,
				service.Name,
				object_patch.WithSubresource("/status"))
			input.Logger.Info("MetalLoadBalancerClass was selected and added to the service status",
				"Service", service.Name,
				"Namespace", service.Namespace,
				"MetalLoadBalancerClass", mlbcForUse.Name,
			)
		}

		desiredIPsCount := len(service.DesiredIPs)
		desiredIPsExist := desiredIPsCount > 0
		for i := 0; i < service.ExternalIPsCount; i++ {
			nodeIndex := i % len(nodes)
			config := L2LBServiceConfig{
				Name:                       fmt.Sprintf("%s-%s-%d", service.Name, mlbcForUse.Name, i),
				Namespace:                  service.Namespace,
				ServiceName:                service.Name,
				ServiceNamespace:           service.Namespace,
				PreferredNode:              nodes[nodeIndex].Name,
				ExternalTrafficPolicy:      service.ExternalTrafficPolicy,
				InternalTrafficPolicy:      service.InternalTrafficPolicy,
				PublishNotReadyAddresses:   service.PublishNotReadyAddresses,
				ClusterIP:                  service.ClusterIP,
				Ports:                      service.Ports,
				Selector:                   service.Selector,
				MetalLoadBalancerClassName: mlbcForUse.Name,
				LBAllowSharedIP:            service.LBAllowSharedIP,
			}
			if desiredIPsExist && i < desiredIPsCount {
				config.DesiredIP = service.DesiredIPs[i]
			}
			l2LBServices = append(l2LBServices, config)
		}
	}

	// L2 Load Balancers are sorted before saving
	l2LoadBalancersInternal := make([]MetalLoadBalancerClassInfo, 0, len(mlbcMap))
	for _, value := range mlbcMap {
		l2LoadBalancersInternal = append(l2LoadBalancersInternal, value)
	}
	sort.Slice(l2LoadBalancersInternal, func(i, j int) bool {
		return l2LoadBalancersInternal[i].Name < l2LoadBalancersInternal[j].Name
	})
	input.Values.Set("metallb.internal.l2loadbalancers", l2LoadBalancersInternal)

	// L2 Load Balancer Services are sorted by Namespace and then Name before saving
	sort.Slice(l2LBServices, func(i, j int) bool {
		if l2LBServices[i].Namespace == l2LBServices[j].Namespace {
			return l2LBServices[i].Name < l2LBServices[j].Name
		}
		return l2LBServices[i].Namespace < l2LBServices[j].Namespace
	})
	input.Values.Set("metallb.internal.l2lbservices", l2LBServices)
	return nil
}

func makeMLBCMapFromSnapshot(snapshots []sdkpkg.Snapshot) (map[string]MetalLoadBalancerClassInfo, string) {
	mlbcMap := make(map[string]MetalLoadBalancerClassInfo)
	var mlbcDefaultName string

	for mlbc, err := range sdkobjectpatch.SnapshotIter[MetalLoadBalancerClassInfo](snapshots) {
		if err != nil {
			continue
		}

		mlbcMap[mlbc.Name] = mlbc
		if mlbc.IsDefault {
			mlbcDefaultName = mlbc.Name
		}
	}

	return mlbcMap, mlbcDefaultName
}
