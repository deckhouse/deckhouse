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
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// 'migration-adopt-old-fashioned-l2-lbs.go' depends on this hook
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/metallb/discovery",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "module_config",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"metallb"},
			},
			FilterFunc: applyModuleConfigFilter,
		},
		{
			Name:       "l2_advertisements",
			ApiVersion: "metallb.io/v1beta1",
			Kind:       "L2Advertisement",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-metallb"},
				},
			},
			FilterFunc: applyL2AdvertisementFilter,
		},
		{
			Name:       "ip_address_pools",
			ApiVersion: "metallb.io/v1beta1",
			Kind:       "IPAddressPool",
			FilterFunc: applyIPAddressPoolFilter,
		},
		{
			Name:       "mlbc_with_label",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "MetalLoadBalancerClass",
			FilterFunc: applyMLBCFilter,
		},
	},
}, migrateMCtoMLBC)

func applyModuleConfigFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &ModuleConfig{}
	err := sdk.FromUnstructured(obj, mc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert Metallb ModuleConfig: %v", err)
	}

	return mc, nil
}

func applyL2AdvertisementFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	l2Advertisement := &L2Advertisement{}
	err := sdk.FromUnstructured(obj, l2Advertisement)
	if err != nil {
		return nil, err
	}

	if len(l2Advertisement.Spec.IPAddressPools) == 0 {
		return nil, nil
	}
	if l2Advertisement.Labels != nil {
		if v, ok := l2Advertisement.Labels["heritage"]; ok && v == "deckhouse" {
			return nil, nil
		}
	}

	return L2AdvertisementInfo{
		Name:           l2Advertisement.Name,
		IPAddressPools: l2Advertisement.Spec.IPAddressPools,
		NodeSelectors:  l2Advertisement.Spec.NodeSelectors,
		Namespace:      l2Advertisement.Namespace,
		Interfaces:     l2Advertisement.Spec.Interfaces,
	}, nil
}

func applyIPAddressPoolFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ipAddressPool := &IPAddressPool{}
	err := sdk.FromUnstructured(obj, ipAddressPool)
	if err != nil {
		return nil, err
	}

	if ipAddressPool.Labels != nil {
		if v, ok := ipAddressPool.Labels["heritage"]; ok && v == "deckhouse" {
			return nil, nil
		}
	}

	return IPAddressPoolInfo{
		Name:      ipAddressPool.Name,
		Addresses: ipAddressPool.Spec.Addresses,
	}, err
}

func applyMLBCFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var mlbc MetalLoadBalancerClass

	err := sdk.FromUnstructured(obj, &mlbc)
	if err != nil {
		return nil, err
	}

	for k, v := range mlbc.Labels {
		if k == "auto-generated-by" && v == "d8-migration-hook" {
			if mlbc.Name != "l2-default" {
				return mlbc.Name, nil
			}
		}
	}
	return nil, nil
}

func createMetalLoadBalancerClass(input *go_hook.HookInput, mlbcInfo *MetalLoadBalancerClassInfo) {
	mlbc := map[string]any{
		"apiVersion": "network.deckhouse.io/v1alpha1",
		"kind":       "MetalLoadBalancerClass",
		"metadata": map[string]any{
			"name":   mlbcInfo.Name,
			"labels": mlbcInfo.Labels,
		},
		"spec": map[string]any{
			"isDefault":    mlbcInfo.IsDefault,
			"type":         "L2",
			"addressPool":  mlbcInfo.AddressPool,
			"nodeSelector": mlbcInfo.NodeSelector,
		},
	}
	if len(mlbcInfo.Interfaces) > 0 {
		mlbc["spec"].(map[string]any)["l2"] = map[string]any{
			"interfaces": mlbcInfo.Interfaces,
		}
	}
	mlbcUnstructured, err := sdk.ToUnstructured(&mlbc)
	if err != nil {
		return
	}
	input.PatchCollector.CreateOrUpdate(mlbcUnstructured)
	input.Logger.Info("MetalLoadBalancerClass created", "name", mlbcInfo.Name)
}

func deleteMetalLoadBalancerClass(input *go_hook.HookInput, mlbcName string) {
	input.PatchCollector.DeleteInBackground(
		"network.deckhouse.io/v1alpha1",
		"MetalLoadBalancerClass",
		"",
		mlbcName,
	)
	input.Logger.Info("MetalLoadBalancerClass deleted", "name", mlbcName)
}

