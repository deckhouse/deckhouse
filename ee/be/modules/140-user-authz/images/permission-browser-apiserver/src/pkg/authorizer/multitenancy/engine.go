/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"
)

const (
	// The reasons are shared with the user-authz webhook through the library: both answer the same
	// question, and the wording must not let a caller tell a closed namespace from a missing one.
	noNamespaceAccessReason      = rules.NoNamespaceAccessReason
	namespaceLimitedAccessReason = rules.NamespaceLimitedAccessReason
)

// privilegedGroups contains groups that bypass multi-tenancy restrictions.
// Users in these groups are allowed full access even without ClusterAuthorizationRules.
var privilegedGroups = map[string]struct{}{
	"system:masters":         {},
	"kubeadm:cluster-admins": {},
	"superadmins":            {},
}

// isPrivilegedUser checks if the user belongs to any privileged group
// that should bypass multi-tenancy restrictions.
func isPrivilegedUser(groups []string) bool {
	for _, group := range groups {
		if _, ok := privilegedGroups[group]; ok {
			return true
		}
	}
	return false
}

// IndependentRBACChecker reports whether a request is allowed by RBAC grants
// that exist independently of ClusterAuthorizationRules: RoleBindings in the
// request's namespace and ClusterRoleBindings not generated from a CAR.
type IndependentRBACChecker interface {
	AllowsIndependently(ctx context.Context, attrs authorizer.Attributes) bool
}

// ResourceScope reports whether a resource is namespaced. known is false when
// the snapshot has never seen the group/resource (missing CRD, broken
// APIService, empty cache). Callers that enforce namespace limits treat
// !known like namespaced: failing open would let a CAR ClusterRoleBinding
// report Allow for a cluster-scoped list of a namespaced resource.
//
// HasData separates "the snapshot exists and does not list this resource"
// from "there is no snapshot at all". The webhook makes the same distinction,
// and only the first case, for the core group, lets RBAC answer.
//
// Implemented by resolver.ResourceScopeCache. This package must not import
// resolver (resolver already imports multitenancy).
type ResourceScope interface {
	Scope(group, resource string) (bool, bool)
	HasData() bool
}

// RulesProvider hands out the current directory of ClusterAuthorizationRules. Directory is nil
// until the rules have been listed once; the engine treats that as "the rules are not known",
// never as "there are no rules".
type RulesProvider interface {
	Directory() *rules.Directory
	HasSynced() bool
}

// RuleBindings answers which ClusterAuthorizationRules bind a user through the ClusterRoleBindings
// user-authz-controller created for them. It is the evidence the engine uses to notice that its
// directory lags behind the controller.
type RuleBindings interface {
	RulesFor(username string, groups []string) []string
}

// Engine implements the multi-tenancy authorization logic of the user-authz webhook over the
// same library, so what this apiserver reports is what the API server enforces.
type Engine struct {
	rules    RulesProvider
	bindings RuleBindings

	nsLister      corev1listers.NamespaceLister
	nsSynced      cache.InformerSynced
	resourceScope ResourceScope

	// independentRBAC, when set, is consulted before returning Deny: requests
	// explicitly granted by CAR-independent RBAC must not be denied by
	// multi-tenancy filters.
	independentRBAC IndependentRBACChecker
}

// SetIndependentRBACChecker wires the CAR-independent RBAC checker into the
// engine. Must be called before the engine starts serving Authorize calls.
func (e *Engine) SetIndependentRBACChecker(checker IndependentRBACChecker) {
	e.independentRBAC = checker
}

// NewEngine creates a new multi-tenancy engine. rulesProvider and bindings are required: without
// the rules the engine has no opinion about anybody, and without the bindings it cannot tell a
// subject nobody limits from a subject whose rule it has not observed yet. resourceScope may be
// nil: every lookup then reports !known, which is fail-closed for filtered cluster-scoped requests.
// Live discovery is never used on Authorize.
func NewEngine(rulesProvider RulesProvider, bindings RuleBindings, nsLister corev1listers.NamespaceLister, nsSynced cache.InformerSynced, resourceScope ResourceScope) (*Engine, error) {
	if rulesProvider == nil {
		return nil, fmt.Errorf("rules provider is required")
	}
	if bindings == nil {
		return nil, fmt.Errorf("rule bindings index is required")
	}
	return &Engine{
		rules:         rulesProvider,
		bindings:      bindings,
		nsLister:      nsLister,
		nsSynced:      nsSynced,
		resourceScope: resourceScope,
	}, nil
}

