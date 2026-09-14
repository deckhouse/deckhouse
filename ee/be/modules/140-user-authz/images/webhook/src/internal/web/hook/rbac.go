/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"fmt"
	"log"
	"sync"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"
	kcache "k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/binding"
)

// independentRBACResolver reports whether a request is allowed by RBAC grants
// that exist independently of ClusterAuthorizationRules.
type independentRBACResolver interface {
	AllowsIndependently(spec *WebhookResourceSpec) bool
}

// isCARManagedClusterRoleBinding reports whether the ClusterRoleBinding was
// generated from a ClusterAuthorizationRule by the user-authz module (by its
// Helm chart in earlier releases, by user-authz-controller now); the naming
// contract lives in go_lib/user-authz/binding.
//
// Such bindings are cluster-wide by construction, but their intended scope is
// the CAR's multi-tenancy options (limitNamespaces etc.), which is exactly
// what this webhook enforces. They must therefore be excluded when we check
// whether the user has access *independently* of any CAR - otherwise the
// CAR's accessLevel would leak into namespaces outside its limitNamespaces.
func isCARManagedClusterRoleBinding(crb *rbacv1.ClusterRoleBinding) bool {
	return binding.IsRuleBinding(crb.Name, crb.GetLabels())
}

// crbSubjectKey is a subject a ClusterRoleBinding names, in the form the request is matched
// against: a username (a User subject, or the canonical name of a ServiceAccount subject) or a
// group name.
type crbSubjectKey struct {
	isGroup bool
	name    string
}

// crbSubjectKeys returns the keys under which the binding has to be found. It mirrors
// subjectsMatch with an empty default namespace, which is what a ClusterRoleBinding gets.
func crbSubjectKeys(subjects []rbacv1.Subject) []crbSubjectKey {
	keys := make([]crbSubjectKey, 0, len(subjects))
	for _, subject := range subjects {
		switch subject.Kind {
		case rbacv1.UserKind:
			keys = append(keys, crbSubjectKey{name: subject.Name})
		case rbacv1.GroupKind:
			keys = append(keys, crbSubjectKey{isGroup: true, name: subject.Name})
		case rbacv1.ServiceAccountKind:
			// A ClusterRoleBinding has no namespace to default to, so the subject's own namespace
			// is the only one; an empty one yields a name no request can carry, as before.
			keys = append(keys, crbSubjectKey{name: fmt.Sprintf("system:serviceaccount:%s:%s", subject.Namespace, subject.Name)})
		}
	}
	return keys
}

// independentCRBIndex maps a subject to the CAR-independent ClusterRoleBindings that name it.
//
// Without it every request the multi-tenancy filters would deny costs a full scan of every
// ClusterRoleBinding in the cluster, and a cluster with a few thousand rules has tens of thousands
// of them. The index is maintained incrementally from the ClusterRoleBinding informer's events, so
// a change costs work proportional to the subjects of the one binding that changed and a lookup
// costs one map access per subject of the request. Nothing here needs debouncing: there is no
// rebuild to coalesce.
//
// CAR-generated bindings are left out entirely (see isCARManagedClusterRoleBinding): they are the
// bindings whose scope this webhook enforces, so they must never answer "granted independently".
type independentCRBIndex struct {
	mu sync.RWMutex
	// bySubject maps a subject to the bindings naming it, keyed by binding name so an update
	// replaces rather than duplicates.
	bySubject map[crbSubjectKey]map[string]*rbacv1.ClusterRoleBinding
	// keysOf records what each binding contributed, so an update or a delete can withdraw exactly
	// the previous contribution.
	keysOf map[string][]crbSubjectKey
}

func newIndependentCRBIndex() *independentCRBIndex {
	return &independentCRBIndex{
		bySubject: make(map[crbSubjectKey]map[string]*rbacv1.ClusterRoleBinding),
		keysOf:    make(map[string][]crbSubjectKey),
	}
}

