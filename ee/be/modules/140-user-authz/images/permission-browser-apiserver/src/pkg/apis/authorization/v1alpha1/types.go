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
	// User is the user to check access for. If empty, uses the authenticated
	// user (self mode). A non-empty value additionally requires create on the
	// bulksubjectaccessreviews/nonself subresource.
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

// +genclient
// +genclient:nonNamespaced
// +genclient:onlyVerbs=create
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WhoCan answers the reverse-RBAC question: given an action (verb on a resource,
// optionally scoped to a namespace/name/subresource), which subjects (Users,
// Groups, ServiceAccounts) are allowed to perform it. This is similar to
// OpenShift's `oc policy who-can`.
//
// This resource is ephemeral - it is not stored, only created.
type WhoCan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec describes the action to resolve subjects for.
	Spec WhoCanSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`

	// Status is filled in by the server with the subjects allowed to perform the action.
	// +optional
	Status WhoCanStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// WhoCanSpec is the specification for a who-can request.
type WhoCanSpec struct {
	// ResourceAttributes describes the resource action to resolve subjects for.
	// +optional
	ResourceAttributes *ResourceAttributes `json:"resourceAttributes,omitempty" protobuf:"bytes,1,opt,name=resourceAttributes"`

	// NonResourceAttributes describes the non-resource action to resolve subjects for.
	// +optional
	NonResourceAttributes *NonResourceAttributes `json:"nonResourceAttributes,omitempty" protobuf:"bytes,2,opt,name=nonResourceAttributes"`
}

// WhoCanStatus contains the subjects allowed to perform the requested action.
type WhoCanStatus struct {
	// Users is the list of user names allowed to perform the action.
	// +optional
	// +listType=atomic
	Users []string `json:"users,omitempty" protobuf:"bytes,1,rep,name=users"`

	// Groups is the list of group names allowed to perform the action.
	// +optional
	// +listType=atomic
	Groups []string `json:"groups,omitempty" protobuf:"bytes,2,rep,name=groups"`

	// ServiceAccounts is the list of service accounts allowed to perform the action.
	// +optional
	// +listType=atomic
	ServiceAccounts []ServiceAccountReference `json:"serviceAccounts,omitempty" protobuf:"bytes,3,rep,name=serviceAccounts"`

	// EvaluationError contains any non-fatal error encountered while resolving subjects.
	// +optional
	EvaluationError string `json:"evaluationError,omitempty" protobuf:"bytes,4,opt,name=evaluationError"`
}

// ServiceAccountReference identifies a ServiceAccount subject by namespace and name.
type ServiceAccountReference struct {
	// Namespace is the namespace of the ServiceAccount.
	Namespace string `json:"namespace" protobuf:"bytes,1,opt,name=namespace"`

	// Name is the name of the ServiceAccount.
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`
}

