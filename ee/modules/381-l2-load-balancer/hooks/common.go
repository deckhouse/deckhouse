/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type L2LoadBalancerInfo struct {
	Name                  string               `json:"name"`
	Namespace             string               `json:"namespace"`
	AddressPool           string               `json:"addressPool"`
	ExternalTrafficPolicy string               `json:"externalTrafficPolicy"`
	SourceRanges          []string             `json:"sourceRanges,omitempty"`
	NodeSelector          map[string]string    `json:"nodeSelector"`
	Selector              map[string]string    `json:"selector"`
	Ports                 []corev1.ServicePort `json:"ports"`
	Nodes                 []map[string]string  `json:"nodes"`
}

type L2LoadBalancer struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec L2LoadBalancerSpec `json:"spec"`
}

type L2LoadBalancerSpec struct {
	AddressPool  string                    `json:"addressPool"`
	NodeSelector map[string]string         `json:"nodeSelector"`
	Nodes        []map[string]string       `json:"nodes"`
	Service      L2LoadBalancerSpecService `json:"service"`
}

type L2LoadBalancerSpecService struct {
	ExternalTrafficPolicy string               `json:"externalTrafficPolicy,omitempty"`
	Selector              map[string]string    `json:"selector"`
	Ports                 []corev1.ServicePort `json:"ports"`
	SourceRanges          []string             `json:"sourceRanges,omitempty"`
}

type NodeInfo struct {
	Name      string
	Labels    map[string]string
	IsLabeled bool // is there `l2-load-balancer.network.deckhouse.io/member` label
}

type NodeSet struct {
	// map["<node name>"]struct{}
	nodes map[string]struct{}
}

func labelSelectorFromMap(m map[string]string) string {
	parts := make([]string, 0, len(m))
	for key, value := range m {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, ",")
}

func NewNodeSet() *NodeSet {
	return &NodeSet{
		nodes: make(map[string]struct{}),
	}
}

func (ns *NodeSet) Put(nodeName string) {
	if _, exists := ns.nodes[nodeName]; !exists {
		ns.nodes[nodeName] = struct{}{}
	}
}

// Check is node with name in set
func (ns *NodeSet) Contains(nodeName string) bool {
	_, exists := ns.nodes[nodeName]
	return exists
}

func (ns *NodeSet) GetNames() []string {
	result := make([]string, 0, len(ns.nodes))
	for key := range ns.nodes {
		result = append(result, key)
	}
	return result
}