// upsert indexes the binding, replacing whatever it contributed before. A CAR-generated binding is
// withdrawn instead: a binding that gains the module's labels stops being an independent grant.
func (i *independentCRBIndex) upsert(crb *rbacv1.ClusterRoleBinding) {
	if crb == nil {
		return
	}
	if isCARManagedClusterRoleBinding(crb) {
		i.delete(crb)
		return
	}

	keys := crbSubjectKeys(crb.Subjects)

	i.mu.Lock()
	defer i.mu.Unlock()

	i.withdrawLocked(crb.Name)
	if len(keys) == 0 {
		return
	}
	for _, key := range keys {
		bindings := i.bySubject[key]
		if bindings == nil {
			bindings = make(map[string]*rbacv1.ClusterRoleBinding, 1)
			i.bySubject[key] = bindings
		}
		bindings[crb.Name] = crb
	}
	i.keysOf[crb.Name] = keys
}

// delete withdraws the binding from the index.
func (i *independentCRBIndex) delete(crb *rbacv1.ClusterRoleBinding) {
	if crb == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.withdrawLocked(crb.Name)
}

// withdrawLocked removes every contribution of the binding. The caller holds the write lock.
func (i *independentCRBIndex) withdrawLocked(name string) {
	for _, key := range i.keysOf[name] {
		bindings := i.bySubject[key]
		if bindings == nil {
			continue
		}
		delete(bindings, name)
		if len(bindings) == 0 {
			delete(i.bySubject, key)
		}
	}
	delete(i.keysOf, name)
}

// forRequest returns the CAR-independent ClusterRoleBindings that name the user or any of their
// groups, each at most once.
func (i *independentCRBIndex) forRequest(username string, groups []string) []*rbacv1.ClusterRoleBinding {
	i.mu.RLock()
	defer i.mu.RUnlock()

	var (
		out  []*rbacv1.ClusterRoleBinding
		seen map[string]struct{}
	)
	collect := func(key crbSubjectKey) {
		for name, crb := range i.bySubject[key] {
			if seen == nil {
				seen = make(map[string]struct{})
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, crb)
		}
	}

	collect(crbSubjectKey{name: username})
	for _, group := range groups {
		collect(crbSubjectKey{isGroup: true, name: group})
	}

	return out
}

// len reports how many bindings the index holds; it exists for the tests.
func (i *independentCRBIndex) len() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.keysOf)
}

// eventHandler keeps the index in step with the ClusterRoleBinding informer.
func (i *independentCRBIndex) eventHandler() kcache.ResourceEventHandler {
	return kcache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if crb, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
				i.upsert(crb)
			}
		},
		UpdateFunc: func(_, newObj interface{}) {
			if crb, ok := newObj.(*rbacv1.ClusterRoleBinding); ok {
				i.upsert(crb)
			}
		},
		DeleteFunc: func(obj interface{}) {
			crb, ok := obj.(*rbacv1.ClusterRoleBinding)
			if !ok {
				// The informer reports a delete it could not observe directly as a tombstone.
				tombstone, isTombstone := obj.(kcache.DeletedFinalStateUnknown)
				if !isTombstone {
					return
				}
				if crb, ok = tombstone.Obj.(*rbacv1.ClusterRoleBinding); !ok {
					return
				}
			}
			i.delete(crb)
		},
	}
}

// RBACEvaluator checks requests against RBAC objects from informer caches.
//
// It is intentionally conservative: it implements the exact upstream RBAC
// matching semantics and treats any uncertainty (missing role, unsynced
// cache, lister error) as "not granted". A false negative here only keeps the
// multi-tenancy deny in place (no worse than without the evaluator), while a
// false positive would let the request through to the real RBAC authorizer -
// which may then allow it via a CAR-generated cluster-wide binding the user
// was not supposed to use outside the CAR scope.
type RBACEvaluator struct {
	logger *log.Logger

	roleLister        rbaclisters.RoleLister
	roleBindingLister rbaclisters.RoleBindingLister
	clusterRoleLister rbaclisters.ClusterRoleLister

	// clusterRoleBindings indexes the CAR-independent ClusterRoleBindings by subject, so a request
	// does not pay for a scan of every binding in the cluster.
	clusterRoleBindings *independentCRBIndex

	synced []kcache.InformerSynced
}