// +genclient
// +genclient:nonNamespaced
// +genclient:onlyVerbs=create
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubjectAccessReport answers the forward question "what is this subject allowed
// to do", with the whole aggregation done server-side: every grant is expanded
// from the subject's bindings into resource rows carrying provenance (which
// binding, which role, and whether the subject matched directly or through a
// group).
//
// It replaces the client-side permission matrix built on top of
// BulkSubjectAccessReview, which required one authorization check per
// resource/verb/namespace combination.
//
// This resource is ephemeral - it is not stored, only created.
type SubjectAccessReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec describes the subject to report on.
	Spec SubjectAccessReportSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`

	// Status is filled in by the server with the subject's effective access.
	// +optional
	Status SubjectAccessReportStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// SubjectAccessReportSpec is the specification for a subject access report.
type SubjectAccessReportSpec struct {
	// Subject is the subject to report on. When empty, the report is built for
	// the authenticated caller (self mode). A non-empty value additionally
	// requires create on the subjectaccessreports/nonself subresource.
	// +optional
	Subject *SubjectReference `json:"subject,omitempty" protobuf:"bytes,1,opt,name=subject"`

	// Groups lists additional groups to attribute to the subject, on top of the
	// implicit ones and (when enabled) the ones resolved from the group catalog.
	// +optional
	// +listType=atomic
	Groups []string `json:"groups,omitempty" protobuf:"bytes,2,rep,name=groups"`

	// ResolveGroups enables resolving the subject's groups from the Group
	// catalog (groups.deckhouse.io), including nested groups. Defaults to true.
	// Ignored for Group subjects.
	// +optional
	ResolveGroups *bool `json:"resolveGroups,omitempty" protobuf:"varint,3,opt,name=resolveGroups"`

	// Namespaces restricts the namespaced part of the report to the listed
	// namespaces. When empty, every namespace where the subject has a
	// RoleBinding is reported.
	// +optional
	// +listType=atomic
	Namespaces []string `json:"namespaces,omitempty" protobuf:"bytes,4,rep,name=namespaces"`

	// ExpandWildcards expands wildcard rules ("*" api groups or resources) into
	// the concrete resources known to discovery. Defaults to true. When
	// disabled, wildcard rules are reported as a single "*" row.
	// +optional
	ExpandWildcards *bool `json:"expandWildcards,omitempty" protobuf:"varint,5,opt,name=expandWildcards"`
}

// Subject kinds accepted in SubjectAccessReport.spec.subject.
const (
	SubjectKindUser           = "User"
	SubjectKindGroup          = "Group"
	SubjectKindServiceAccount = "ServiceAccount"
)

// Origins of a group attributed to the subject, reported in ResolvedGroup.Source.
const (
	// GroupSourceExplicit marks a group passed in spec.groups.
	GroupSourceExplicit = "explicit"
	// GroupSourceImplicit marks a pseudo-group every subject of this kind
	// carries, such as system:authenticated.
	GroupSourceImplicit = "implicit"
	// GroupSourceCatalog marks a group resolved from the Group catalog.
	GroupSourceCatalog = "catalog"
	// GroupSourceCaller marks a group taken from the authenticated caller in
	// self mode.
	GroupSourceCaller = "caller"
)

// SubjectReference identifies the subject a report is built for.
type SubjectReference struct {
	// Kind is one of User, Group or ServiceAccount.
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`

	// Name is the user name, group name or ServiceAccount name.
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`

	// Namespace is the ServiceAccount namespace. Required for ServiceAccount
	// subjects, ignored otherwise.
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`
}

// SubjectAccessReportStatus contains the subject's effective access.
type SubjectAccessReportStatus struct {
	// Subject echoes the subject the report was built for, including the
	// groups that were actually taken into account.
	Subject ResolvedSubject `json:"subject" protobuf:"bytes,1,opt,name=subject"`

	// RoleAssignments lists every binding that grants the subject a role.
	// +optional
	// +listType=atomic
	RoleAssignments []RoleAssignment `json:"roleAssignments,omitempty" protobuf:"bytes,2,rep,name=roleAssignments"`

	// Scopes holds the access grouped by scope: one cluster-wide scope (grants
	// coming from ClusterRoleBindings) plus one scope per set of namespaces
	// sharing identical local access.
	// +optional
	// +listType=atomic
	Scopes []AccessScope `json:"scopes,omitempty" protobuf:"bytes,3,rep,name=scopes"`

	// Notes carries non-fatal remarks about how the report was built, for
	// example that a ClusterAuthorizationRule narrowed the result or that the
	// group catalog was unavailable.
	// +optional
	// +listType=atomic
	Notes []string `json:"notes,omitempty" protobuf:"bytes,4,rep,name=notes"`

	// EvaluationError contains any non-fatal error encountered while building
	// the report. A non-empty value means the report is partial.
	// +optional
	EvaluationError string `json:"evaluationError,omitempty" protobuf:"bytes,5,opt,name=evaluationError"`

	// Truncated is true when output limits were hit and the report is
	// incomplete.
	// +optional
	Truncated bool `json:"truncated,omitempty" protobuf:"varint,6,opt,name=truncated"`
}

