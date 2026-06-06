/*
Copyright 2024 Flant JSC

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

// ApplicablePolicy is a reference to a ClusterObjectGrantPolicy that is referenced in a ClusterObjectGrant as applicable to it.
type ApplicablePolicy struct {
	// Name holds the reference to the ClusterObjectGrantPolicy resource.
	// +required
	Name string `json:"name"`

	// Default is an optional default resource name to apply in case the resource lacks the reference when created.
	// +optional
	Default string `json:"default,omitempty"`

	// Allowed is an explicit list of resource names granted to the matching projects.
	// A resource is granted if its name is listed here OR it matches AllowedSelector (union).
	// +optional
	Allowed []string `json:"allowed,omitempty"`

	// AllowedSelector grants every object of the policy's granted resource whose labels match
	// this selector. It is combined with Allowed as a union. An empty selector matches nothing
	// (use an explicit empty matchLabels/matchExpressions object to opt in to "match all").
	// +optional
	AllowedSelector *metav1.LabelSelector `json:"allowedSelector,omitempty"`
}

type AvailableClusterObjectStatusField struct {
	// Value holds the passed-through value of the status field from available resource.
	// +required
	Value string `json:"value"`

	// Descriptions holds a set of human readable descriptions of a field in different languages.
	// +required
	Descriptions ObjectStatusFieldDescription `json:"description"`
}

type AvailableClusterObjectRef struct {
	// Name is the name of the resource made available in the project.
	// +required
	Name string `json:"name"`

	// Default is set to `true` for resources that are selected as default cluster-wide
	// or via the `.spec.clusterObjectGrantPolicies[].default` field.
	// +optional
	Default bool `json:"default,omitempty"`

	// StatusFields holds the values of the additional resource fields exported by the
	// referenced ClusterObjectGrantPolicy via its objectPolicyStatusFields.
	// +optional
	StatusFields []AvailableClusterObjectStatusField `json:"statusFields,omitempty"`
}

// ProjectAvailability lists the objects made available in a single project that this grant matches.
type ProjectAvailability struct {
	// Project is the name of the matching project (namespace).
	// +required
	Project string `json:"project"`

	// Available is a grouped list of objects available in the project, grouped by resource kind.
	// +optional
	Available map[string][]AvailableClusterObjectRef `json:"available,omitempty"`
}

// ClusterObjectGrantSpec defines the desired state of ClusterObjectGrant
type ClusterObjectGrantSpec struct {
	// ProjectSelector selects the projects (by their namespace labels) this grant applies to.
	// A nil selector matches no projects; an explicit empty selector matches all project namespaces.
	// +optional
	ProjectSelector *metav1.LabelSelector `json:"projectSelector,omitempty"`

	// Policies holds a set of policy rules applied to the projects this grant matches.
	Policies []ApplicablePolicy `json:"clusterObjectGrantPolicies"`
}

// ClusterObjectGrantStatus defines the observed state of ClusterObjectGrant.
type ClusterObjectGrantStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Projects holds, per matching project, the objects made available there.
	// +listType=map
	// +listMapKey=project
	// +optional
	Projects []ProjectAvailability `json:"projects,omitempty"`

	// Conditions represent the current state of the ClusterObjectGrant resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ClusterObjectGrant is the Schema for the clusterobjectgrants API
type ClusterObjectGrant struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ClusterObjectGrant
	// +required
	Spec ClusterObjectGrantSpec `json:"spec"`

	// status defines the observed state of ClusterObjectGrant
	// +optional
	Status ClusterObjectGrantStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterObjectGrantList contains a list of ClusterObjectGrant
type ClusterObjectGrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ClusterObjectGrant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterObjectGrant{}, &ClusterObjectGrantList{})
}