func migrateMCtoMLBC(_ context.Context, input *go_hook.HookInput) error {
	snapsMC := input.Snapshots.Get("module_config")
	if len(snapsMC) != 1 || snapsMC[0] == nil {
		return nil
	}

	mc := new(ModuleConfig)

	err := snapsMC[0].UnmarshalTo(mc)
	if err != nil || mc.Spec.Version >= 2 {
		input.Logger.Info("processing skipped", "ModuleConfig version", mc.Spec.Version)
		return nil
	}

	// Create default MLBC
	var mlbcDefault MetalLoadBalancerClassInfo
	mlbcDefault.Name = "l2-default"
	mlbcDefault.IsDefault = true
	mlbcDefault.Labels = map[string]string{
		"auto-generated-by": "d8-migration-hook",
	}

	// Getting addressPools and nodeSelector from MC
	existsBGPPool := false
	existsL2Pool := false
	if len(mc.Spec.Settings.AddressPools) > 0 {
		for _, addressPool := range mc.Spec.Settings.AddressPools {
			if addressPool.Protocol == "bgp" {
				existsBGPPool = true
			}
			if addressPool.Protocol == "layer2" {
				existsL2Pool = true
			}
			mlbcDefault.AddressPool = append(mlbcDefault.AddressPool, addressPool.Addresses...)
		}
	}
	if mc.Spec.Settings.Speaker.NodeSelector != nil {
		mlbcDefault.NodeSelector = mc.Spec.Settings.Speaker.NodeSelector
	}

	if existsBGPPool {
		deleteMetalLoadBalancerClass(input, mlbcDefault.Name)
	} else if existsL2Pool {
		createMetalLoadBalancerClass(input, &mlbcDefault)
	}

	// Collect addresses from IPAddressPools
	ipAddressPools := make(map[string][]string, 4)
	snapsIAP := input.Snapshots.Get("ip_address_pools")
	for ipAddressPool, err := range sdkobjectpatch.SnapshotIter[IPAddressPoolInfo](snapsIAP) {
		if err != nil {
			continue
		}

		ipAddressPools[ipAddressPool.Name] = ipAddressPool.Addresses
	}

	// Create other MLBCs
	ipAddressPoolToMLBCMap := make(map[string]string, 4)
	desiredAutogeneratedMLBCs := make(map[string]bool, 4)
	snapsL2A := input.Snapshots.Get("l2_advertisements")
	for l2Advertisement, err := range sdkobjectpatch.SnapshotIter[L2AdvertisementInfo](snapsL2A) {
		if err != nil {
			continue
		}

		var mlbc MetalLoadBalancerClassInfo
		mlbc.IsDefault = false
		mlbc.Labels = map[string]string{
			"auto-generated-by": "d8-migration-hook",
		}
		mlbc.Name = "autogenerated-" + l2Advertisement.Name

		if len(l2Advertisement.Interfaces) > 0 {
			mlbc.Interfaces = l2Advertisement.Interfaces
		}
		if len(l2Advertisement.NodeSelectors) > 0 {
			if l2Advertisement.NodeSelectors[0].MatchLabels != nil {
				mlbc.NodeSelector = l2Advertisement.NodeSelectors[0].MatchLabels
			}
		}
		// Collecting addresses from IPAddressPools
		for _, pool := range l2Advertisement.IPAddressPools {
			if addresses, ok := ipAddressPools[pool]; ok {
				mlbc.AddressPool = append(mlbc.AddressPool, addresses...)
				ipAddressPoolToMLBCMap[pool] = mlbc.Name // Needed for use in another hook
			}
		}
		if len(mlbc.AddressPool) > 0 {
			desiredAutogeneratedMLBCs[mlbc.Name] = true // Needed to remove orphan MLBCs
			createMetalLoadBalancerClass(input, &mlbc)
		}
	}

	input.Values.Set("metallb.internal.ipAddressPoolToMLBCMap", ipAddressPoolToMLBCMap)

	// Delete orphan MLBC with the label
	snapsMLBC := input.Snapshots.Get("mlbc_with_label")
	for mlbcName, err := range sdkobjectpatch.SnapshotIter[string](snapsMLBC) {
		if err != nil {
			continue
		}

		if _, ok := desiredAutogeneratedMLBCs[mlbcName]; !ok {
			deleteMetalLoadBalancerClass(input, mlbcName)
		}
	}

	return nil
}
