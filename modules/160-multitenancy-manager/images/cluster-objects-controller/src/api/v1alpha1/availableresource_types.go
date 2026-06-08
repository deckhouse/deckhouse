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

// NOTE: json tags are required. Any new fields you add must have json tags for the fields to be serialized.

// AvailableObject is one granted name available in a project, optionally the project default.
type AvailableObject struct {
	// Name is the granted object name available in the project.
	// +required
	Name string `json:"name"`

	// Default is true for the per-project default name.
	// +optional
	Default bool `json:"default,omitempty"`
}

// AvailableResourceStatus is the per-project catalog for one ClusterGrantableResource: the
// allowed names and the default. It carries no quota (that is GrantQuota).
type AvailableResourceStatus struct {
	// GrantedResourceKind is the kind of the granted resource (informational).
	// +optional
	GrantedResourceKind string `json:"grantedResourceKind,omitempty"`

	// Available lists the granted names available in this project, with the default flagged.
	// +listType=map
	// +listMapKey=name
	// +optional
	Available []AvailableObject `json:"available,omitempty"`

	// ObservedGeneration is the controller generation that produced this status.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=available
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=`.status.grantedResourceKind`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AvailableResource is the per-project, controller-owned catalog (discovery only) for one
// ClusterGrantableResource. metadata.name equals the grantable resource name.
type AvailableResource struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// status defines the observed catalog for this project.
	// +optional
	Status AvailableResourceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// AvailableResourceList contains a list of AvailableResource.
type AvailableResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []AvailableResource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AvailableResource{}, &AvailableResourceList{})
}
