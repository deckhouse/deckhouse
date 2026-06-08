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

// GrantResource is one entry of a grant: it references a ClusterGrantableResource and decides
// the per-project allow-list and default. It carries no quota (object quota lives on GrantQuota).
type GrantResource struct {
	// ResourceRef is the name of a ClusterGrantableResource.
	// +required
	ResourceRef string `json:"resourceRef"`

	// Allowed is an explicit list of granted object names (union with AllowedSelector).
	// +optional
	Allowed []string `json:"allowed,omitempty"`

	// AllowedSelector grants every object of the granted resource whose labels match (union with Allowed).
	// Only meaningful for object-backed resources.
	// +optional
	AllowedSelector *metav1.LabelSelector `json:"allowedSelector,omitempty"`

	// Denied lists object names explicitly excluded for matched projects (overrides Allowed).
	// +optional
	Denied []string `json:"denied,omitempty"`

	// DeniedSelector excludes granted objects whose labels match (overrides Allowed/AllowedSelector).
	// +optional
	DeniedSelector *metav1.LabelSelector `json:"deniedSelector,omitempty"`

	// Default is the per-project default name (overrides the registration's defaultFrom).
	// +optional
	Default string `json:"default,omitempty"`

	// AvailabilityDefault overrides the resource's defaultAvailability (All/None) for matched projects.
	// +optional
	AvailabilityDefault AvailabilityDefault `json:"availabilityDefault,omitempty"`
}

// ClusterObjectGrantSpec defines the desired state of ClusterObjectGrant.
type ClusterObjectGrantSpec struct {
	// ProjectSelector selects the Projects (by their labels, propagated to namespaces) this grant
	// applies to. A nil selector matches no projects; an explicit empty selector matches all.
	// +optional
	ProjectSelector *metav1.LabelSelector `json:"projectSelector,omitempty"`

	// Resources holds the per-resource allow-list/default entries applied to matched projects.
	// +required
	Resources []GrantResource `json:"resources"`
}

// ClusterObjectGrantStatus defines the observed state of ClusterObjectGrant.
type ClusterObjectGrantStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the current state of the ClusterObjectGrant resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=cog
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ClusterObjectGrant is the Schema for the clusterobjectgrants API.
type ClusterObjectGrant struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ClusterObjectGrant.
	// +required
	Spec ClusterObjectGrantSpec `json:"spec"`

	// status defines the observed state of ClusterObjectGrant.
	// +optional
	Status ClusterObjectGrantStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterObjectGrantList contains a list of ClusterObjectGrant.
type ClusterObjectGrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ClusterObjectGrant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterObjectGrant{}, &ClusterObjectGrantList{})
}