// ResolvedSubject describes the identity the report was built for.
type ResolvedSubject struct {
	// Kind is one of User, Group or ServiceAccount.
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`

	// Name is the subject name.
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`

	// Namespace is the ServiceAccount namespace, if applicable.
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`

	// Groups lists the groups taken into account, with their origin.
	// +optional
	// +listType=atomic
	Groups []ResolvedGroup `json:"groups,omitempty" protobuf:"bytes,4,rep,name=groups"`
}

// ResolvedGroup is a group attributed to the subject and where it came from.
type ResolvedGroup struct {
	// Name is the RBAC group name.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// Source is one of: explicit (passed in spec.groups), implicit (attached to
	// every subject of this kind, e.g. system:authenticated), catalog (resolved
	// from a Group resource), caller (taken from the authenticated caller in
	// self mode).
	Source string `json:"source" protobuf:"bytes,2,opt,name=source"`

	// Via is the chain of nested groups that led to this group, for groups
	// resolved from the catalog.
	// +optional
	// +listType=atomic
	Via []string `json:"via,omitempty" protobuf:"bytes,3,rep,name=via"`
}

// RoleAssignment is a binding that grants the subject a role.
type RoleAssignment struct {
	// BindingKind is ClusterRoleBinding or RoleBinding.
	BindingKind string `json:"bindingKind" protobuf:"bytes,1,opt,name=bindingKind"`

	// BindingName is the name of the binding.
	BindingName string `json:"bindingName" protobuf:"bytes,2,opt,name=bindingName"`

	// Namespace is the RoleBinding namespace, empty for ClusterRoleBindings.
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`

	// RoleKind is ClusterRole or Role.
	RoleKind string `json:"roleKind" protobuf:"bytes,4,opt,name=roleKind"`

	// RoleName is the name of the referenced role.
	RoleName string `json:"roleName" protobuf:"bytes,5,opt,name=roleName"`

	// MatchedBy describes which subject entry of the binding matched.
	MatchedBy SubjectMatch `json:"matchedBy" protobuf:"bytes,6,opt,name=matchedBy"`

	// Role carries display metadata of the referenced role.
	Role RoleDescriptor `json:"role" protobuf:"bytes,7,opt,name=role"`
}

// SubjectMatch describes how the subject matched a binding: directly by name or
// through one of its groups. It lets clients show the access with and without
// group-derived grants without asking the server again.
type SubjectMatch struct {
	// Kind is User, Group or ServiceAccount.
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`

	// Name is the name of the matched subject entry: the group name when the
	// subject matched through a group, otherwise the subject's own name.
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`
}

