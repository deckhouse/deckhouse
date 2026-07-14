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

// DefaultingMode is the defaulting behaviour of a reference path.
// +kubebuilder:validation:Enum=None;FillEmpty;Coerce
type DefaultingMode string

const (
	// DefaultingNone validates only; the field is never filled in (use for opt-in toggle annotations).
	DefaultingNone DefaultingMode = "None"
	// DefaultingFillEmpty injects the per-project default into an empty field on CREATE.
	DefaultingFillEmpty DefaultingMode = "FillEmpty"
	// DefaultingCoerce is FillEmpty plus rewriting a non-empty value that is not available to the
	// project to the default (for fields a built-in admission controller pre-fills, e.g. PVC
	// storageClassName via DefaultStorageClass).
	DefaultingCoerce DefaultingMode = "Coerce"
)

// UsageRule matches the usage object like a webhook/RBAC rule. A resource may live in several
// groups and versions; "*" matches any group or version.
type UsageRule struct {
	// APIGroups are the API groups of the usage object (e.g. networking.k8s.io, extensions); "*" = any.
	// +required
	APIGroups []string `json:"apiGroups"`

	// APIVersions are the versions to match (e.g. v1, v1beta1); "*" = any.
	// +required
	APIVersions []string `json:"apiVersions"`

	// Resources are the plural names of the usage object (e.g. ingresses).
	// +required
	Resources []string `json:"resources"`
}

// MatchPredicate guards a field path: it applies only when the predicate holds on the object.
type MatchPredicate struct {
	// FieldPath is the JSONPath to the value tested by the predicate.
	// +required
	FieldPath string `json:"fieldPath"`

	// Equals matches when the value equals this string.
	// +optional
	Equals string `json:"equals,omitempty"`

	// In matches when the value is one of these strings.
	// +optional
	In []string `json:"in,omitempty"`
}

// FieldPath is one version-scoped location of the granted name within the usage object, together with
// its guard and defaulting behaviour. The entry whose APIGroups/APIVersions match the request's GVK
// wins (a scoped entry beats an unscoped one); an entry with empty scope is the fallback.
type FieldPath struct {
	// APIGroups restricts this entry to these groups (empty = any matched group).
	// +optional
	APIGroups []string `json:"apiGroups,omitempty"`

	// APIVersions restricts this entry to these versions (empty = any matched version).
	// +optional
	APIVersions []string `json:"apiVersions,omitempty"`

	// Path is the JSONPath to the granted object's name (a string). May target an annotation, e.g.
	// $.metadata.annotations['cert-manager.io/cluster-issuer'].
	// +required
	Path string `json:"path"`

	// Match guards this entry; it applies only when the predicate holds on the object.
	// +optional
	Match *MatchPredicate `json:"match,omitempty"`

	// Defaulting is the defaulting behaviour at this path: None (validate only), FillEmpty, or Coerce.
	// +optional
	// +kubebuilder:default=None
	Defaulting DefaultingMode `json:"defaulting,omitempty"`
}

// GrantableClusterResourceReferenceSpec declares one place a granted cluster resource is referenced
// (a validation/defaulting path). Any module may register one for its own resources.
type GrantableClusterResourceReferenceSpec struct {
	// GrantableClusterResourceName is the GrantableClusterResourceDefinition this path validates against.
	// +required
	GrantableClusterResourceName string `json:"grantableClusterResourceName"`

	// Rule matches which usage objects this reference applies to (groups/versions/resources).
	// +required
	Rule UsageRule `json:"rule"`

	// FieldPaths are the version-scoped locations of the granted name, with per-entry guard and
	// defaulting. At least one entry is required; provide an unscoped entry as the fallback.
	// +required
	// +kubebuilder:validation:MinItems=1
	FieldPaths []FieldPath `json:"fieldPaths"`
}

// GrantableClusterResourceReferenceStatus is the observed state of a GrantableClusterResourceReference.
type GrantableClusterResourceReferenceStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Bound is true when GrantableClusterResourceName resolves to an existing definition.
	// +optional
	Bound bool `json:"bound,omitempty"`

	// Conditions represent the current state of the reference (notably Bound: Resolved/UnknownResource).
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=gcrr
// +kubebuilder:printcolumn:name="Resource",type=string,JSONPath=`.spec.grantableClusterResourceName`
// +kubebuilder:printcolumn:name="Bound",type=boolean,JSONPath=`.status.bound`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GrantableClusterResourceReference registers a validation/defaulting path for a granted cluster resource.
type GrantableClusterResourceReference struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of GrantableClusterResourceReference.
	// +required
	Spec GrantableClusterResourceReferenceSpec `json:"spec"`

	// status defines the observed state of GrantableClusterResourceReference.
	// +optional
	Status GrantableClusterResourceReferenceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// GrantableClusterResourceReferenceList contains a list of GrantableClusterResourceReference.
type GrantableClusterResourceReferenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []GrantableClusterResourceReference `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GrantableClusterResourceReference{}, &GrantableClusterResourceReferenceList{})
}
