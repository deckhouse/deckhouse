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

	// Enabled is TODO
	Enabled bool `json:"enabled,omitempty"`

	// Mandatory is TODO
	Mandatory bool `json:"mandatory,omitempty"`

	// Polarity is TODO
	Polarity string `json:"polarity,omitempty"`

	// NodeLabelSelector is TODO
	NodeLabelSelector map[string]string `json:"nodeLabelSelector,omitempty"`
}

// VCDAffinityRuleStatus defines the observed state of VCDAffinityRule.
type VCDAffinityRuleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Message is TODO
	Message string `json:"message,omitempty"`

	// Rule ID is TODO
	RuleID string `json:"ruleID,omitempty"`

	// Nodes is TODO
	Nodes []VCDAffinityRuleStatusNode `json:"nodes,omitempty"`

	// NodeCount is TODO
	NodeCount int `json:"nodeCount,omitempty"`
}

type VCDAffinityRuleStatusNode struct {
	// Name is TODO
	Name string `json:"name,omitempty"`
	// ID is TODO
	ID string `json:"ID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=vcdar
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="NodeCount",type=integer,JSONPath=`.status.nodeCount`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`

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
