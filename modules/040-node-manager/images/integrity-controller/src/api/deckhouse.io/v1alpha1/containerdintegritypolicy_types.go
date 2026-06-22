/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	ProtectedNamespace []string `json:"protectedNamespace,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="heritage=deckhouse"
// +kubebuilder:metadata:labels="module=node-manager"
// +kubebuilder:printcolumn:name="ProtectedNamespaces",type="string",JSONPath=".status.protectedNamespace",description="Namespaces protected by this policy"

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