// Authorize implements authorizer.Authorizer
// This authorizer only denies; it never allows (returns NoOpinion if access is not restricted)
func (e *Engine) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	if !attrs.IsResourceRequest() {
		return authorizer.DecisionNoOpinion, "", nil
	}

	user := attrs.GetUser()
	if user == nil {
		return authorizer.DecisionNoOpinion, "", nil
	}

	entries := e.affectedEntries(user.GetName(), user.GetGroups())
	if len(entries) == 0 {
		// NOTE: We intentionally return NoOpinion here (not Deny) even for users without CAR.
		// This method is part of the Kubernetes authorizer chain for API requests.
		// NoOpinion means "I have no opinion, let RBAC decide".
		// If we returned Deny here, users without CAR couldn't do ANYTHING,
		// even with valid RBAC permissions (RoleBindings).
		//
		// The deny-by-default logic is applied only in GetNamespaceAccessType /
		// IsNamespaceAllowedWithFilter, which are used for filtering the
		// accessiblenamespaces API response (data filtering), not for API
		// request authorization.
		return authorizer.DecisionNoOpinion, "", nil
	}

	combined := rules.Combine(entries)

	// Check namespaced request
	if attrs.GetNamespace() != "" {
		return e.authorizeNamespacedRequest(ctx, attrs, &combined)
	}

	// Check cluster-scoped request for namespaced resource
	if attrs.GetResource() != "" {
		return e.authorizeClusterScopedRequest(ctx, attrs, &combined)
	}

	return authorizer.DecisionNoOpinion, "", nil
}

// authorizeNamespacedRequest checks if the user can access the specific namespace.
//
// The multi-tenancy scope here is CAR-only (LimitNamespaces, namespaceSelectors,
// the system-namespace gate): inside that scope the CAR's cluster-wide
// accessLevel binding is meant to apply, so we return NoOpinion and let RBAC
// decide. Outside that scope the request is denied unless CAR-independent RBAC
// explicitly grants it - this both keeps RoleBinding/AuthorizationRule access
// working and prevents the CAR accessLevel from leaking into namespaces not
// listed in limitNamespaces.
func (e *Engine) authorizeNamespacedRequest(ctx context.Context, attrs authorizer.Attributes, entry *rules.Entry) (authorizer.Decision, string, error) {
	if !entry.HasAnyFilters() {
		return authorizer.DecisionNoOpinion, "", nil
	}

	allowed, err := rules.NamespaceAllowed(entry, attrs.GetNamespace(), e.namespaceLabels())
	if err != nil {
		// A namespace that does not exist fails the lookup as well as a cache problem. Neither
		// reaches the caller (see noNamespaceAccessReason); the operator gets the detail here.
		klog.V(4).Infof("namespace selector check for %q failed: %v", attrs.GetNamespace(), err)
	}
	if allowed {
		return authorizer.DecisionNoOpinion, "", nil
	}

	// The namespace is outside the CAR scope. Requests granted by
	// CAR-independent RBAC (RoleBindings in the namespace, non-CAR
	// ClusterRoleBindings) must not be denied.
	if e.independentRBAC != nil && e.independentRBAC.AllowsIndependently(ctx, attrs) {
		return authorizer.DecisionNoOpinion, "", nil
	}

	return authorizer.DecisionDeny, noNamespaceAccessReason, nil
}

// authorizeClusterScopedRequest checks if cluster-scoped requests for namespaced resources should be denied
func (e *Engine) authorizeClusterScopedRequest(ctx context.Context, attrs authorizer.Attributes, entry *rules.Entry) (authorizer.Decision, string, error) {
	if !entry.HasAnyFilters() {
		return authorizer.DecisionNoOpinion, "", nil
	}

	// Known cluster-scoped resources are not limited by namespace filters.
	// Everything else (namespaced, or unknown) is treated as namespaced so a
	// discovery miss cannot fail-open through a CAR ClusterRoleBinding. The one
	// exception is a core resource absent from a populated snapshot: it does not
	// exist, and the webhook lets RBAC answer that case instead of denying it,
	// so BulkSAR must not report Deny where the webhook would not.
	scope := rules.ResourceScope{Core: attrs.GetAPIGroup() == ""}
	if e.resourceScope != nil {
		scope.Namespaced, scope.Known = e.resourceScope.Scope(attrs.GetAPIGroup(), attrs.GetResource())
		scope.CoreGroupPopulated = e.resourceScope.HasData()
	}
	if !rules.ClusterScopedDenied(scope) {
		return authorizer.DecisionNoOpinion, "", nil
	}

	if e.independentRBAC != nil && e.independentRBAC.AllowsIndependently(ctx, attrs) {
		return authorizer.DecisionNoOpinion, "", nil
	}
	return authorizer.DecisionDeny, namespaceLimitedAccessReason, nil
}

