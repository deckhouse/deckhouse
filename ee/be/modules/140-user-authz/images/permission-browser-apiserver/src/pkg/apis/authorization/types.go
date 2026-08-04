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
	// User is the user to check access for. If empty, uses the authenticated
	// user (self mode). A non-empty value additionally requires create on the
	// bulksubjectaccessreviews/nonself subresource.
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

// +genclient
// +genclient:nonNamespaced
// +genclient:onlyVerbs=create
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WhoCan answers the reverse-RBAC question: given an action (verb on a resource,
// optionally scoped to a namespace/name/subresource), which subjects (Users,
// Groups, ServiceAccounts) are allowed to perform it.
// This resource is ephemeral - it is not stored, only created.
type WhoCan struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Spec describes the action to resolve subjects for.
	Spec WhoCanSpec

	// Status is filled in by the server with the subjects allowed to perform the action.
	Status WhoCanStatus
}

// WhoCanSpec is the specification for a who-can request.
type WhoCanSpec struct {
	// ResourceAttributes describes the resource action to resolve subjects for.
	// +optional
	ResourceAttributes *ResourceAttributes

	// NonResourceAttributes describes the non-resource action to resolve subjects for.
	// +optional
	NonResourceAttributes *NonResourceAttributes
}

// WhoCanStatus contains the subjects allowed to perform the requested action.
type WhoCanStatus struct {
	// Users is the list of user names allowed to perform the action.
	// +optional
	Users []string

	// Groups is the list of group names allowed to perform the action.
	// +optional
	Groups []string

	// ServiceAccounts is the list of service accounts allowed to perform the action.
	// +optional
	ServiceAccounts []ServiceAccountReference

	// EvaluationError contains any non-fatal error encountered while resolving subjects.
	// +optional
	EvaluationError string
}

// ServiceAccountReference identifies a ServiceAccount subject by namespace and name.
type ServiceAccountReference struct {
	// Namespace is the namespace of the ServiceAccount.
	Namespace string

	// Name is the name of the ServiceAccount.
	Name string
}

// +genclient
// +genclient:nonNamespaced
// +genclient:onlyVerbs=create
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubjectAccessReport answers the forward question "what is this subject allowed
// to do", with the whole aggregation done server-side.
// This resource is ephemeral - it is not stored, only created.
type SubjectAccessReport struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Spec describes the subject to report on.
	Spec SubjectAccessReportSpec

	// Status is filled in by the server with the subject's effective access.
	Status SubjectAccessReportStatus
}

// SubjectAccessReportSpec is the specification for a subject access report.
type SubjectAccessReportSpec struct {
	// Subject is the subject to report on. When empty, the report is built for
	// the authenticated caller (self mode).
	// +optional
	Subject *SubjectReference

	// Groups lists additional groups to attribute to the subject.
	// +optional
	Groups []string

	// ResolveGroups enables resolving the subject's groups from the Group
	// catalog. Defaults to true.
	// +optional
	ResolveGroups *bool

	// Namespaces restricts the namespaced part of the report.
	// +optional
	Namespaces []string

	// ExpandWildcards expands wildcard rules into concrete resources.
	// Defaults to true.
	// +optional
	ExpandWildcards *bool
}

// SubjectReference identifies the subject a report is built for.
type SubjectReference struct {
	// Kind is one of User, Group or ServiceAccount.
	Kind string

	// Name is the user name, group name or ServiceAccount name.
	Name string

	// Namespace is the ServiceAccount namespace.
	// +optional
	Namespace string
}

// SubjectAccessReportStatus contains the subject's effective access.
type SubjectAccessReportStatus struct {
	// Subject echoes the subject the report was built for.
	Subject ResolvedSubject

	// RoleAssignments lists every binding that grants the subject a role.
	// +optional
	RoleAssignments []RoleAssignment

	// Scopes holds the access grouped by scope.
	// +optional
	Scopes []AccessScope

	// Notes carries non-fatal remarks about how the report was built.
	// +optional
	Notes []string

	// EvaluationError contains any non-fatal error encountered while building
	// the report.
	// +optional
	EvaluationError string

	// Truncated is true when output limits were hit.
	// +optional
	Truncated bool
}

// ResolvedSubject describes the identity the report was built for.
type ResolvedSubject struct {
	// Kind is one of User, Group or ServiceAccount.
	Kind string

	// Name is the subject name.
	Name string

	// Namespace is the ServiceAccount namespace, if applicable.
	// +optional
	Namespace string

	// Groups lists the groups taken into account, with their origin.
	// +optional
	Groups []ResolvedGroup
}

// ResolvedGroup is a group attributed to the subject and where it came from.
type ResolvedGroup struct {
	// Name is the RBAC group name.
	Name string

	// Source is one of: explicit, implicit, catalog, caller.
	Source string

	// Via is the chain of nested groups that led to this group.
	// +optional
	Via []string
}

