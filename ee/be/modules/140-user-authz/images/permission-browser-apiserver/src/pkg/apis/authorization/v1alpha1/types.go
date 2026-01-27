/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +genclient:onlyVerbs=create
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BulkSubjectAccessReview checks whether a user or group can perform a set of actions.
// This resource is ephemeral - it is not stored, only created.
type BulkSubjectAccessReview struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec holds information about the request being evaluated
	Spec BulkSubjectAccessReviewSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`

	// Status is filled in by the server and indicates whether the requests are allowed or not
	// +optional
	Status BulkSubjectAccessReviewStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// BulkSubjectAccessReviewSpec is the specification for a bulk access review request
type BulkSubjectAccessReviewSpec struct {
	// User is the user to check access for. If empty, uses the authenticated user (self mode).
	// +optional
	User string `json:"user,omitempty" protobuf:"bytes,1,opt,name=user"`

	// UID information about the requesting user.
	// +optional
	UID string `json:"uid,omitempty" protobuf:"bytes,2,opt,name=uid"`

	// Groups is the list of groups the user belongs to.
	// +optional
	// +listType=atomic
	Groups []string `json:"groups,omitempty" protobuf:"bytes,3,rep,name=groups"`

	// Extra corresponds to the user.Info.GetExtra() method from the authenticator.
	// +optional
	Extra map[string]ExtraValue `json:"extra,omitempty" protobuf:"bytes,4,rep,name=extra"`

	// Requests is the list of resource access requests to check
	// +listType=atomic
	Requests []SubjectAccessReviewRequest `json:"requests" protobuf:"bytes,5,rep,name=requests"`
}

// ExtraValue masks the value so protobuf can generate
// +protobuf.nullable=true
// +protobuf.options.(gogoproto.goproto_stringer)=false
// +listType=atomic
type ExtraValue []string

// SubjectAccessReviewRequest contains the resource attributes for a single access check
type SubjectAccessReviewRequest struct {
	// ResourceAttributes describes information for a resource access request
	// +optional
	ResourceAttributes *ResourceAttributes `json:"resourceAttributes,omitempty" protobuf:"bytes,1,opt,name=resourceAttributes"`

	// NonResourceAttributes describes information for a non-resource access request
	// +optional
	NonResourceAttributes *NonResourceAttributes `json:"nonResourceAttributes,omitempty" protobuf:"bytes,2,opt,name=nonResourceAttributes"`
}

// ResourceAttributes includes the authorization attributes available for resource requests
type ResourceAttributes struct {
	// Namespace is the namespace of the action being requested.
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,1,opt,name=namespace"`

	// Verb is a kubernetes resource API verb, like: get, list, watch, create, update, delete, proxy.
	// +optional
	Verb string `json:"verb,omitempty" protobuf:"bytes,2,opt,name=verb"`

	// Group is the API Group of the Resource.
	// +optional
	Group string `json:"group,omitempty" protobuf:"bytes,3,opt,name=group"`

	// Version is the API Version of the Resource.
	// +optional
	Version string `json:"version,omitempty" protobuf:"bytes,4,opt,name=version"`

	// Resource is one of the existing resource types.
	// +optional
	Resource string `json:"resource,omitempty" protobuf:"bytes,5,opt,name=resource"`

	// Subresource is one of the existing resource types.
	// +optional
	Subresource string `json:"subresource,omitempty" protobuf:"bytes,6,opt,name=subresource"`

	// Name is the name of the resource being requested for a "get" or deleted for a "delete".
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,7,opt,name=name"`
}

// NonResourceAttributes includes the authorization attributes for non-resource requests
type NonResourceAttributes struct {
	// Path is the URL path of the request
	// +optional
	Path string `json:"path,omitempty" protobuf:"bytes,1,opt,name=path"`

	// Verb is the standard HTTP verb
	// +optional
	Verb string `json:"verb,omitempty" protobuf:"bytes,2,opt,name=verb"`
}

// BulkSubjectAccessReviewStatus contains the results of the access review
type BulkSubjectAccessReviewStatus struct {
	// Results contains the authorization check results for each request, in the same order as spec.requests
	// +listType=atomic
	Results []SubjectAccessReviewResult `json:"results" protobuf:"bytes,1,rep,name=results"`
}

// SubjectAccessReviewResult contains the result of a single authorization check
type SubjectAccessReviewResult struct {
	// Allowed is true if the action would be allowed, false otherwise.
	Allowed bool `json:"allowed" protobuf:"varint,1,opt,name=allowed"`

	// Denied is true if the action is explicitly denied, false otherwise.
	// A request might be denied even if not explicitly denied (e.g., no matching RBAC rules).
	// +optional
	Denied bool `json:"denied,omitempty" protobuf:"varint,2,opt,name=denied"`

	// Reason is optional and indicates why a request was allowed or denied.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,3,opt,name=reason"`

	// EvaluationError contains any error that occurred during authorization check.
	// +optional
	EvaluationError string `json:"evaluationError,omitempty" protobuf:"bytes,4,opt,name=evaluationError"`
}

// +genclient
// +genclient:nonNamespaced
// +genclient:onlyVerbs=get,list
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccessibleNamespace represents a namespace that the requesting user has access to.
// This is a read-only, computed resource similar to OpenShift Projects.
//
// LIMITATIONS:
// - Watch is NOT supported - clients must poll for updates
// - resourceVersion is always empty ("") - do not rely on it for caching
// - The list is computed at request time based on RBAC and multi-tenancy rules
type AccessibleNamespace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccessibleNamespaceList is a list of accessible namespaces for the requesting user
type AccessibleNamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is the list of accessible namespaces
	Items []AccessibleNamespace `json:"items" protobuf:"bytes,2,rep,name=items"`
}