// RoleDescriptor carries the display metadata of a role, taken from its
// rbac.deckhouse.io/* labels and meta.deckhouse.io annotations, so clients do
// not have to parse role names.
type RoleDescriptor struct {
	// Kind is the role model object type: role, capability, custom-role,
	// custom-capability, or empty for roles outside the model.
	// +optional
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`

	// Scope is one of system, subsystem, namespace, project, or empty when
	// unknown.
	// +optional
	Scope string `json:"scope,omitempty" protobuf:"bytes,2,opt,name=scope"`

	// Level is the access level: viewer, user, manager, admin, superadmin.
	// +optional
	Level string `json:"level,omitempty" protobuf:"bytes,3,opt,name=level"`

	// Subsystem is the subsystem name for subsystem-scoped roles.
	// +optional
	Subsystem string `json:"subsystem,omitempty" protobuf:"bytes,4,opt,name=subsystem"`

	// Deprecated is true for the compatibility alias roles.
	// +optional
	Deprecated bool `json:"deprecated,omitempty" protobuf:"varint,5,opt,name=deprecated"`

	// Titles holds the localized display names keyed by language ("en", "ru").
	// +optional
	Titles map[string]string `json:"titles,omitempty" protobuf:"bytes,6,rep,name=titles"`

	// Descriptions holds the localized descriptions keyed by language.
	// +optional
	Descriptions map[string]string `json:"descriptions,omitempty" protobuf:"bytes,7,rep,name=descriptions"`
}

// AccessScope is the access the subject has in one area: either cluster-wide
// (granted by ClusterRoleBindings and therefore applying in every namespace) or
// local to a set of namespaces that share identical access.
type AccessScope struct {
	// Cluster is true for the cluster-wide scope.
	// +optional
	Cluster bool `json:"cluster,omitempty" protobuf:"varint,1,opt,name=cluster"`

	// Namespaces lists the namespaces this scope applies to. Empty for the
	// cluster-wide scope.
	// +optional
	// +listType=atomic
	Namespaces []string `json:"namespaces,omitempty" protobuf:"bytes,2,rep,name=namespaces"`

	// Resources lists the resource access granted in this scope.
	// +optional
	// +listType=atomic
	Resources []ResourceAccess `json:"resources,omitempty" protobuf:"bytes,3,rep,name=resources"`

	// NonResourceRules lists the non-resource URL access granted in this scope.
	// +optional
	// +listType=atomic
	NonResourceRules []NonResourceAccess `json:"nonResourceRules,omitempty" protobuf:"bytes,4,rep,name=nonResourceRules"`

	// Caveat describes admission-level restrictions that RBAC alone does not
	// express.
	// +optional
	Caveat AccessCaveat `json:"caveat,omitempty" protobuf:"bytes,5,opt,name=caveat"`
}

// ResourceAccess is the set of verbs the subject may use on one resource type.
type ResourceAccess struct {
	// Group is the API group, empty for the core group.
	// +optional
	Group string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`

	// Resource is the resource name, possibly with a subresource
	// ("pods/log"), or "*" for an unexpanded wildcard.
	Resource string `json:"resource" protobuf:"bytes,2,opt,name=resource"`

	// Verbs is the union of the verbs granted by all sources below.
	// +listType=atomic
	Verbs []string `json:"verbs" protobuf:"bytes,3,rep,name=verbs"`

	// ViaWildcard is true when this row came from a wildcard rule rather than
	// from an explicitly named resource.
	// +optional
	ViaWildcard bool `json:"viaWildcard,omitempty" protobuf:"varint,4,opt,name=viaWildcard"`

	// ResourceNames restricts the access to individually named objects.
	// +optional
	// +listType=atomic
	ResourceNames []string `json:"resourceNames,omitempty" protobuf:"bytes,5,rep,name=resourceNames"`

	// Sources describes where the access comes from.
	// +optional
	// +listType=atomic
	Sources []AccessSource `json:"sources,omitempty" protobuf:"bytes,6,rep,name=sources"`

	// ViaVerbWildcard is true when the access comes from a "verbs: [*]" rule.
	// Verbs above then lists the common verbs the wildcard covers, not all of
	// them: the rule also grants the privileged ones RBAC checks separately
	// (escalate and bind on roles, impersonate on subjects, proxy on nodes).
	// Without this flag such a row would be indistinguishable from an explicit
	// grant of exactly those verbs.
	// +optional
	ViaVerbWildcard bool `json:"viaVerbWildcard,omitempty" protobuf:"varint,7,opt,name=viaVerbWildcard"`
}

// NonResourceAccess is the set of verbs the subject may use on a non-resource URL.
type NonResourceAccess struct {
	// Path is the non-resource URL, possibly with a trailing "*".
	Path string `json:"path" protobuf:"bytes,1,opt,name=path"`

	// Verbs is the union of the verbs granted by all sources below.
	// +listType=atomic
	Verbs []string `json:"verbs" protobuf:"bytes,2,rep,name=verbs"`

	// Sources describes where the access comes from.
	// +optional
	// +listType=atomic
	Sources []AccessSource `json:"sources,omitempty" protobuf:"bytes,3,rep,name=sources"`

	// ViaVerbWildcard is true when the access comes from a "verbs: [*]" rule.
	// +optional
	ViaVerbWildcard bool `json:"viaVerbWildcard,omitempty" protobuf:"varint,4,opt,name=viaVerbWildcard"`
}

