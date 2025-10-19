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

	// Indicates whether the affinity rule is mandatory (`true`) or only preferred (`false`).
	// If set to `true`, the rule must be enforced; if `false`, the rule is applied if possible but not strictly required.
	// +optional
	Required bool `json:"required,omitempty"`

	// The polarity of the affinity rule. Must be either `Affinity` or `AntiAffinity`.
	// `Affinity` means that nodes should be placed on the same host,
	// while `AntiAffinity` means they should be placed on different hosts.
	// +kubebuilder:validation:Enum=Affinity;AntiAffinity
	// +kubebuilder:validation:Required
	Polarity string `json:"polarity,omitempty"`

	// NodeLabelSelector is a selector for the nodes that this rule applies to.
	// Empty selector means that the rule applies to all nodes.
	// +optional
	NodeLabelSelector metav1.LabelSelector `json:"nodeLabelSelector,omitempty"`
}

// VCDAffinityRuleStatus defines the observed state of VCDAffinityRule.
type VCDAffinityRuleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Message is text description of the current status of the rule.
	Message string `json:"message,omitempty"`

	// Rule ID is a unique identifier for the rule from the VCD API.
	RuleID string `json:"ruleID,omitempty"`

	// Nodes is a list of nodes that are affected by the rule. Each node has a name and an ID.
	Nodes []VCDAffinityRuleStatusNode `json:"nodes,omitempty"`

	// NodeCount is the number of nodes that are affected by the rule.
	NodeCount int `json:"nodeCount,omitempty"`
}

type VCDAffinityRuleStatusNode struct {
	// Name is the name of the node.
	Name string `json:"name,omitempty"`
	// ID is the unique identifier for the node from the VCD API.
	ID string `json:"ID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=vcdar
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="NodeCount",type=integer,JSONPath=`.status.nodeCount`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`
// +kubebuilder:metadata:labels="heritage=deckhouse"
// +kubebuilder:metadata:labels="module=cloud-provider-vcd"

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
