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
	// DefaultingCoerce is FillEmpty plus rewriting to the default a non-empty value that is not
	// available to the project or that the author did not set per the request's managedFields (for
	// fields a built-in admission controller pre-fills, e.g. PVC storageClassName via
	// DefaultStorageClass); an explicitly empty value is kept.
	DefaultingCoerce DefaultingMode = "Coerce"
)

// UsageRule matches the usage object like a webhook/RBAC rule. A resource may live in several
// groups and versions; "*" matches any group or version.
type UsageRule struct {
	// APIGroups are the API groups of the usage object (e.g. networking.k8s.io, extensions). "*" is
	// rejected: the webhook is registered for the listed groups only.
	// +required
	APIGroups []string `json:"apiGroups"`

	// APIVersions are the versions to match (e.g. v1, v1beta1); "*" = any.
	// +required
	APIVersions []string `json:"apiVersions"`

	// Resources are the plural names of the usage object (e.g. ingresses).
	// +required
	Resources []string `json:"resources"`
}

// MatchPredicate guards a field path: it applies only when the predicate holds on the object. A
// predicate with neither Equals nor In holds when FieldPath selects a string value.
type MatchPredicate struct {
	// FieldPath is the JSONPath to the value tested by the predicate. It must be a valid RFC 9535
	// JSONPath; the GrantableClusterResourceReference validating webhook rejects one that does not parse.
	// +required
	// +kubebuilder:validation:MaxLength=256
	FieldPath string `json:"fieldPath"`

	// Equals matches when the value equals this string.
	// +optional
	Equals string `json:"equals,omitempty"`

	// In matches when the value is one of these strings.
	// +optional
	In []string `json:"in,omitempty"`
}

// FieldPath is one scoped location of the granted name within the usage object, together with its
// guard and defaulting behaviour. Among the entries matching the request's resource/group/version
// whose Match holds on the object the most specific one wins (Resources beats APIGroups beats
// APIVersions, and at equal scope an entry with Match beats one without it); an entry with empty
// scope and no Match is the fallback, and "*" in a scope dimension counts as unrestricted.
// +kubebuilder:validation:XValidation:rule="!has(self.missingName) || has(self.missingPath)",message="missingName requires missingPath"
type FieldPath struct {
	// APIGroups restricts this entry to these groups (empty = any matched group).
	// +optional
	APIGroups []string `json:"apiGroups,omitempty"`

	// APIVersions restricts this entry to these versions (empty = any matched version).
	// +optional
	APIVersions []string `json:"apiVersions,omitempty"`

	// Resources restricts this entry to these plural resource names, e.g. pods, cronjobs
	// (empty = any matched resource). "*" behaves like an omitted field: it matches any resource and
	// adds no specificity.
	// +optional
	// +kubebuilder:validation:items:MinLength=1
	Resources []string `json:"resources,omitempty"`

	// Path is the JSONPath to the granted object's name (a string). May target an annotation, e.g.
	// $.metadata.annotations['cert-manager.io/cluster-issuer']. It must be a valid RFC 9535 JSONPath
	// whatever the Defaulting mode; the GrantableClusterResourceReference validating webhook rejects one
	// that does not parse. Validation evaluates the full JSONPath, but defaulting writes a single field,
	// so with Defaulting FillEmpty or Coerce the path must be a simple member path (see Defaulting).
	// +required
	// +kubebuilder:validation:MaxLength=256
	Path string `json:"path"`

	// MissingPath is the JSONPath to the places of the object where a name is expected but not given,
	// e.g. the elements of a list of templates that leave the field Path reads unset:
	// $.spec.volumeClaimTemplates[?!@.spec.storageClassName]. Such a place gets the cluster default of
	// the GrantableClusterResourceDefinition. With defaultFrom, the default it finds is decided as a name
	// the object adds: an object in which the path selects anything is denied when that default is not
	// available to the project, and allowed when there is no default. Without defaultFrom, such an
	// object is denied in a namespace to which a ClusterResourceGrantPolicy with an entry for the
	// definition applies. On UPDATE the nodes the old object already had are not counted. It must be a
	// valid RFC 9535 JSONPath; the GrantableClusterResourceReference validating webhook rejects one that
	// does not parse.
	// +optional
	// +kubebuilder:validation:MaxLength=256
	MissingPath string `json:"missingPath,omitempty"`

	// MissingName is the name the places MissingPath selects are decided as, instead of the cluster
	// default of the GrantableClusterResourceDefinition (or, without defaultFrom, instead of the denial
	// in a namespace a ClusterResourceGrantPolicy applies to). Set it when such places are served by one
	// object by name whatever the cluster default is, e.g. an Ingress without a class is served by the
	// controllers of the nginx IngressClass. An object in which MissingPath selects anything is denied
	// when this name is not available to the project. Requires MissingPath: the CRD rejects it alone.
	// +optional
	// +kubebuilder:validation:MaxLength=253
	MissingName string `json:"missingName,omitempty"`

	// Match guards this entry; it applies only when the predicate holds on the object.
	// +optional
	Match *MatchPredicate `json:"match,omitempty"`

	// Defaulting is the defaulting behaviour at this path: None (validate only), FillEmpty, or Coerce.
	// FillEmpty and Coerce require Path to be a simple member path, e.g. $.spec.storageClassName or
	// $.metadata.annotations['cert-manager.io/cluster-issuer']: wildcards, array indexes and filters
	// select no single field to write to. The GrantableClusterResourceReference validating webhook
	// rejects such a combination; with None any valid RFC 9535 JSONPath is allowed.
	// +optional
	// +kubebuilder:default=None
	Defaulting DefaultingMode `json:"defaulting,omitempty"`

	// Message is appended to the denial of a value at this path that is not available to the project,
	// and to the denial of an object for the places MissingPath selects.
	// +optional
	// +kubebuilder:validation:MaxLength=512
	Message string `json:"message,omitempty"`
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

	// FieldPaths are the scoped locations of the granted name, with per-entry guard and defaulting.
	// At least one entry is required. Every entry's scope must be a subset of Rule, and every
	// group/version/resource Rule matches must select an entry; an unscoped entry is the simplest
	// fallback. The GrantableClusterResourceReference validating webhook rejects a violation.
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

	// Conditions represent the current state of the reference: Bound (Resolved/UnknownResource) and
	// FieldPathsValid (Valid/InvalidFieldPaths).
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