// AccessSource attributes a slice of the access to the binding and role that
// granted it.
type AccessSource struct {
	// Verbs is the subset of the row's verbs granted by this source.
	// +listType=atomic
	Verbs []string `json:"verbs" protobuf:"bytes,1,rep,name=verbs"`

	// BindingKind is ClusterRoleBinding or RoleBinding.
	BindingKind string `json:"bindingKind" protobuf:"bytes,2,opt,name=bindingKind"`

	// BindingName is the name of the binding.
	BindingName string `json:"bindingName" protobuf:"bytes,3,opt,name=bindingName"`

	// BindingNamespace is the RoleBinding namespace, empty for
	// ClusterRoleBindings.
	// +optional
	BindingNamespace string `json:"bindingNamespace,omitempty" protobuf:"bytes,4,opt,name=bindingNamespace"`

	// RoleKind is ClusterRole or Role.
	RoleKind string `json:"roleKind" protobuf:"bytes,5,opt,name=roleKind"`

	// RoleName is the name of the referenced role.
	RoleName string `json:"roleName" protobuf:"bytes,6,opt,name=roleName"`

	// MatchedBy describes which subject entry of the binding matched.
	MatchedBy SubjectMatch `json:"matchedBy" protobuf:"bytes,7,opt,name=matchedBy"`

	// Role carries display metadata of the referenced role.
	Role RoleDescriptor `json:"role" protobuf:"bytes,8,opt,name=role"`

	// ViaVerbWildcard is true when this source grants the access through a
	// "verbs: [*]" rule, so its verbs are a readable sample rather than the
	// complete list.
	// +optional
	ViaVerbWildcard bool `json:"viaVerbWildcard,omitempty" protobuf:"varint,9,opt,name=viaVerbWildcard"`
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
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec describes the roles to report on.
	Spec RoleAccessReportSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`

	// Status is filled in by the server with what those roles grant.
	// +optional
	Status RoleAccessReportStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// RoleAccessReportSpec is the specification for a role access report.
type RoleAccessReportSpec struct {
	// Model selects the role model to report on: "primary" for the scope-based
	// roles, "legacy" for the access levels of ClusterAuthorizationRule.
	// Defaults to primary.
	// +optional
	Model string `json:"model,omitempty" protobuf:"bytes,1,opt,name=model"`

	// Roles narrows the report. An empty selection reports every role of the
	// model.
	// +optional
	Roles RoleSelection `json:"roles,omitempty" protobuf:"bytes,2,opt,name=roles"`

	// ExpandWildcards expands wildcard rules into concrete resources against
	// the discovery snapshot. Defaults to true.
	// +optional
	ExpandWildcards *bool `json:"expandWildcards,omitempty" protobuf:"varint,3,opt,name=expandWildcards"`

	// IncludeComposition reports which capability contributed each row, and the
	// list of capabilities a role aggregates. Defaults to false: the plain
	// matrix is what most of the export needs.
	// +optional
	IncludeComposition *bool `json:"includeComposition,omitempty" protobuf:"varint,4,opt,name=includeComposition"`

	// IncludeInventory adds every resource of the cluster to the report, not
	// only the ones some role grants. Defaults to false. It is what a coverage
	// review needs: the resources no role covers are the answer, and they
	// cannot be derived from the roles.
	// +optional
	IncludeInventory *bool `json:"includeInventory,omitempty" protobuf:"varint,5,opt,name=includeInventory"`
}

// RoleSelection narrows which roles a report covers. Names and Scopes are
// combined with AND; an empty selection matches every role of the model.
type RoleSelection struct {
	// Names lists roles by name.
	//
	// The legacy model has no role names -- it is the fixed list of access
	// levels -- so there a name is read as a level and joins AccessLevels
	// instead of narrowing it. Naming "Editor" in either field reports the
	// Editor level.
	// +optional
	// +listType=atomic
	Names []string `json:"names,omitempty" protobuf:"bytes,1,rep,name=names"`

	// Scopes lists the scopes to report on: namespace, project, subsystem,
	// system. Primary model only.
	// +optional
	// +listType=atomic
	Scopes []string `json:"scopes,omitempty" protobuf:"bytes,2,rep,name=scopes"`

	// AccessLevels lists the access levels to report on. Legacy model only.
	// +optional
	// +listType=atomic
	AccessLevels []string `json:"accessLevels,omitempty" protobuf:"bytes,3,rep,name=accessLevels"`

	// ExcludeCustom leaves out the roles created in this cluster, reporting
	// only the model the platform ships. A role named in Names is reported
	// either way. Primary model only. Defaults to false.
	// +optional
	ExcludeCustom bool `json:"excludeCustom,omitempty" protobuf:"varint,4,opt,name=excludeCustom"`
}

// RoleAccessReportStatus contains what the selected roles grant.
type RoleAccessReportStatus struct {
	// Snapshot describes when and against what the report was built.
	Snapshot ReportSnapshot `json:"snapshot" protobuf:"bytes,1,opt,name=snapshot"`

	// Roles holds one entry per reported role, ordered by name.
	// +optional
	// +listType=atomic
	Roles []RoleAccess `json:"roles,omitempty" protobuf:"bytes,2,rep,name=roles"`

	// Notes carries non-fatal remarks about how the report was built.
	// +optional
	// +listType=atomic
	Notes []string `json:"notes,omitempty" protobuf:"bytes,3,rep,name=notes"`

	// Truncated is true when output limits were hit.
	// +optional
	Truncated bool `json:"truncated,omitempty" protobuf:"varint,4,opt,name=truncated"`

	// Inventory lists every resource of the cluster, present when the request
	// asked for it. Roles say what is covered; this says what there is to cover.
	// +optional
	// +listType=atomic
	Inventory []InventoryResource `json:"inventory,omitempty" protobuf:"bytes,5,rep,name=inventory"`
}

// InventoryResource is one resource of the cluster as discovery describes it.
type InventoryResource struct {
	// Group is the API group; empty for the core group.
	// +optional
	Group string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`

	// Resource is the plural name, carrying the subresource when there is one:
	// "pods", "pods/exec".
	Resource string `json:"resource" protobuf:"bytes,2,opt,name=resource"`

	// Kind is the object kind, as discovery reports it.
	// +optional
	Kind string `json:"kind,omitempty" protobuf:"bytes,3,opt,name=kind"`

	// Namespaced is true when the resource lives inside a namespace.
	// +optional
	Namespaced bool `json:"namespaced,omitempty" protobuf:"varint,4,opt,name=namespaced"`

	// Module is the Deckhouse module that installs the resource, read from the
	// labels of its CustomResourceDefinition. The API group is not a substitute:
	// operator-trivy ships aquasecurity.github.io.
	// +optional
	Module string `json:"module,omitempty" protobuf:"bytes,6,opt,name=module"`

	// Custom is true for a CRD the platform does not install -- the resources of
	// the cluster owner. Built-in and aggregated APIs are not custom.
	// +optional
	Custom bool `json:"custom,omitempty" protobuf:"varint,7,opt,name=custom"`

	// System is true for a kind only the platform is meant to touch: the
	// internal storage of a controller, like the sessions and passwords of dex.
	// Nobody is supposed to be granted it, so a coverage review has nothing to
	// look for there.
	// +optional
	System bool `json:"system,omitempty" protobuf:"varint,8,opt,name=system"`

	// ClusterConfig is true for a kind that holds the configuration of the
	// cluster -- the objects a human writes. Read from the label the backup of
	// the cluster configuration is built from.
	// +optional
	ClusterConfig bool `json:"clusterConfig,omitempty" protobuf:"varint,9,opt,name=clusterConfig"`

	// Verbs are the ones the API server accepts for this resource. Coverage is
	// measured against them rather than against a fixed list of eight:
	// tokenreviews only ever accept create, and "1 of 8" would read as a gap in
	// the role model where there is none.
	// +optional
	// +listType=atomic
	Verbs []string `json:"verbs,omitempty" protobuf:"bytes,5,rep,name=verbs"`
}

