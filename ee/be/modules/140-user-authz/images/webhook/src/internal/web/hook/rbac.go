/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"fmt"
	"log"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"
	kcache "k8s.io/client-go/tools/cache"
)

// independentRBACResolver reports whether a request is allowed by RBAC grants
// that exist independently of ClusterAuthorizationRules.
type independentRBACResolver interface {
	AllowsIndependently(spec *WebhookResourceSpec) bool
}

// carManagedCRBPrefix is the name prefix of ClusterRoleBindings rendered from
// ClusterAuthorizationRules by the user-authz module (see
// modules/140-user-authz/templates/cluster-role-bindings.yaml:
// "user-authz:<car-name>:<postfix>").
const carManagedCRBPrefix = "user-authz:"

// isCARManagedClusterRoleBinding reports whether the ClusterRoleBinding was
// generated from a ClusterAuthorizationRule by the user-authz module.
//
// Such bindings are cluster-wide by construction, but their intended scope is
// the CAR's multi-tenancy options (limitNamespaces etc.), which is exactly
// what this webhook enforces. They must therefore be excluded when we check
// whether the user has access *independently* of any CAR - otherwise the
// CAR's accessLevel would leak into namespaces outside its limitNamespaces.
func isCARManagedClusterRoleBinding(binding *rbacv1.ClusterRoleBinding) bool {
	if !strings.HasPrefix(binding.Name, carManagedCRBPrefix) {
		return false
	}
	bindingLabels := binding.GetLabels()
	return bindingLabels["heritage"] == "deckhouse" && bindingLabels["module"] == "user-authz"
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

	roleLister               rbaclisters.RoleLister
	roleBindingLister        rbaclisters.RoleBindingLister
	clusterRoleLister        rbaclisters.ClusterRoleLister
	clusterRoleBindingLister rbaclisters.ClusterRoleBindingLister

	synced []kcache.InformerSynced
}

// NewRBACEvaluator registers RBAC informers in the factory and returns an
// evaluator backed by their listers. The caller is responsible for starting
// the factory and waiting for cache sync.
func NewRBACEvaluator(logger *log.Logger, informerFactory informers.SharedInformerFactory) *RBACEvaluator {
	rbacInformers := informerFactory.Rbac().V1()

	roles := rbacInformers.Roles()
	roleBindings := rbacInformers.RoleBindings()
	clusterRoles := rbacInformers.ClusterRoles()
	clusterRoleBindings := rbacInformers.ClusterRoleBindings()

	return &RBACEvaluator{
		logger:                   logger,
		roleLister:               roles.Lister(),
		roleBindingLister:        roleBindings.Lister(),
		clusterRoleLister:        clusterRoles.Lister(),
		clusterRoleBindingLister: clusterRoleBindings.Lister(),
		synced: []kcache.InformerSynced{
			roles.Informer().HasSynced,
			roleBindings.Informer().HasSynced,
			clusterRoles.Informer().HasSynced,
			clusterRoleBindings.Informer().HasSynced,
		},
	}
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
	bindings, err := e.clusterRoleBindingLister.List(labels.Everything())
	if err != nil {
		e.logger.Printf("independent RBAC check: failed to list ClusterRoleBindings: %v", err)
		return false
	}

	for _, binding := range bindings {
		if isCARManagedClusterRoleBinding(binding) {
			continue
		}
		if !subjectsMatch(binding.Subjects, spec, "") {
			continue
		}

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