// affectedEntries collects the directory entries of the User/ServiceAccount/Groups.
//
// It also applies the ordering guard: user-authz-controller creates the ClusterRoleBindings of a
// rule seconds after the rule, and the rule and the bindings reach this apiserver over independent
// watches. A subject bound by a rule binding whose rule is not in the directory (the directory
// lags, or has not been listed yet) is treated as maximally restricted, so the report never shows
// more than the webhook will allow. The restriction lifts by itself when the rule arrives.
func (e *Engine) affectedEntries(username string, groups []string) []rules.Entry {
	dir := e.rules.Directory()
	entries := dir.Lookup(username, groups)

	for _, rule := range e.bindings.RulesFor(username, groups) {
		if dir.KnowsRule(rule) {
			continue
		}
		klog.V(2).Infof("user %q is bound by rule %q not observed yet (rules synced: %v); restricting until it arrives", username, rule, e.rules.HasSynced())
		entries = append(entries, rules.Restricted())
		break
	}

	return entries
}

// namespaceLabels returns the label lookup for the namespaceSelector check, or nil when the engine
// has no namespace cache: selectors then match nothing, as before.
func (e *Engine) namespaceLabels() rules.NamespaceLabels {
	if e.nsLister == nil {
		return nil
	}
	return func(namespace string) (labels.Set, error) {
		if e.nsSynced != nil && !e.nsSynced() {
			return nil, fmt.Errorf("namespace cache is not synced yet")
		}
		ns, err := e.nsLister.Get(namespace)
		if err != nil {
			return nil, err
		}
		set := labels.Set(ns.GetLabels())
		if set == nil {
			set = labels.Set{}
		}
		return set, nil
	}
}

// GetNamespaceAccessType evaluates the user's namespace access and returns:
//   - accessType: AllNamespacesAllowed, NoNamespacesAllowed, or FilteredAccess
//   - filter: the combined directory entry for filtering (only valid when accessType == FilteredAccess)
//
// This method computes the affected entries once and returns the filter for reuse,
// avoiding redundant lookups when filtering multiple namespaces.
func (e *Engine) GetNamespaceAccessType(userInfo user.Info) (NamespaceAccessType, *DirectoryEntry) {
	if userInfo == nil {
		return AllNamespacesAllowed, nil
	}

	entries := e.affectedEntries(userInfo.GetName(), userInfo.GetGroups())
	if len(entries) == 0 {
		// No ClusterAuthorizationRules apply to this user.
		// Privileged users (system:masters, etc.) bypass MT restrictions.
		if isPrivilegedUser(userInfo.GetGroups()) {
			klog.V(4).Infof("GetNamespaceAccessType: user=%s is privileged, all namespaces allowed", userInfo.GetName())
			return AllNamespacesAllowed, nil
		}
		// Non-privileged users without CAR get no access (deny-by-default).
		klog.V(4).Infof("GetNamespaceAccessType: user=%s has no CAR and is not privileged (deny-by-default)", userInfo.GetName())
		return NoNamespacesAllowed, nil
	}

	combined := rules.Combine(entries)
	if !combined.HasAnyFilters() {
		return AllNamespacesAllowed, nil
	}

	// User has restrictions - caller must filter each namespace
	return FilteredAccess, &combined
}

// IsNamespaceAllowedWithFilter checks if a namespace is allowed using a pre-computed filter.
// Use this with the filter returned by GetNamespaceAccessType to avoid redundant lookups.
func (e *Engine) IsNamespaceAllowedWithFilter(namespace string, filter *DirectoryEntry) bool {
	if filter == nil {
		return true
	}
	allowed, _ := rules.NamespaceAllowed(filter, namespace, e.namespaceLabels())
	return allowed
}
