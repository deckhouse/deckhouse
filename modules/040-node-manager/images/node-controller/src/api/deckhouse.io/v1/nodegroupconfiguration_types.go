package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type NodeGroupConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NodeGroupConfigurationSpec `json:"spec"`
}

type NodeGroupConfigurationSpec struct {
	Content    string   `json:"content"`
	Weight     int      `json:"weight,omitempty"`
	NodeGroups []string `json:"nodeGroups,omitempty"`
	Bundles    []string `json:"bundles,omitempty"`
}

// +kubebuilder:object:root=true
type NodeGroupConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeGroupConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeGroupConfiguration{}, &NodeGroupConfigurationList{})
}