// NewRBACEvaluator registers RBAC informers in the factory and returns an
// evaluator backed by their listers. The caller is responsible for starting
// the factory and waiting for cache sync.
//
// The ClusterRoleBinding index is fed by an event handler, which has to be registered before the
// factory starts; a failure to register it is fatal, because an index that never fills would report
// every request as not granted independently and the webhook would deny access that
// RoleBindings and non-CAR ClusterRoleBindings do grant.
func NewRBACEvaluator(logger *log.Logger, informerFactory informers.SharedInformerFactory) (*RBACEvaluator, error) {
	rbacInformers := informerFactory.Rbac().V1()

	roles := rbacInformers.Roles()
	roleBindings := rbacInformers.RoleBindings()
	clusterRoles := rbacInformers.ClusterRoles()
	clusterRoleBindings := rbacInformers.ClusterRoleBindings()

	index := newIndependentCRBIndex()
	indexSynced, err := clusterRoleBindings.Informer().AddEventHandler(index.eventHandler())
	if err != nil {
		return nil, fmt.Errorf("register the independent ClusterRoleBinding index: %w", err)
	}

	return &RBACEvaluator{
		logger:              logger,
		roleLister:          roles.Lister(),
		roleBindingLister:   roleBindings.Lister(),
		clusterRoleLister:   clusterRoles.Lister(),
		clusterRoleBindings: index,
		synced: []kcache.InformerSynced{
			roles.Informer().HasSynced,
			roleBindings.Informer().HasSynced,
			clusterRoles.Informer().HasSynced,
			clusterRoleBindings.Informer().HasSynced,
			// The informer reports synced once the initial list has been popped; the handler that
			// fills the index is fed from a separate queue. Answering from a half-filled index
			// would deny requests that a non-CAR ClusterRoleBinding grants.
			indexSynced.HasSynced,
		},
	}, nil
}

// Synced reports whether all RBAC informer caches have synced.
func (e *RBACEvaluator) Synced() []kcache.InformerSynced {
	return e.synced
}

func (e *RBACEvaluator) cachesSynced() bool {
	for _, synced := range e.synced {
		if !synced() {
			return false
		}
	}
	return true
}

// AllowsIndependently reports whether the request is allowed by RBAC grants
// that exist independently of any ClusterAuthorizationRule:
//
//   - RoleBindings in the request's namespace: namespace-scoped grants can
//     never escalate beyond their namespace, so all of them count (including
//     RoleBindings rendered from AuthorizationRules);
//   - ClusterRoleBindings that were NOT generated from a CAR: deliberate
//     cluster-wide grants given outside the user-authz accessLevel machinery.
func (e *RBACEvaluator) AllowsIndependently(spec *WebhookResourceSpec) bool {
	if !e.cachesSynced() {
		e.logger.Println("independent RBAC check skipped: informer caches are not synced yet")
		return false
	}

	if e.clusterRoleBindingsAllow(spec) {
		return true
	}

	if spec.ResourceAttributes.Namespace != "" && e.roleBindingsAllow(spec) {
		return true
	}

	return false
}

func (e *RBACEvaluator) clusterRoleBindingsAllow(spec *WebhookResourceSpec) bool {
	// Only the bindings that name the request's user or one of its groups can grant anything, and
	// the index already excludes the CAR-generated ones.
	for _, binding := range e.clusterRoleBindings.forRequest(spec.User, spec.Group) {
		role, err := e.clusterRoleLister.Get(binding.RoleRef.Name)
		if err != nil {
			continue
		}

		if rulesAllow(role.Rules, &spec.ResourceAttributes) {
			return true
		}
	}

	return false
}

