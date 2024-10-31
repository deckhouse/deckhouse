/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"slices"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

type L2Advertisement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   L2AdvertisementSpec   `json:"spec,omitempty"`
	Status L2AdvertisementStatus `json:"status,omitempty"`
}

type L2AdvertisementSpec struct {
	IPAddressPools         []string               `json:"ipAddressPools,omitempty"`
	IPAddressPoolSelectors []metav1.LabelSelector `json:"ipAddressPoolSelectors,omitempty"`
	NodeSelectors          []metav1.LabelSelector `json:"nodeSelectors,omitempty"`
	Interfaces             []string               `json:"interfaces,omitempty"`
}

type L2AdvertisementStatus struct {
}

type IPAddressPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPAddressPoolSpec   `json:"spec"`
	Status IPAddressPoolStatus `json:"status,omitempty"`
}

type IPAddressPoolSpec struct {
	Addresses     []string           `json:"addresses"`
	AutoAssign    *bool              `json:"autoAssign,omitempty"`
	AvoidBuggyIPs bool               `json:"avoidBuggyIPs,omitempty"`
	AllocateTo    *ServiceAllocation `json:"serviceAllocation,omitempty"`
}

type ServiceAllocation struct {
	Priority           int                    `json:"priority,omitempty"`
	Namespaces         []string               `json:"namespaces,omitempty"`
	NamespaceSelectors []metav1.LabelSelector `json:"namespaceSelectors,omitempty"`
	ServiceSelectors   []metav1.LabelSelector `json:"serviceSelectors,omitempty"`
}

type IPAddressPoolStatus struct {
}

type L2AdvertisementInfo struct {
	IPAddressPools []string
	NodeSelectors  []metav1.LabelSelector
	Namespace      string
}

const (
	metallbConfigurationStatusKey = "metallb:ConfigurationStatus"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "l2advertisements",
			ApiVersion: "metallb.io/v1beta1",
			Kind:       "L2Advertisement",
			FilterFunc: applyL2AdvertisementFilter,
		},
		{
			Name:       "ipaddresspools",
			ApiVersion: "metallb.io/v1beta1",
			Kind:       "IPAddressPool",
			FilterFunc: applyIPAddressPoolFilter,
		},
	},
}, checkAllRequirementsForUpgrade)

func applyL2AdvertisementFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	l2Advertisement := &L2Advertisement{}
	err := sdk.FromUnstructured(obj, l2Advertisement)
	if err != nil {
		return nil, err
	}

	if len(l2Advertisement.Spec.IPAddressPools) == 0 {
		return nil, nil
	}

	return L2AdvertisementInfo{
		IPAddressPools: l2Advertisement.Spec.IPAddressPools,
		NodeSelectors:  l2Advertisement.Spec.NodeSelectors,
		Namespace:      l2Advertisement.Namespace,
	}, nil
}

func applyIPAddressPoolFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ipAddressPool := &IPAddressPool{}
	err := sdk.FromUnstructured(obj, ipAddressPool)
	if err != nil {
		return nil, err
	}

	return ipAddressPool.Name, err
}

func checkAllRequirementsForUpgrade(input *go_hook.HookInput) error {
	ipAddressPoolNamesFromL2A := make([]string, 0, 8)
	l2AdvertisementSnaps := input.Snapshots["l2advertisements"]
	for _, l2AdvertisementSnap := range l2AdvertisementSnaps {
		l2Advertisement := l2AdvertisementSnap.(L2AdvertisementInfo)
		// Check a namespace
		if l2Advertisement.Namespace != "d8-metallb" {
			requirements.SaveValue(metallbConfigurationStatusKey, "NSMismatch")
			return nil
		}

		// There should only be one matchLabels (not matchExpressions) in nodeSelectors
		if len(l2Advertisement.NodeSelectors) > 0 {
			if len(l2Advertisement.NodeSelectors) != 1 {
				requirements.SaveValue(metallbConfigurationStatusKey, "NodeSelectorsMismatch")
				return nil
			}
			nodeSelector := l2Advertisement.NodeSelectors[0]
			if len(nodeSelector.MatchExpressions) > 0 {
				requirements.SaveValue(metallbConfigurationStatusKey, "NodeSelectorsMismatch")
				return nil
			}
		}

		// Collect names of ipAddressPools from L2Advertisement
		ipAddressPoolNamesFromL2A = append(ipAddressPoolNamesFromL2A, l2Advertisement.IPAddressPools...)
	}

	ipAddressPoolNamesFromIAP := make([]string, 0, 8)
	ipAddressPoolSnaps := input.Snapshots["ipaddresspools"]
	for _, ipAddressPoolSnap := range ipAddressPoolSnaps {
		ipAddressPoolName := ipAddressPoolSnap.(string)
		// Collect names of ipAddressPools from IPAddressPools
		ipAddressPoolNamesFromIAP = append(ipAddressPoolNamesFromIAP, ipAddressPoolName)
	}

	// Are only layer2 pools in the cluster?
	sort.Strings(ipAddressPoolNamesFromL2A) // Only layer2 pools
	sort.Strings(ipAddressPoolNamesFromIAP) // All pools of cluster
	if !slices.Equal(ipAddressPoolNamesFromL2A, ipAddressPoolNamesFromIAP) {
		requirements.SaveValue(metallbConfigurationStatusKey, "AddressPoolsMismatch")
		return nil
	}
	requirements.SaveValue(metallbConfigurationStatusKey, "OK")
	return nil
}
