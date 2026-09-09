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

	"github.com/deckhouse/deckhouse/go_lib/user-authz/decision"
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
// One method, returning the type the decision consumes. The distinctions it has to draw - the
// snapshot lists the resource, the snapshot exists and does not, there is no snapshot, the group
// could not be read - belong next to the data that answers them. Split across three predicates they
// were assembled here instead, which meant every test fake had to reassemble them the same way, and
// a fake that did not left the suite green and the behaviour unsafe.
//
// Implemented by resolver.ResourceScopeCache. This package must not import
// resolver (resolver already imports multitenancy).
type ResourceScope interface {
	ScopeOf(group, resource string) rules.ResourceScope
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
//
// The sequence lives in go_lib/user-authz/decision, which the authorization webhook calls with the
// same inputs. What this apiserver reports and what the API server enforces are the same question,
// and they used to be two implementations of it that drifted: a resource that does not exist was
// reported as permitted here and forbidden there.
func (e *Engine) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	if !attrs.IsResourceRequest() {
		return authorizer.DecisionNoOpinion, "", nil
	}

	info := attrs.GetUser()
	if info == nil {
		return authorizer.DecisionNoOpinion, "", nil
	}

	// NOTE: a subject no rule names gets NoOpinion, not Deny. This method is part of the
	// authorizer chain for API requests, and denying here would stop users without a
	// ClusterAuthorizationRule from doing anything their RoleBindings allow. Deny-by-default
	// belongs to GetNamespaceAccessType, which filters the reported namespaces rather than
	// authorizing a request.
	result := decision.Authorize(decision.Request{
		User:      info.GetName(),
		Groups:    info.GetGroups(),
		Namespace: attrs.GetNamespace(),
		Resource:  attrs.GetResource(),
		APIGroup:  attrs.GetAPIGroup(),
	}, e.sources(ctx, attrs))

	if result.Denied() {
		return authorizer.DecisionDeny, result.Reason, nil
	}
	return authorizer.DecisionNoOpinion, "", nil
}

// sources are the facts the shared decision needs and this apiserver holds.
func (e *Engine) sources(ctx context.Context, attrs authorizer.Attributes) decision.Sources {
	src := decision.Sources{
		Directory:       e.rules.Directory(),
		Bindings:        e.bindings,
		NamespaceLabels: e.namespaceLabels(),
		ResourceScope:   e.resourceScopeOf,
		// V(2), not V(4): the only thing this logs is why a request was denied, and the webhook
		// logs the same line at default verbosity. A denial nobody can see the reason for is the
		// hardest kind of support case.
		Logf: klog.V(2).Infof,
		OnRestricted: func(username, rule string) {
			klog.V(2).Infof("user %q is bound by rule %q not observed binding it (rules synced: %v); restricting until the rule arrives",
				username, rule, e.rules.HasSynced())
		},
	}
	if e.independentRBAC != nil && attrs != nil {
		src.IndependentRBAC = func() bool { return e.independentRBAC.AllowsIndependently(ctx, attrs) }
	}
	return src
}

// affectedEntries are the directory entries that apply to a subject, with the ordering guard
// applied. It is the shared one; the method exists so callers inside this package read naturally.
func (e *Engine) affectedEntries(username string, groups []string) []rules.Entry {
	return decision.Entries(e.sources(context.Background(), nil), username, groups)
}

// resourceScopeOf reads the background snapshot. It never fails: an engine with no snapshot source
// answers "nothing is known", which the shared decision treats as a reason to deny.
func (e *Engine) resourceScopeOf(group, resource string) (rules.ResourceScope, error) {
	if e.resourceScope == nil {
		return rules.ResourceScope{}, nil
	}
	return e.resourceScope.ScopeOf(group, resource), nil
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
// The folding is the shared one; what stays here is this apiserver's own policy for a subject no
// rule names. An authorizer must let RBAC decide that case, but a report that filters a list has to
// choose, and it chooses deny-by-default unless the subject is in a privileged group.
func (e *Engine) GetNamespaceAccessType(userInfo user.Info) (NamespaceAccessType, *DirectoryEntry) {
	if userInfo == nil {
		return AllNamespacesAllowed, nil
	}

	access, entry := decision.NamespaceAccess(e.sources(context.Background(), nil),
		userInfo.GetName(), userInfo.GetGroups(), isPrivilegedUser(userInfo.GetGroups()))

	switch access {
	case decision.AllNamespaces:
		return AllNamespacesAllowed, nil
	case decision.NoNamespaces:
		klog.V(4).Infof("GetNamespaceAccessType: user=%s has no CAR and is not privileged (deny-by-default)", userInfo.GetName())
		return NoNamespacesAllowed, nil
	default:
		return FilteredAccess, &entry
	}
}

// IsNamespaceAllowedWithFilter checks if a namespace is allowed using a pre-computed filter.
// Use this with the filter returned by GetNamespaceAccessType to avoid redundant lookups.
func (e *Engine) IsNamespaceAllowedWithFilter(namespace string, filter *DirectoryEntry) bool {
	return decision.NamespaceAllowed(e.sources(context.Background(), nil), filter, namespace)
}