// ReportSnapshot is what makes a report reproducible: the same cluster,
// unchanged, must produce the same document, and a reader must be able to tell
// what the document was built from.
type ReportSnapshot struct {
	// Time is when the report was built.
	Time metav1.Time `json:"time" protobuf:"bytes,1,opt,name=time"`

	// Model is the role model the report covers.
	Model string `json:"model" protobuf:"bytes,2,opt,name=model"`

	// ExpandedWildcards is true when wildcard rules were expanded.
	// +optional
	ExpandedWildcards bool `json:"expandedWildcards,omitempty" protobuf:"varint,3,opt,name=expandedWildcards"`

	// DiscoveryResources is the number of resources in the discovery snapshot
	// the wildcards were expanded against. Without it a wildcard row cannot be
	// interpreted: "every resource" means one thing on a cluster with
	// virtualization installed and another without it.
	// +optional
	DiscoveryResources int `json:"discoveryResources,omitempty" protobuf:"varint,4,opt,name=discoveryResources"`

	// Digest is a hash over the canonical form of the reported roles. Two
	// reports of an unchanged cluster carry the same digest, so a reader can
	// tell "nothing changed" from "I did not look".
	// +optional
	Digest string `json:"digest,omitempty" protobuf:"bytes,5,opt,name=digest"`
}

