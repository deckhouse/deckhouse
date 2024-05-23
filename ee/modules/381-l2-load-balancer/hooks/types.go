/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
)

type NodeInfo struct {
	Name      string
	Labels    map[string]string
	IsLabeled bool
}

type ServiceInfo struct {
	AnnotationIsMissed bool
	Name               string
	Namespace          string
	L2LoadBalancerName string
	LoadBalancerClass  string
	ClusterIP          string
	ExternalIPsCount   int
	Ports              []v1.ServicePort
	Selector           map[string]string
}

type L2LBServiceStatusInfo struct {
	Name      string
	Namespace string
	IP        string
}

type L2LBServiceConfig struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	ServiceName       string            `json:"serviceName"`
	ServiceNamespace  string            `json:"serviceNamespace"`
	PreferredNode     string            `json:"preferredNode,omitempty"`
	LoadBalancerClass string            `json:"loadBalancerClass"`
	ClusterIP         string            `json:"clusterIP"`
	Ports             []v1.ServicePort  `json:"ports"`
	Selector          map[string]string `json:"selector"`
}

type L2LoadBalancerInfo struct {
	Name         string            `json:"name"`
	AddressPool  []string          `json:"addressPool"`
	Interfaces   []string          `json:"interfaces"`
	NodeSelector map[string]string `json:"nodeSelector"`
}

type L2LoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   L2LoadBalancerSpec   `json:"spec,omitempty"`
	Status L2LoadBalancerStatus `json:"status,omitempty"`
}

type L2LoadBalancerSpec struct {
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	Interfaces   []string          `json:"interfaces,omitempty"`
	AddressPool  []string          `json:"addressPool,omitempty"`
}

type L2LoadBalancerStatus struct {
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