// RoleAssignment is a binding that grants the subject a role.
type RoleAssignment struct {
	// BindingKind is ClusterRoleBinding or RoleBinding.
	BindingKind string

	// BindingName is the name of the binding.
	BindingName string

	// Namespace is the RoleBinding namespace, empty for ClusterRoleBindings.
	// +optional
	Namespace string

	// RoleKind is ClusterRole or Role.
	RoleKind string

	// RoleName is the name of the referenced role.
	RoleName string

	// MatchedBy describes which subject entry of the binding matched.
	MatchedBy SubjectMatch

	// Role carries display metadata of the referenced role.
	Role RoleDescriptor
}

// SubjectMatch describes how the subject matched a binding.
type SubjectMatch struct {
	// Kind is User, Group or ServiceAccount.
	Kind string

	// Name is the name of the matched subject entry.
	Name string
}

// RoleDescriptor carries the display metadata of a role.
type RoleDescriptor struct {
	// Kind is the role model object type.
	// +optional
	Kind string

	// Scope is one of system, subsystem, namespace, project.
	// +optional
	Scope string

	// Level is the access level.
	// +optional
	Level string

	// Subsystem is the subsystem name for subsystem-scoped roles.
	// +optional
	Subsystem string

	// Deprecated is true for the compatibility alias roles.
	// +optional
	Deprecated bool

	// Titles holds the localized display names keyed by language.
	// +optional
	Titles map[string]string

	// Descriptions holds the localized descriptions keyed by language.
	// +optional
	Descriptions map[string]string
}

// AccessScope is the access the subject has in one area.
type AccessScope struct {
	// Cluster is true for the cluster-wide scope.
	// +optional
	Cluster bool

	// Namespaces lists the namespaces this scope applies to.
	// +optional
	Namespaces []string

	// Resources lists the resource access granted in this scope.
	// +optional
	Resources []ResourceAccess

	// NonResourceRules lists the non-resource URL access granted in this scope.
	// +optional
	NonResourceRules []NonResourceAccess

	// Caveat describes admission-level restrictions.
	// +optional
	Caveat AccessCaveat
}

// ResourceAccess is the set of verbs the subject may use on one resource type.
type ResourceAccess struct {
	// Group is the API group, empty for the core group.
	// +optional
	Group string

	// Resource is the resource name, possibly with a subresource.
	Resource string

	// Verbs is the union of the verbs granted by all sources.
	Verbs []string

	// ViaWildcard is true when this row came from a wildcard rule.
	// +optional
	ViaWildcard bool

	// ResourceNames restricts the access to individually named objects.
	// +optional
	ResourceNames []string

	// Sources describes where the access comes from.
	// +optional
	Sources []AccessSource

	// ViaVerbWildcard is true when the access comes from a "verbs: [*]" rule, so
	// Verbs above is a readable sample rather than the complete list.
	// +optional
	ViaVerbWildcard bool
}

// NonResourceAccess is the set of verbs the subject may use on a non-resource URL.
type NonResourceAccess struct {
	// Path is the non-resource URL.
	Path string

	// Verbs is the union of the verbs granted by all sources.
	Verbs []string

	// Sources describes where the access comes from.
	// +optional
	Sources []AccessSource

	// ViaVerbWildcard is true when the access comes from a "verbs: [*]" rule.
	// +optional
	ViaVerbWildcard bool
}

// AccessSource attributes a slice of the access to the binding and role that
// granted it.
type AccessSource struct {
	// Verbs is the subset of the row's verbs granted by this source.
	Verbs []string

	// BindingKind is ClusterRoleBinding or RoleBinding.
	BindingKind string

	// BindingName is the name of the binding.
	BindingName string

	// BindingNamespace is the RoleBinding namespace.
	// +optional
	BindingNamespace string

	// RoleKind is ClusterRole or Role.
	RoleKind string

	// RoleName is the name of the referenced role.
	RoleName string

	// MatchedBy describes which subject entry of the binding matched.
	MatchedBy SubjectMatch

	// Role carries display metadata of the referenced role.
	Role RoleDescriptor

	// ViaVerbWildcard is true when this source grants the access through a
	// "verbs: [*]" rule.
	// +optional
	ViaVerbWildcard bool
}

// +genclient
// +genclient:nonNamespaced
// +genclient:onlyVerbs=create
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RoleAccessReport answers "what does this role grant", the catalogue side of
// the question SubjectAccessReport answers for a subject. It exists for the
// export a security officer keeps and a regulator reads: which resources are
// covered by which role, in a form that can be diffed against the previous one.
// This resource is ephemeral - it is not stored, only created.
type RoleAccessReport struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Spec describes the roles to report on.
	Spec RoleAccessReportSpec

	// Status is filled in by the server with what those roles grant.
	Status RoleAccessReportStatus
}

