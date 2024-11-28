/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	metallbConfigurationStatusKey = "metallb:ConfigurationStatus"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "l2_advertisements",
			ApiVersion: "metallb.io/v1beta1",
			Kind:       "L2Advertisement",
			FilterFunc: applyL2AdvertisementFilter,
		},
		{
			Name:       "ip_address_pools",
			ApiVersion: "metallb.io/v1beta1",
			Kind:       "IPAddressPool",
			FilterFunc: applyIPAddressPoolFilter,
		},
		{
			Name:       "services",
			ApiVersion: "v1",
			Kind:       "Service",
			FilterFunc: applyServiceFilter,
		},
		{
			Name:       "module_config",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"metallb"},
			},
			FilterFunc: applyModuleConfigFilter,
		},
	},
}, checkAllRequirementsForUpgrade)

func applyL2AdvertisementFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	l2Advertisement := &L2Advertisement{}
	err := sdk.FromUnstructured(obj, l2Advertisement)
	if err != nil {
		return nil, err
	}

	return L2AdvertisementInfo{
		IPAddressPools: l2Advertisement.Spec.IPAddressPools,
		NodeSelectors:  l2Advertisement.Spec.NodeSelectors,
		Namespace:      l2Advertisement.Namespace,
		Name:           l2Advertisement.Name,
	}, nil
}

func applyIPAddressPoolFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ipAddressPool := &IPAddressPool{}
	err := sdk.FromUnstructured(obj, ipAddressPool)
	if err != nil {
		return nil, err
	}
	return IPAddressPoolInfo{
		Name:      ipAddressPool.Name,
		Namespace: ipAddressPool.Namespace,
	}, nil
}

func applyModuleConfigFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &ModuleConfig{}
	err := sdk.FromUnstructured(obj, mc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert Metallb ModuleConfig: %v", err)
	}
	return mc, nil
}

func applyServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var service v1.Service

	err := sdk.FromUnstructured(obj, &service)
	if err != nil {
		return nil, err
	}

	if service.Spec.Type != v1.ServiceTypeLoadBalancer {
		return nil, nil
	}

	olbsInfo := OrphanedLoadBalancerServiceInfo{
		Name:       service.Name,
		Namespace:  service.Namespace,
		IsOrphaned: true,
	}

	if service.Spec.LoadBalancerClass != nil {
		olbsInfo.IsOrphaned = false
	}

	if _, ok := service.Annotations["metallb.universe.tf/address-pool"]; ok {
		olbsInfo.IsOrphaned = false
	}

	if _, ok := service.Annotations["metallb.universe.tf/ip-allocated-from-pool"]; ok {
		olbsInfo.IsOrphaned = false
	}

	return olbsInfo, nil
}

func checkAllRequirementsForUpgrade(input *go_hook.HookInput) error {
	// Disable all alerts
	for _, alertGroup := range []string{
		"D8MetallbIpAddressPoolNSMismatch",
		"D8MetallbL2AdvertisementNSMismatch",
		"D8MetallbOrphanedLoadBalancerDetected",
		"D8MetallbL2AdvertisementNodeSelectorsMismatch",
		"D8MetallbBothBGPAndL2PoolsConfigured",
	} {
		input.MetricsCollector.Expire(alertGroup)
	}
	requirements.SaveValue(metallbConfigurationStatusKey, "OK")

	// Check ModuleConfig version
	mcSnaps := input.Snapshots["module_config"]
	if len(mcSnaps) != 1 {
		return nil
	}
	mc, ok := mcSnaps[0].(*ModuleConfig)
	if !ok || mc.Spec.Version >= 2 {
		return nil
	}

	// Are only layer2 pools in the cluster?
	protocols := make(map[string]bool)
	l2AdvertisementsCount := len(input.Snapshots["l2_advertisements"])
	for _, pool := range mc.Spec.Settings.AddressPools {
		protocols[pool.Protocol] = true
		if protocols["bgp"] && (protocols["layer2"] || l2AdvertisementsCount > 0) {
			requirements.SaveValue(metallbConfigurationStatusKey, "Misconfigured")
			input.MetricsCollector.Set("d8_metallb_not_only_layer2_pools", 1,
				map[string]string{}, metrics.WithGroup("D8MetallbBothBGPAndL2PoolsConfigured"))
			break
		}
	}

	l2AdvertisementSnaps := input.Snapshots["l2_advertisements"]
	for _, l2AdvertisementSnap := range l2AdvertisementSnaps {
		if l2Advertisement, ok := l2AdvertisementSnap.(L2AdvertisementInfo); ok {
			// Check the namespace
			if l2Advertisement.Namespace != "d8-metallb" {
				requirements.SaveValue(metallbConfigurationStatusKey, "Misconfigured")
				input.MetricsCollector.Set("d8_metallb_l2advertisement_ns_mismatch", 1,
					map[string]string{
						"namespace": l2Advertisement.Namespace,
						"name":      l2Advertisement.Name,
					}, metrics.WithGroup("D8MetallbL2AdvertisementNSMismatch"))
			}

			// There should only be one matchLabels (not matchExpressions) in nodeSelectors
			if len(l2Advertisement.NodeSelectors) > 1 {
				requirements.SaveValue(metallbConfigurationStatusKey, "Misconfigured")
				input.MetricsCollector.Set("d8_metallb_l2advertisement_node_selectors_mismatch", 1,
					map[string]string{
						"name": l2Advertisement.Name,
					}, metrics.WithGroup("D8MetallbL2AdvertisementNodeSelectorsMismatch"))
			} else if len(l2Advertisement.NodeSelectors) == 1 {
				nodeSelector := l2Advertisement.NodeSelectors[0]
				if len(nodeSelector.MatchExpressions) > 0 {
					requirements.SaveValue(metallbConfigurationStatusKey, "Misconfigured")
					input.MetricsCollector.Set("d8_metallb_l2advertisement_node_selectors_mismatch", 1,
						map[string]string{
							"name": l2Advertisement.Name,
						}, metrics.WithGroup("D8MetallbL2AdvertisementNodeSelectorsMismatch"))
				}
			}
		}
	}

	ipAddressPoolSnaps := input.Snapshots["ip_address_pools"]
	for _, ipAddressPoolSnap := range ipAddressPoolSnaps {
		if ipAddressPool, ok := ipAddressPoolSnap.(IPAddressPoolInfo); ok {
			// Check a namespace
			if ipAddressPool.Namespace != "d8-metallb" {
				requirements.SaveValue(metallbConfigurationStatusKey, "Misconfigured")
				input.MetricsCollector.Set("d8_metallb_ipaddress_pool_ns_mismatch", 1,
					map[string]string{
						"namespace": ipAddressPool.Namespace,
						"name":      ipAddressPool.Name,
					}, metrics.WithGroup("D8MetallbIpAddressPoolNSMismatch"))
			}
		}
	}

	// Search orphaned Services
	serviceSnaps := input.Snapshots["services"]
	for _, serviceSnap := range serviceSnaps {
		if serviceSnap == nil {
			continue
		}
		if service, ok := serviceSnap.(OrphanedLoadBalancerServiceInfo); ok && service.IsOrphaned {
			requirements.SaveValue(metallbConfigurationStatusKey, "Misconfigured")
			input.MetricsCollector.Set("d8_metallb_orphaned_loadbalancer_detected", 1,
				map[string]string{
					"name":      service.Name,
					"namespace": service.Namespace,
				}, metrics.WithGroup("D8MetallbOrphanedLoadBalancerDetected"))
		}
	}
	return nil
}
