/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	IPAddressPools []string               `json:"ipAddressPools,omitempty"`
	NodeSelectors  []metav1.LabelSelector `json:"nodeSelectors,omitempty"`
	Namespace      string                 `json:"namespace,omitempty"`
	Name           string                 `json:"name,omitempty"`
}

type IPAddressPoolInfo struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type ModuleConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModuleConfigSpec   `json:"spec"`
	Status ModuleConfigStatus `json:"status,omitempty"`
}

type ModuleConfigSpec struct {
	Version  int            `json:"version,omitempty"`
	Settings SettingsValues `json:"settings,omitempty"`
	Enabled  bool           `json:"enabled,omitempty"`
}

type SettingsValues struct {
	Speaker      Speaker       `json:"speaker" yaml:"speaker"`
	AddressPools []AddressPool `json:"addressPools" yaml:"addressPools"`
}

type Speaker struct {
	NodeSelector map[string]string `json:"nodeSelector" yaml:"nodeSelector"`
	Tolerations  []v1.Toleration   `json:"tolerations" yaml:"tolerations"`
}

type AddressPool struct {
	Name      string   `json:"name" yaml:"name"`
	Protocol  string   `json:"protocol" yaml:"protocol"`
	Addresses []string `json:"addresses" yaml:"addresses"`
}

type ModuleConfigStatus struct {
	Version string `json:"version"`
	Message string `json:"message"`
}

type OrphanedLoadBalancerServiceInfo struct {
	Name       string `json:"name,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	IsOrphaned bool   `json:"isOrphaned,omitempty"`
}
