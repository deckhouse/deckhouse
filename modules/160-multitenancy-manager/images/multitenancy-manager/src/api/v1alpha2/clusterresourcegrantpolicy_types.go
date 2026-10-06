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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"controller/api/v1alpha1"
)

// ClusterResourceGrantPolicySpec defines the desired state of ClusterResourceGrantPolicy.
//
// A policy applies to a namespace of a project when every selector it sets matches: ProjectSelector
// the labels of the Project object, NamespaceSelector the labels of the namespace. A policy that sets
// neither selects nothing on its own; it is a library policy, applied through the grantPolicies of a
// ProjectTemplate. An explicit empty selector matches everything.
type ClusterResourceGrantPolicySpec struct {
	// ProjectSelector selects projects by the labels of the Project object. Every namespace of a
	// selected project, including an additional namespace created later, is covered unless
	// NamespaceSelector narrows it. Unset, any project is taken.
	// +optional
	ProjectSelector *metav1.LabelSelector `json:"projectSelector,omitempty"`

	// NamespaceSelector selects namespaces of projects by the labels of the namespace. Unset, every
	// namespace of the selected projects is taken.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// Resources holds the per-resource allow-list/default entries applied to matched namespaces.
	// +required
	Resources []v1alpha1.GrantResource `json:"resources"`
}

// IsLibrary reports whether the policy sets no selector, so it applies only through the ProjectTemplates
// that reference it.
func (s *ClusterResourceGrantPolicySpec) IsLibrary() bool {
	return s.ProjectSelector == nil && s.NamespaceSelector == nil
}

// ClusterResourceGrantPolicyStatus defines the observed state of ClusterResourceGrantPolicy.
type ClusterResourceGrantPolicyStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the current state of the ClusterResourceGrantPolicy resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:scope=Cluster,shortName=crgp
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ClusterResourceGrantPolicy is the Schema for the clusterresourcegrantpolicies API.
type ClusterResourceGrantPolicy struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ClusterResourceGrantPolicy.
	// +required
	Spec ClusterResourceGrantPolicySpec `json:"spec"`

	// status defines the observed state of ClusterResourceGrantPolicy.
	// +optional
	Status ClusterResourceGrantPolicyStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterResourceGrantPolicyList contains a list of ClusterResourceGrantPolicy.
type ClusterResourceGrantPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ClusterResourceGrantPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterResourceGrantPolicy{}, &ClusterResourceGrantPolicyList{})
}
