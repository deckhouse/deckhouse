/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// VCDAffinityRuleSpec defines the desired state of VCDAffinityRule.
type VCDAffinityRuleSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Polarity is TODO
	Polarity string `json:"polarity,omitempty"`

	// NodeLabelSelector is TODO
	NodeLabelSelector map[string]string `json:"nodeLabelSelector,omitempty"`
}

// VCDAffinityRuleStatus defines the observed state of VCDAffinityRule.
type VCDAffinityRuleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Message string                      `json:"message,omitempty"`
	Nodes   []VCDAffinityRuleStatusNode `json:"nodes,omitempty"`
}

type VCDAffinityRuleStatusNode struct {
	Name       string `json:"name,omitempty"`
	ProviderID string `json:"providerID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=vcdar
// +kubebuilder:subresource:status

// VCDAffinityRule is the Schema for the vcdaffinityrules API.
type VCDAffinityRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VCDAffinityRuleSpec   `json:"spec,omitempty"`
	Status VCDAffinityRuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VCDAffinityRuleList contains a list of VCDAffinityRule.
type VCDAffinityRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VCDAffinityRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VCDAffinityRule{}, &VCDAffinityRuleList{})
}
