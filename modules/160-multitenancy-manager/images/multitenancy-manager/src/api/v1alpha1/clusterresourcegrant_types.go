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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required. Any new fields you add must have json tags for the fields to be serialized.

// ClusterResourceGrantSpec is the object-quota pool. Objects is keyed by grantable-resource name → granted
// name (or "*") → measure → limit (a resource.Quantity; -1 means unlimited).
type ClusterResourceGrantSpec struct {
	// Objects holds the per-class object-quota limits.
	// +optional
	Objects map[string]map[string]map[string]resource.Quantity `json:"objects,omitempty"`
}

// ClusterResourceGrantMeasureStatus is the live usage of one (resource, name, measure) tuple. On the pool
// object Used/Limit equal the project total; on a rendered per-namespace object Used/Limit are
// this namespace's, and ProjectUsed/ProjectLimit carry the project total.
type ClusterResourceGrantMeasureStatus struct {
	// Resource is the grantable resource name (e.g. storageclasses).
	// +required
	Resource string `json:"resource"`

	// Name is the granted name or "*".
	// +required
	Name string `json:"name"`

	// Measure is the measure key (e.g. requests.storage, services).
	// +required
	Measure string `json:"measure"`

	// Used is the effective usage at this scope.
	// +optional
	Used resource.Quantity `json:"used,omitempty"`

	// Limit is the effective limit at this scope (omitted = unlimited).
	// +optional
	Limit *resource.Quantity `json:"limit,omitempty"`

	// ProjectUsed is the project-wide usage (set on rendered per-namespace objects).
	// +optional
	ProjectUsed *resource.Quantity `json:"projectUsed,omitempty"`

	// ProjectLimit is the project-wide limit (set on rendered per-namespace objects).
	// +optional
	ProjectLimit *resource.Quantity `json:"projectLimit,omitempty"`
}

// ClusterResourceGrantStatus is the observed usage.
type ClusterResourceGrantStatus struct {
	// ObservedGeneration is the controller generation that produced this status.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Objects lists usage per (resource, name, measure).
	// +optional
	Objects []ClusterResourceGrantMeasureStatus `json:"objects,omitempty"`

	// Conditions represent the current state of the ClusterResourceGrant resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=crg
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ClusterResourceGrant is the object-quota resource: spec is the project pool (authored in the control
// namespace, cluster-admin only), status is the live usage. The controller renders read-only
// per-namespace copies into each workload namespace.
type ClusterResourceGrant struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the object-quota pool.
	// +optional
	Spec ClusterResourceGrantSpec `json:"spec,omitzero"`

	// status defines the observed usage.
	// +optional
	Status ClusterResourceGrantStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterResourceGrantList contains a list of ClusterResourceGrant.
type ClusterResourceGrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ClusterResourceGrant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterResourceGrant{}, &ClusterResourceGrantList{})
}
