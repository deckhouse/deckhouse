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

// EnforcementMode selects who enforces grants for a registered resource.
// +kubebuilder:validation:Enum=Managed;External
type EnforcementMode string

const (
	// EnforcementManaged means the built-in webhooks deny/default uses of the resource.
	EnforcementManaged EnforcementMode = "Managed"
	// EnforcementExternal means the module's own webhook enforces; we only materialize the catalog.
	EnforcementExternal EnforcementMode = "External"
)

// AvailabilityDefault is the baseline availability when no grant decides an object.
// +kubebuilder:validation:Enum=All;None
type AvailabilityDefault string

const (
	// AvailabilityAll means objects are usable unless a grant narrows (opt-out).
	AvailabilityAll AvailabilityDefault = "All"
	// AvailabilityNone means nothing is usable unless granted (opt-in).
	AvailabilityNone AvailabilityDefault = "None"
)

// GrantedResource identifies the cluster-scoped resource being governed by group and kind. When
// absent the grant is value-backed (the granted names are values of a reference field, e.g.
// loadBalancerClass). The served version is resolved via discovery.
type GrantedResource struct {
	// APIGroup is the API group of the granted resource (e.g. storage.k8s.io; "" for the core group).
	// +optional
	APIGroup string `json:"apiGroup,omitempty"`

	// Kind is the kind of the granted resource (e.g. StorageClass).
	// +optional
	Kind string `json:"kind,omitempty"`
}

// ResourceFilter matches granted objects by explicit names and/or labels.
type ResourceFilter struct {
	// Names is an explicit list of object names.
	// +optional
	Names []string `json:"names,omitempty"`

	// MatchLabels is a map of {key,value} pairs matched against the granted object labels.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// MatchExpressions is a list of label selector requirements matched against the granted object labels.
	// +optional
	MatchExpressions []metav1.LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// DefaultFrom marks the cluster-wide default object by an annotation (fallback default).
type DefaultFrom struct {
	// AnnotationKey is the annotation on a granted object that marks it as the cluster-wide default.
	// +optional
	AnnotationKey string `json:"annotationKey,omitempty"`
}

// GrantableClusterResourceDefinitionSpec is the desired state of a GrantableClusterResourceDefinition.
// It declares governance and baseline availability only; where the resource is referenced (and how it
// is validated/defaulted) lives in GrantableClusterResourceReference objects.
type GrantableClusterResourceDefinitionSpec struct {
	// GrantedResource is the cluster-scoped resource being governed (absent ⇒ value-backed).
	// +optional
	GrantedResource *GrantedResource `json:"grantedResource,omitempty"`

	// Enforcement is Managed (our webhooks) or External (the module's own webhook enforces).
	// +optional
	// +kubebuilder:default=Managed
	Enforcement EnforcementMode `json:"enforcement,omitempty"`

	// DefaultAvailability is the baseline when no grant allows an object: All or None.
	// +optional
	// +kubebuilder:default=All
	DefaultAvailability AvailabilityDefault `json:"defaultAvailability,omitempty"`

	// Excluded lists filters of objects never available to tenants, regardless of any grant (hard
	// deny). An object is excluded if it matches ANY of the filters (the filters are unioned), which
	// lets a registration express "available by default to set A OR set B" by excluding everything
	// outside both sets.
	// +optional
	Excluded []ResourceFilter `json:"excluded,omitempty"`

	// DefaultFrom marks the cluster-wide default object by annotation (fallback default).
	// +optional
	DefaultFrom *DefaultFrom `json:"defaultFrom,omitempty"`
}

// ResourceReferenceBinding is one GrantableClusterResourceReference bound to this definition.
type ResourceReferenceBinding struct {
	// Name is the GrantableClusterResourceReference object name.
	// +required
	Name string `json:"name"`

	// Resources are the usage-object plural resources the reference matches (from its rule).
	// +optional
	Resources []string `json:"resources,omitempty"`
}

// GrantableClusterResourceDefinitionStatus is the observed state of a GrantableClusterResourceDefinition.
type GrantableClusterResourceDefinitionStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// References is the reverse index of GrantableClusterResourceReference objects bound to this
	// definition (i.e. the validation/defaulting paths registered against it).
	// +optional
	References []ResourceReferenceBinding `json:"references,omitempty"`

	// ReferenceCount is the number of bound references (len(References)).
	// +optional
	ReferenceCount int `json:"referenceCount,omitempty"`

	// Conditions represent the current state of the resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=gcrd
// +kubebuilder:printcolumn:name="Granted",type=string,JSONPath=`.spec.grantedResource.kind`
// +kubebuilder:printcolumn:name="Enforcement",type=string,JSONPath=`.spec.enforcement`
// +kubebuilder:printcolumn:name="Default",type=string,JSONPath=`.spec.defaultAvailability`
// +kubebuilder:printcolumn:name="References",type=integer,JSONPath=`.status.referenceCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GrantableClusterResourceDefinition registers a cluster-scoped resource as grant-controllable.
type GrantableClusterResourceDefinition struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of GrantableClusterResourceDefinition.
	// +required
	Spec GrantableClusterResourceDefinitionSpec `json:"spec"`

	// status defines the observed state of GrantableClusterResourceDefinition.
	// +optional
	Status GrantableClusterResourceDefinitionStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// GrantableClusterResourceDefinitionList contains a list of GrantableClusterResourceDefinition.
type GrantableClusterResourceDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []GrantableClusterResourceDefinition `json:"items"`
}

// IsValueBacked reports whether the resource is value-backed (no grantedResource GVK).
func (r *GrantableClusterResourceDefinition) IsValueBacked() bool {
	return r.Spec.GrantedResource == nil || r.Spec.GrantedResource.Kind == ""
}

func init() {
	SchemeBuilder.Register(&GrantableClusterResourceDefinition{}, &GrantableClusterResourceDefinitionList{})
}
