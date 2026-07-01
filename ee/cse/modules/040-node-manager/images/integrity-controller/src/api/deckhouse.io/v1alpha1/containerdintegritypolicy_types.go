/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContainerdIntegrityPolicySpec defines the desired state of ContainerdIntegrityPolicy.
type ContainerdIntegrityPolicySpec struct {
	// PEM-encoded CA certificate used to verify image signatures.
	CA string `json:"ca"`

	// Label selector that defines which namespaces are protected by this policy.
	ProtectedNamespaces ProtectedNamespacesSelector `json:"protectedNamespaces"`
}

// ProtectedNamespacesSelector selects namespaces by labels.
type ProtectedNamespacesSelector struct {
	// MatchLabels is a map of {key,value} pairs.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// ContainerdIntegrityPolicyStatus defines the observed state of ContainerdIntegrityPolicy.
type ContainerdIntegrityPolicyStatus struct {
	// List of namespace names that match the protectedNamespaces selector.
	// +optional
	ProtectedNamespaces []string `json:"protectedNamespaces,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="heritage=deckhouse"
// +kubebuilder:metadata:labels="module=node-manager"
// +kubebuilder:printcolumn:name="ProtectedNamespaces",type="string",JSONPath=".status.protectedNamespaces",description="Namespaces protected by this policy"

// ContainerdIntegrityPolicy defines containerd image integrity policy for selected namespaces.
type ContainerdIntegrityPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContainerdIntegrityPolicySpec   `json:"spec,omitempty"`
	Status ContainerdIntegrityPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ContainerdIntegrityPolicyList contains a list of ContainerdIntegrityPolicy.
type ContainerdIntegrityPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContainerdIntegrityPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ContainerdIntegrityPolicy{}, &ContainerdIntegrityPolicyList{})
}
