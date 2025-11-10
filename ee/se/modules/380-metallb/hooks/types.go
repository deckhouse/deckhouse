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

const (
	keyAnnotationL2BalancerName   = "network.deckhouse.io/l2-load-balancer-name"
	keyAnnotationExternalIPsCount = "network.deckhouse.io/l2-load-balancer-external-ips-count"
	memberLabelKey                = "l2-load-balancer.network.deckhouse.io/member"
	metallbAllocatedPool          = "metallb.io/ip-allocated-from-pool"
	l2LoadBalancerIPsAnnotate     = "network.deckhouse.io/load-balancer-ips"
	lbAllowSharedIPAnnotate       = "network.deckhouse.io/load-balancer-shared-ip-key"
	mlbcAnnotate                  = "network.deckhouse.io/metal-load-balancer-class"
)

type NodeInfo struct {
	Name      string            `json:"name,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	IsLabeled bool              `json:"isLabeled,omitempty"`
}

type ServiceInfo struct {
	PublishNotReadyAddresses  bool                            `json:"publishNotReadyAddresses,omitempty"`
	Name                      string                          `json:"name,omitempty"`
	Namespace                 string                          `json:"namespace,omitempty"`
	LoadBalancerClass         string                          `json:"loadBalancerClass,omitempty"`
	AssignedLoadBalancerClass string                          `json:"assignedLoadBalancerClass,omitempty"`
	ClusterIP                 string                          `json:"clusterIP,omitempty"`
	ExternalIPsCount          int                             `json:"externalIPsCount,omitempty"`
	Ports                     []v1.ServicePort                `json:"ports,omitempty"`
	ExternalTrafficPolicy     v1.ServiceExternalTrafficPolicy `json:"externalTrafficPolicy,omitempty"`
	InternalTrafficPolicy     v1.ServiceInternalTrafficPolicy `json:"internalTrafficPolicy,omitempty"`
	Selector                  map[string]string               `json:"selector,omitempty"`
	DesiredIPs                []string                        `json:"desiredIPs,omitempty"`
	LBAllowSharedIP           string                          `json:"lbAllowSharedIP,omitempty"`
	AnnotationMLBC            string                          `json:"annotationMLBC,omitempty"`
	Conditions                []metav1.Condition              `json:"conditions,omitempty"`
}

type ServiceUpdaterInfo struct {
	Name       string             `json:"name,omitempty"`
	Namespace  string             `json:"namespace,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type L2LBServiceStatusInfo struct {
	Name              string `json:"name,omitempty"`
	Namespace         string `json:"namespace,omitempty"`
	LoadBalancerClass string `json:"loadBalancerClass,omitempty"`
	IP                string `json:"ip,omitempty"`
}

type L2LBServiceConfig struct {
	PublishNotReadyAddresses   bool                            `json:"publishNotReadyAddresses"`
	Name                       string                          `json:"name"`
	Namespace                  string                          `json:"namespace"`
	ServiceName                string                          `json:"serviceName"`
	ServiceNamespace           string                          `json:"serviceNamespace"`
	PreferredNode              string                          `json:"preferredNode,omitempty"`
	ClusterIP                  string                          `json:"clusterIP"`
	Ports                      []v1.ServicePort                `json:"ports"`
	ExternalTrafficPolicy      v1.ServiceExternalTrafficPolicy `json:"externalTrafficPolicy"`
	InternalTrafficPolicy      v1.ServiceInternalTrafficPolicy `json:"internalTrafficPolicy"`
	Selector                   map[string]string               `json:"selector"`
	MetalLoadBalancerClassName string                          `json:"mlbcName"`
	DesiredIP                  string                          `json:"desiredIP"`
	LBAllowSharedIP            string                          `json:"lbAllowSharedIP"`
}

type MetalLoadBalancerClassInfo struct {
	Name         string            `json:"name"`
	Labels       map[string]string `json:"labels,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	AddressPool  []string          `json:"addressPool"`
	Interfaces   []string          `json:"interfaces"`
	NodeSelector map[string]string `json:"nodeSelector"`
	IsDefault    bool              `json:"isDefault"`
}

type MetalLoadBalancerClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MetalLoadBalancerClassSpec   `json:"spec,omitempty"`
	Status MetalLoadBalancerClassStatus `json:"status,omitempty"`
}

type MetalLoadBalancerClassSpec struct {
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	L2           L2Type            `json:"l2,omitempty"`
	AddressPool  []string          `json:"addressPool,omitempty"`
	IsDefault    bool              `json:"isDefault,omitempty"`
}

type L2Type struct {
	Interfaces []string `json:"interfaces,omitempty"`
}

type MetalLoadBalancerClassStatus struct {
}

type SDNInternalL2LBServiceSpec struct {
	v1.ServiceSpec `json:",inline"`
	ServiceRef     SDNInternalL2LBServiceReference `json:"serviceRef"`
}

type SDNInternalL2LBServiceReference struct {
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,1,opt,name=namespace"`
	Name      string `json:"name" protobuf:"bytes,2,opt,name=name"`
}

type SDNInternalL2LBService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   SDNInternalL2LBServiceSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status v1.ServiceStatus           `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

type SDNIpsLBName struct {
	Ips         []string `json:"ips,omitempty"`
	LBClassName string   `json:"lbClassName,omitempty"`
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

type IPAddressPoolInfo struct {
	Name      string   `json:"name,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
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
	Name           string                 `json:"name,omitempty"`
	IPAddressPools []string               `json:"ipAddressPools,omitempty"`
	Interfaces     []string               `json:"interfaces,omitempty"`
	NodeSelectors  []metav1.LabelSelector `json:"nodeSelectors,omitempty"`
	Namespace      string                 `json:"namespace,omitempty"`
}

type ServiceInfoForAlert struct {
	Name        string            `json:"name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}
