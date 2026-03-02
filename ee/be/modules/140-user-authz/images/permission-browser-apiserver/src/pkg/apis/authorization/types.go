/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package authorization

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
	metav1.TypeMeta
	metav1.ObjectMeta

	// Spec holds information about the request being evaluated
	Spec BulkSubjectAccessReviewSpec

	// Status is filled in by the server and indicates whether the requests are allowed or not
	Status BulkSubjectAccessReviewStatus
}

// BulkSubjectAccessReviewSpec is the specification for a bulk access review request
type BulkSubjectAccessReviewSpec struct {
	// User is the user to check access for. If empty, uses the authenticated user (self mode).
	// +optional
	User string

	// UID information about the requesting user.
	// +optional
	UID string

	// Groups is the list of groups the user belongs to.
	// +optional
	Groups []string

	// Extra corresponds to the user.Info.GetExtra() method from the authenticator.
	// +optional
	Extra map[string]ExtraValue

	// Requests is the list of resource access requests to check
	Requests []SubjectAccessReviewRequest
}

// ExtraValue masks the value so protobuf can generate
type ExtraValue []string

// SubjectAccessReviewRequest contains the resource attributes for a single access check
type SubjectAccessReviewRequest struct {
	// ResourceAttributes describes information for a resource access request
	// +optional
	ResourceAttributes *ResourceAttributes

	// NonResourceAttributes describes information for a non-resource access request
	// +optional
	NonResourceAttributes *NonResourceAttributes
}

// ResourceAttributes includes the authorization attributes available for resource requests
type ResourceAttributes struct {
	// Namespace is the namespace of the action being requested.
	// +optional
	Namespace string

	// Verb is a kubernetes resource API verb, like: get, list, watch, create, update, delete, proxy.
	// +optional
	Verb string

	// Group is the API Group of the Resource.
	// +optional
	Group string

	// Version is the API Version of the Resource.
	// +optional
	Version string

	// Resource is one of the existing resource types.
	// +optional
	Resource string

	// Subresource is one of the existing resource types.
	// +optional
	Subresource string

	// Name is the name of the resource being requested for a "get" or deleted for a "delete".
	// +optional
	Name string
}

// NonResourceAttributes includes the authorization attributes for non-resource requests
type NonResourceAttributes struct {
	// Path is the URL path of the request
	// +optional
	Path string

	// Verb is the standard HTTP verb
	// +optional
	Verb string
}

// BulkSubjectAccessReviewStatus contains the results of the access review
type BulkSubjectAccessReviewStatus struct {
	// Results contains the authorization check results for each request, in the same order as spec.requests
	Results []SubjectAccessReviewResult
}

// SubjectAccessReviewResult contains the result of a single authorization check
type SubjectAccessReviewResult struct {
	// Allowed is true if the action would be allowed, false otherwise.
	Allowed bool

	// Denied is true if the action is explicitly denied, false otherwise.
	// A request might be denied even if not explicitly denied (e.g., no matching RBAC rules).
	// +optional
	Denied bool

	// Reason is optional and indicates why a request was allowed or denied.
	// +optional
	Reason string

	// EvaluationError contains any error that occurred during authorization check.
	// +optional
	EvaluationError string
}

// +genclient
// +genclient:nonNamespaced
// +genclient:onlyVerbs=get,list
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccessibleNamespace represents a namespace that the requesting user has access to.
// This is a read-only, computed resource - watch is not supported.
type AccessibleNamespace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AccessibleNamespaceList is a list of accessible namespaces
type AccessibleNamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of accessible namespaces
	Items []AccessibleNamespace `json:"items"`
}
