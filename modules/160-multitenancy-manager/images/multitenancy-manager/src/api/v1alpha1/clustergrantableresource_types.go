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
	// EnforcementExternal means the module's own webhook enforces; we only materialize the catalog/quota.
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

// GrantedResource is the GVK of the cluster-scoped resource being governed. When absent the
// grant is value-backed (the granted names are values of a reference field, e.g. loadBalancerClass).
type GrantedResource struct {
	// APIVersion is the group/version of the granted resource (e.g. storage.k8s.io/v1).
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

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

// PathOverride overrides FieldPath for specific groups/versions, because a field may sit at
// different paths across versions.
type PathOverride struct {
	// APIGroups restricts this override to these groups (empty = any matched group).
	// +optional
	APIGroups []string `json:"apiGroups,omitempty"`

	// APIVersions restricts this override to these versions (empty = any matched version).
	// +optional
	APIVersions []string `json:"apiVersions,omitempty"`

	// FieldPath is the JSONPath to the granted name for the matched group/versions.
	// +required
	FieldPath string `json:"fieldPath"`
}

// MatchPredicate guards a usage reference: it applies only when the predicate holds on the object.
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

// QuantityMeasure is a summable quantity field; Name is the measure (quota) key.
type QuantityMeasure struct {
	// Name is the measure key used in GrantQuota (e.g. requests.storage).
	// +required
	Name string `json:"name"`

	// FieldPath is the JSONPath to the resource.Quantity value to sum.
	// +required
	FieldPath string `json:"fieldPath"`
}

// UsageReference declares where a granted name is referenced and what is measurable there.
type UsageReference struct {
	// Rule matches which usage objects this reference applies to (groups/versions/resources).
	// +required
	Rule UsageRule `json:"rule"`

	// FieldPath is the default JSONPath to the granted object's name (a string), for all matched
	// group/versions. May target an annotation, e.g. $.metadata.annotations['ipam.cilium.io/ip-pool'].
	// +required
	FieldPath string `json:"fieldPath"`

	// Paths overrides FieldPath per group/version when the field moved between versions.
	// +optional
	Paths []PathOverride `json:"paths,omitempty"`

	// Match guards this reference; it applies only when the predicate holds on the object.
	// +optional
	Match *MatchPredicate `json:"match,omitempty"`

	// Countable, if true, lets the count of these usage objects be limited; the measure key is the
	// resource plural.
	// +optional
	Countable bool `json:"countable,omitempty"`

	// Quantities are summable quantity fields; each Name is a measure key.
	// +optional
	Quantities []QuantityMeasure `json:"quantities,omitempty"`
}

// ClusterGrantableResourceSpec is the desired state of a ClusterGrantableResource.
type ClusterGrantableResourceSpec struct {
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

	// CoerceToDefault, when true, makes the /defaults webhook rewrite a referencing value that is not
	// available to the project to the project default, instead of letting /is-granted reject it. Enable
	// it only for fields a built-in admission controller pre-populates with a cluster default that the
	// project may not allow — e.g. PVC spec.storageClassName (DefaultStorageClass). For fields with no
	// such defaulter, leave it false so an explicit out-of-list value is rejected, not silently rewritten.
	// +optional
	CoerceToDefault bool `json:"coerceToDefault,omitempty"`

	// UsageReferences declares where the granted name is referenced and what is measurable.
	// +optional
	UsageReferences []UsageReference `json:"usageReferences,omitempty"`
}

// ClusterGrantableResourceStatus is the observed state of a ClusterGrantableResource.
type ClusterGrantableResourceStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the current state of the resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=cgr
// +kubebuilder:printcolumn:name="Granted",type=string,JSONPath=`.spec.grantedResource.kind`
// +kubebuilder:printcolumn:name="Enforcement",type=string,JSONPath=`.spec.enforcement`
// +kubebuilder:printcolumn:name="Default",type=string,JSONPath=`.spec.defaultAvailability`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ClusterGrantableResource registers a cluster-scoped resource as grant-controllable.
type ClusterGrantableResource struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ClusterGrantableResource.
	// +required
	Spec ClusterGrantableResourceSpec `json:"spec"`

	// status defines the observed state of ClusterGrantableResource.
	// +optional
	Status ClusterGrantableResourceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterGrantableResourceList contains a list of ClusterGrantableResource.
type ClusterGrantableResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ClusterGrantableResource `json:"items"`
}

// IsValueBacked reports whether the resource is value-backed (no grantedResource GVK).
func (r *ClusterGrantableResource) IsValueBacked() bool {
	return r.Spec.GrantedResource == nil || r.Spec.GrantedResource.Kind == ""
}

func init() {
	SchemeBuilder.Register(&ClusterGrantableResource{}, &ClusterGrantableResourceList{})
}