// RoleAccess is what one role grants.
type RoleAccess struct {
	// Name is the ClusterRole name, or the access level in the legacy model.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// Role carries the display metadata of the role.
	// +optional
	Role RoleDescriptor `json:"role,omitempty" protobuf:"bytes,2,opt,name=role"`

	// LegacyNames lists the names this role had in the previous model. There
	// can be more than one: the rename folded the kubernetes-suffixed variants
	// into a single role. The export carries them so a document can be compared
	// against one issued before the rename.
	// +optional
	// +listType=atomic
	LegacyNames []string `json:"legacyNames,omitempty" protobuf:"bytes,3,rep,name=legacyNames"`

	// Namespaced is true when the role only ever applies inside a namespace.
	// Its cluster-scoped rules, if any, are left out: they exist in RBAC but
	// can never be exercised, and an export that lists them overstates access.
	// +optional
	Namespaced bool `json:"namespaced,omitempty" protobuf:"varint,4,opt,name=namespaced"`

	// Composition lists the capabilities the role aggregates, and for the
	// legacy model the roles an access level binds. Filled when
	// spec.includeComposition is set.
	// +optional
	// +listType=atomic
	Composition []RoleComponent `json:"composition,omitempty" protobuf:"bytes,5,rep,name=composition"`

	// Resources lists the resource access the role grants.
	// +optional
	// +listType=atomic
	Resources []ResourceAccess `json:"resources,omitempty" protobuf:"bytes,6,rep,name=resources"`

	// NonResourceRules lists the non-resource URL access the role grants.
	// +optional
	// +listType=atomic
	NonResourceRules []NonResourceAccess `json:"nonResourceRules,omitempty" protobuf:"bytes,7,rep,name=nonResourceRules"`

	// Truncated is true when this role alone hit the output limits.
	// +optional
	Truncated bool `json:"truncated,omitempty" protobuf:"varint,8,opt,name=truncated"`

	// Notes carries remarks about this role alone.
	// +optional
	// +listType=atomic
	Notes []string `json:"notes,omitempty" protobuf:"bytes,9,rep,name=notes"`
}

// RoleComponent is one part a role is assembled from: a capability in the
// primary model, a bound ClusterRole in the legacy one.
type RoleComponent struct {
	// Name is the ClusterRole name of the component.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// Role carries the display metadata of the component.
	// +optional
	Role RoleDescriptor `json:"role,omitempty" protobuf:"bytes,2,opt,name=role"`
}

// AccessCaveat describes restrictions applied on top of RBAC by admission
// webhooks, which access reviews cannot see.
type AccessCaveat struct {
	// ProtectedVerbs is true when the scope grants verbs (update, patch,
	// delete, deletecollection) that the system-resource admission webhook
	// restricts for non-superadmin subjects.
	// +optional
	ProtectedVerbs bool `json:"protectedVerbs,omitempty" protobuf:"varint,1,opt,name=protectedVerbs"`

	// Superadmin is true when the subject holds a superadmin role covering the
	// whole scope and therefore bypasses the restriction.
	// +optional
	Superadmin bool `json:"superadmin,omitempty" protobuf:"varint,2,opt,name=superadmin"`

	// SuperadminNamespaces lists the namespaces of the scope where the subject
	// is superadmin.
	// +optional
	// +listType=atomic
	SuperadminNamespaces []string `json:"superadminNamespaces,omitempty" protobuf:"bytes,3,rep,name=superadminNamespaces"`

	// RestrictedNamespaces lists the namespaces of the scope where the webhook
	// restriction applies.
	// +optional
	// +listType=atomic
	RestrictedNamespaces []string `json:"restrictedNamespaces,omitempty" protobuf:"bytes,4,rep,name=restrictedNamespaces"`
}