func (e *RBACEvaluator) roleBindingsAllow(spec *WebhookResourceSpec) bool {
	namespace := spec.ResourceAttributes.Namespace
	bindings, err := e.roleBindingLister.RoleBindings(namespace).List(labels.Everything())
	if err != nil {
		e.logger.Printf("independent RBAC check: failed to list RoleBindings in %s: %v", namespace, err)
		return false
	}

	for _, binding := range bindings {
		if !subjectsMatch(binding.Subjects, spec, namespace) {
			continue
		}

		var rules []rbacv1.PolicyRule
		if binding.RoleRef.Kind == "ClusterRole" {
			role, err := e.clusterRoleLister.Get(binding.RoleRef.Name)
			if err != nil {
				continue
			}
			rules = role.Rules
		} else {
			role, err := e.roleLister.Roles(namespace).Get(binding.RoleRef.Name)
			if err != nil {
				continue
			}
			rules = role.Rules
		}

		if rulesAllow(rules, &spec.ResourceAttributes) {
			return true
		}
	}

	return false
}

// subjectsMatch checks whether any binding subject matches the request's user.
// defaultNamespace fills in an empty ServiceAccount subject namespace (RBAC
// defaults it to the RoleBinding's namespace).
func subjectsMatch(subjects []rbacv1.Subject, spec *WebhookResourceSpec, defaultNamespace string) bool {
	for _, subject := range subjects {
		switch subject.Kind {
		case rbacv1.UserKind:
			if subject.Name == spec.User {
				return true
			}
		case rbacv1.GroupKind:
			for _, group := range spec.Group {
				if subject.Name == group {
					return true
				}
			}
		case rbacv1.ServiceAccountKind:
			saNamespace := subject.Namespace
			if saNamespace == "" {
				saNamespace = defaultNamespace
			}
			if fmt.Sprintf("system:serviceaccount:%s:%s", saNamespace, subject.Name) == spec.User {
				return true
			}
		}
	}
	return false
}

func rulesAllow(rules []rbacv1.PolicyRule, attrs *WebhookResourceAttributes) bool {
	for i := range rules {
		if ruleAllows(&rules[i], attrs) {
			return true
		}
	}
	return false
}

// ruleAllows mirrors upstream RBAC evaluation semantics
// (k8s.io/kubernetes/pkg/apis/rbac/v1 evaluation helpers) for resource
// requests. Being stricter than upstream is acceptable (the multi-tenancy
// deny stays in place), being looser is not.
func ruleAllows(rule *rbacv1.PolicyRule, attrs *WebhookResourceAttributes) bool {
	return verbMatches(rule.Verbs, attrs.Verb) &&
		apiGroupMatches(rule.APIGroups, attrs.Group) &&
		resourceMatches(rule.Resources, attrs.Resource, attrs.Subresource) &&
		resourceNameMatches(rule.ResourceNames, attrs.Name)
}

func verbMatches(ruleVerbs []string, verb string) bool {
	for _, ruleVerb := range ruleVerbs {
		if ruleVerb == rbacv1.VerbAll || ruleVerb == verb {
			return true
		}
	}
	return false
}

func apiGroupMatches(ruleGroups []string, group string) bool {
	for _, ruleGroup := range ruleGroups {
		if ruleGroup == rbacv1.APIGroupAll || ruleGroup == group {
			return true
		}
	}
	return false
}

// resourceMatches follows upstream RBAC: a rule resource matches when it is
// "*", the exact "resource/subresource" combination, or "*/subresource".
// Notably a rule for the bare resource does NOT grant its subresources, and
// "resource/*" is not a wildcard.
func resourceMatches(ruleResources []string, resource, subresource string) bool {
	combined := resource
	if subresource != "" {
		combined = resource + "/" + subresource
	}

	for _, ruleResource := range ruleResources {
		if ruleResource == rbacv1.ResourceAll || ruleResource == combined {
			return true
		}
		if subresource != "" && ruleResource == "*/"+subresource {
			return true
		}
	}
	return false
}

// resourceNameMatches follows upstream RBAC: a rule with resourceNames
// requires an exact name match, so it never matches requests without a name
// (e.g. list/watch/create).
func resourceNameMatches(ruleNames []string, name string) bool {
	if len(ruleNames) == 0 {
		return true
	}
	for _, ruleName := range ruleNames {
		if ruleName == name {
			return true
		}
	}
	return false
}
