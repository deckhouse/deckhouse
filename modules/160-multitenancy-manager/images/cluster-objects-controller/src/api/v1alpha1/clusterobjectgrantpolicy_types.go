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

// NOTE: json tags are required.
// Any new fields you add must have json tags for the fields to be serialized.

// GrantedResource defines which resource should be taken under access control of ClusterObjectGrantPolicy.
type GrantedResource struct {
	metav1.TypeMeta `json:",inline"`

	// Defaults specifies how to distinguish which object of specified resource must be used as default.
	// +optional
	Defaults GrantedResourceDefaultingCriteria `json:"defaults,omitempty"`
}

// GrantedResourceDefaultingCriteria specifies how to distinguish which resource object must be used as default.
type GrantedResourceDefaultingCriteria struct {
	// AnnotationKey specifies which annotation on object marks it as the default one among others.
	// +optional
	AnnotationKey string `json:"annotationKey,omitempty"`
}

// GrantedResourceUsageReference defines where to look for references to the granted resource.
type GrantedResourceUsageReference struct {
	metav1.TypeMeta `json:",inline"`

	// FieldPath is a JSONPath reference to the field to lookup for granted resource usage.
	FieldPath string `json:"fieldPath"`
}

// ObjectPolicyStatusFields TODO: fill and document me
type ObjectPolicyStatusFields struct{}

// ClusterObjectGrantPolicySpec defines the desired state of ClusterObjectGrantPolicy
type ClusterObjectGrantPolicySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// GrantedResource defines which resource should be taken under access control of this policy.
	GrantedResource GrantedResource `json:"grantedResource"`

	// UsageReferences defines where to look for references to the granted resource and enforce access control policy.
	UsageReferences []GrantedResourceUsageReference `json:"usageReferences"`

	// ObjectPolicyStatusFields TODO: document this
	// +optional
	ObjectPolicyStatusFields ObjectPolicyStatusFields `json:"objectPolicyStatusFields,omitempty"`
}

// ClusterObjectGrantPolicyStatus defines the observed state of ClusterObjectGrantPolicy.
type ClusterObjectGrantPolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions represent the current state of the ClusterObjectGrantPolicy resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ClusterObjectGrantPolicy is the Schema for the clusterobjectgrantpolicies API
type ClusterObjectGrantPolicy struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ClusterObjectGrantPolicy
	// +required
	Spec ClusterObjectGrantPolicySpec `json:"spec"`

	// status defines the observed state of ClusterObjectGrantPolicy
	// +optional
	Status ClusterObjectGrantPolicyStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterObjectGrantPolicyList contains a list of ClusterObjectGrantPolicy
type ClusterObjectGrantPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ClusterObjectGrantPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterObjectGrantPolicy{}, &ClusterObjectGrantPolicyList{})
}