// RoleAccessReportSpec is the specification for a role access report.
type RoleAccessReportSpec struct {
	// Model selects the role model to report on: "primary" for the scope-based
	// roles, "legacy" for the access levels of ClusterAuthorizationRule.
	// Defaults to primary.
	// +optional
	Model string

	// Roles narrows the report. An empty selection reports every role of the
	// model.
	// +optional
	Roles RoleSelection

	// ExpandWildcards expands wildcard rules into concrete resources against
	// the discovery snapshot. Defaults to true.
	// +optional
	ExpandWildcards *bool

	// IncludeComposition reports which capability contributed each row, and the
	// list of capabilities a role aggregates. Defaults to false: the plain
	// matrix is what most of the export needs.
	// +optional
	IncludeComposition *bool
}

// RoleSelection narrows which roles a report covers. The fields are combined
// with AND; an empty selection matches every role of the model.
type RoleSelection struct {
	// Names lists roles by name.
	// +optional
	Names []string

	// Scopes lists the scopes to report on: namespace, project, subsystem,
	// system. Primary model only.
	// +optional
	Scopes []string

	// AccessLevels lists the access levels to report on. Legacy model only.
	// +optional
	AccessLevels []string
}

// RoleAccessReportStatus contains what the selected roles grant.
type RoleAccessReportStatus struct {
	// Snapshot describes when and against what the report was built.
	Snapshot ReportSnapshot

	// Roles holds one entry per reported role, ordered by name.
	// +optional
	Roles []RoleAccess

	// Notes carries non-fatal remarks about how the report was built.
	// +optional
	Notes []string

	// Truncated is true when output limits were hit.
	// +optional
	Truncated bool
}

// ReportSnapshot is what makes a report reproducible: the same cluster,
// unchanged, must produce the same document, and a reader must be able to tell
// what the document was built from.
type ReportSnapshot struct {
	// Time is when the report was built.
	Time metav1.Time

	// Model is the role model the report covers.
	Model string

	// ExpandedWildcards is true when wildcard rules were expanded.
	// +optional
	ExpandedWildcards bool

	// DiscoveryResources is the number of resources in the discovery snapshot
	// the wildcards were expanded against. Without it a wildcard row cannot be
	// interpreted: "every resource" means one thing on a cluster with
	// virtualization installed and another without it.
	// +optional
	DiscoveryResources int

	// Digest is a hash over the canonical form of the reported roles. Two
	// reports of an unchanged cluster carry the same digest, so a reader can
	// tell "nothing changed" from "I did not look".
	// +optional
	Digest string
}

// RoleAccess is what one role grants.
type RoleAccess struct {
	// Name is the ClusterRole name, or the access level in the legacy model.
	Name string

	// Role carries the display metadata of the role.
	// +optional
	Role RoleDescriptor

	// LegacyNames lists the names this role had in the previous model. There
	// can be more than one: the rename folded the kubernetes-suffixed variants
	// into a single role. The export carries them so a document can be compared
	// against one issued before the rename.
	// +optional
	LegacyNames []string

	// Namespaced is true when the role only ever applies inside a namespace.
	// Its cluster-scoped rules, if any, are left out: they exist in RBAC but
	// can never be exercised, and an export that lists them overstates access.
	// +optional
	Namespaced bool

	// Composition lists the capabilities the role aggregates, and for the
	// legacy model the roles an access level binds. Filled when
	// spec.includeComposition is set.
	// +optional
	Composition []RoleComponent

	// Resources lists the resource access the role grants.
	// +optional
	Resources []ResourceAccess

	// NonResourceRules lists the non-resource URL access the role grants.
	// +optional
	NonResourceRules []NonResourceAccess

	// Truncated is true when this role alone hit the output limits.
	// +optional
	Truncated bool

	// Notes carries remarks about this role alone.
	// +optional
	Notes []string
}

// RoleComponent is one part a role is assembled from: a capability in the
// primary model, a bound ClusterRole in the legacy one.
type RoleComponent struct {
	// Name is the ClusterRole name of the component.
	Name string

	// Role carries the display metadata of the component.
	// +optional
	Role RoleDescriptor
}

// AccessCaveat describes restrictions applied on top of RBAC by admission
// webhooks.
type AccessCaveat struct {
	// ProtectedVerbs is true when the scope grants webhook-restricted verbs.
	// +optional
	ProtectedVerbs bool

	// Superadmin is true when the subject bypasses the restriction.
	// +optional
	Superadmin bool

	// SuperadminNamespaces lists the namespaces where the subject is superadmin.
	// +optional
	SuperadminNamespaces []string

	// RestrictedNamespaces lists the namespaces where the restriction applies.
	// +optional
	RestrictedNamespaces []string
}
