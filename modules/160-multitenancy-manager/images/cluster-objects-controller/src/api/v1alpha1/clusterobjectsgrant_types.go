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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ApplicablePolicy is a reference to a ClusterObjectGrantPolicy that is referenced in a ClusterObjectsGrant as applicable to it.
type ApplicablePolicy struct {
	// Name holds the reference to the ClusterObjectGrantPolicy resource.
	// +required
	Name string `json:"name"`

	// Default is an optional default resource name to apply in case the resource lacks the reference when created.
	// +optional
	Default string `json:"default"`

	// Allowed is a list of all resource names that are granted to the project this ClusterObjectsGrant is related to.
	// +required
	Allowed []string `json:"allowed"`
}

type AvailableClusterObjectStatusField struct {
	// Value holds the passed-trough value of the status field from available resource.
	// +required
	Value string `json:"value"`

	// Descriptions holds a set of human readable descriptions of a field in diffirent languages.
	// +required
	Descriptions ObjectStatusFieldDescription `json:"description"`
}

type AvailableClusterObjectRef struct {
	// Name is the name of the resource made availeble in the project.
	// +required
	Name string `json:"name"`

	// Default is set to `true` for resources that are selected as default cluster-wide
	// or via the `.spec.clusterObjectGrantPolicies[].default` field.
	// +optional
	Default bool `json:"default,omitempty"`

	// StatusFields TODO: doc
	// +optional
	StatusFields []AvailableClusterObjectStatusField `json:"statusFields,omitempty"`
}

// ClusterObjectsGrantSpec defines the desired state of ClusterObjectsGrant
type ClusterObjectsGrantSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// Policies holds a set of policy rules applied to the project that this grant is related to.
	Policies []ApplicablePolicy `json:"clusterObjectGrantPolicies"`
}

// ClusterObjectsGrantStatus defines the observed state of ClusterObjectsGrant.
type ClusterObjectsGrantStatus struct {
	// Available is a grouped list of objects available in project, grouped by resource kind.
	Available map[string][]AvailableClusterObjectRef `json:"available"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=pgrant;pgrants

// ClusterObjectsGrant is the Schema for the clusterobjectsgrants API
type ClusterObjectsGrant struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ClusterObjectsGrant
	// +required
	Spec ClusterObjectsGrantSpec `json:"spec"`

	// status defines the observed state of ClusterObjectsGrant
	// +optional
	Status ClusterObjectsGrantStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterObjectsGrantList contains a list of ClusterObjectsGrant
type ClusterObjectsGrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ClusterObjectsGrant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterObjectsGrant{}, &ClusterObjectsGrantList{})
}
