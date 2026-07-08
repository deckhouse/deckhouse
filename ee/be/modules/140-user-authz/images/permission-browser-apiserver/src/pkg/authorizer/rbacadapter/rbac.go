/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package rbacadapter

import (
	"context"
	"fmt"
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/informers"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/klog/v2"
)

// RBACAuthorizer implements RBAC authorization using informers
type RBACAuthorizer struct {
	roleLister               rbaclisters.RoleLister
	roleBindingLister        rbaclisters.RoleBindingLister
	clusterRoleLister        rbaclisters.ClusterRoleLister
	clusterRoleBindingLister rbaclisters.ClusterRoleBindingLister
}

// carManagedCRBPrefix is the name prefix of ClusterRoleBindings rendered from
// ClusterAuthorizationRules by the user-authz module (see
// modules/140-user-authz/templates/cluster-role-bindings.yaml:
// "user-authz:<car-name>:<postfix>").
const carManagedCRBPrefix = "user-authz:"

// IsCARManagedClusterRoleBinding reports whether the ClusterRoleBinding was
// generated from a ClusterAuthorizationRule by the user-authz module.
//
// Such bindings are cluster-wide by construction, but their intended scope is
// limited by the CAR's multi-tenancy options (limitNamespaces etc.). They must
// therefore be excluded when we check whether the user has access to a
// namespace *independently* of any CAR.
func IsCARManagedClusterRoleBinding(binding *rbacv1.ClusterRoleBinding) bool {
	if !strings.HasPrefix(binding.Name, carManagedCRBPrefix) {
		return false
	}
	labels := binding.GetLabels()
	return labels["heritage"] == "deckhouse" && labels["module"] == "user-authz"
}

// NewRBACAuthorizer creates a new RBAC authorizer from informers
func NewRBACAuthorizer(informerFactory informers.SharedInformerFactory) *RBACAuthorizer {
	rbacInformers := informerFactory.Rbac().V1()

	return &RBACAuthorizer{
		roleLister:               rbacInformers.Roles().Lister(),
		roleBindingLister:        rbacInformers.RoleBindings().Lister(),
		clusterRoleLister:        rbacInformers.ClusterRoles().Lister(),
		clusterRoleBindingLister: rbacInformers.ClusterRoleBindings().Lister(),
	}
}

// Authorize implements authorizer.Authorizer
func (r *RBACAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	user := attrs.GetUser()
	if user == nil {
		return authorizer.DecisionNoOpinion, "", nil
	}

	userName := user.GetName()
	userGroups := user.GetGroups()

	// Check ClusterRoleBindings for cluster-scoped or namespaced requests
	if allowed, reason := r.checkClusterRoleBindings(attrs, userName, userGroups); allowed {
		klog.V(5).Infof("RBAC: allowed by ClusterRoleBinding: %s", reason)
		return authorizer.DecisionAllow, reason, nil
	}

	// Check RoleBindings for namespaced requests
	if attrs.GetNamespace() != "" {
		if allowed, reason := r.checkRoleBindings(attrs, userName, userGroups); allowed {
			klog.V(5).Infof("RBAC: allowed by RoleBinding: %s", reason)
			return authorizer.DecisionAllow, reason, nil
		}
	}

	return authorizer.DecisionNoOpinion, "", nil
}

// AllowsIndependently reports whether the request is allowed by RBAC grants
// that exist independently of any ClusterAuthorizationRule: RoleBindings in
// the request's namespace (namespace-scoped grants can never escalate beyond
// their namespace) and ClusterRoleBindings that were NOT generated from a CAR.
//
// It is used by the multi-tenancy engine to avoid denying requests that plain
// RBAC explicitly grants, while still denying access that would only be
// possible through a CAR-generated cluster-wide binding outside the CAR's
// limitNamespaces scope.
func (r *RBACAuthorizer) AllowsIndependently(_ context.Context, attrs authorizer.Attributes) bool {
	user := attrs.GetUser()
	if user == nil {
		return false
	}

	userName := user.GetName()
	userGroups := user.GetGroups()

	if allowed, _ := r.checkClusterRoleBindingsFiltered(attrs, userName, userGroups, true); allowed {
		return true
	}

	if attrs.GetNamespace() != "" {
		if allowed, _ := r.checkRoleBindings(attrs, userName, userGroups); allowed {
			return true
		}
	}

	return false
}

// checkClusterRoleBindings checks if the user has access via ClusterRoleBindings
func (r *RBACAuthorizer) checkClusterRoleBindings(attrs authorizer.Attributes, userName string, userGroups []string) (bool, string) {
	return r.checkClusterRoleBindingsFiltered(attrs, userName, userGroups, false)
}

// checkClusterRoleBindingsFiltered checks ClusterRoleBindings, optionally
// skipping CAR-generated bindings (see IsCARManagedClusterRoleBinding).
func (r *RBACAuthorizer) checkClusterRoleBindingsFiltered(attrs authorizer.Attributes, userName string, userGroups []string, skipCARManaged bool) (bool, string) {
	bindings, err := r.clusterRoleBindingLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list ClusterRoleBindings: %v", err)
		return false, ""
	}

	for _, binding := range bindings {
		if skipCARManaged && IsCARManagedClusterRoleBinding(binding) {
			continue
		}
		if !r.subjectMatches(binding.Subjects, userName, userGroups, "") {
			continue
		}

		// Get the ClusterRole
		role, err := r.clusterRoleLister.Get(binding.RoleRef.Name)
		if err != nil {
			klog.V(5).Infof("Failed to get ClusterRole %s: %v", binding.RoleRef.Name, err)
			continue
		}

		if r.ruleAllows(role.Rules, attrs) {
			return true, fmt.Sprintf("RBAC: allowed by ClusterRoleBinding %q of ClusterRole %q to user %q",
				binding.Name, role.Name, userName)
		}
	}

	return false, ""
}

// checkRoleBindings checks if the user has access via RoleBindings in the namespace
func (r *RBACAuthorizer) checkRoleBindings(attrs authorizer.Attributes, userName string, userGroups []string) (bool, string) {
	namespace := attrs.GetNamespace()
	bindings, err := r.roleBindingLister.RoleBindings(namespace).List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list RoleBindings in namespace %s: %v", namespace, err)
		return false, ""
	}

	for _, binding := range bindings {
		if !r.subjectMatches(binding.Subjects, userName, userGroups, namespace) {
			continue
		}

		var rules []rbacv1.PolicyRule
		if binding.RoleRef.Kind == "ClusterRole" {
			role, err := r.clusterRoleLister.Get(binding.RoleRef.Name)
			if err != nil {
				klog.V(5).Infof("Failed to get ClusterRole %s: %v", binding.RoleRef.Name, err)
				continue
			}
			rules = role.Rules
		} else {
			role, err := r.roleLister.Roles(namespace).Get(binding.RoleRef.Name)
			if err != nil {
				klog.V(5).Infof("Failed to get Role %s/%s: %v", namespace, binding.RoleRef.Name, err)
				continue
			}
			rules = role.Rules
		}

		if r.ruleAllows(rules, attrs) {
			return true, fmt.Sprintf("RBAC: allowed by RoleBinding %q of %s %q to user %q",
				binding.Name, binding.RoleRef.Kind, binding.RoleRef.Name, userName)
		}
	}

	return false, ""
}

// subjectMatches checks if any subject in the list matches the user
func (r *RBACAuthorizer) subjectMatches(subjects []rbacv1.Subject, userName string, userGroups []string, _ string) bool {
	for _, subject := range subjects {
		switch subject.Kind {
		case rbacv1.UserKind:
			if subject.Name == userName {
				return true
			}
		case rbacv1.GroupKind:
			for _, group := range userGroups {
				if subject.Name == group {
					return true
				}
			}
		case rbacv1.ServiceAccountKind:
			// ServiceAccount subjects are in format "system:serviceaccount:<namespace>:<name>"
			saName := fmt.Sprintf("system:serviceaccount:%s:%s", subject.Namespace, subject.Name)
			if saName == userName {
				return true
			}
		}
	}

	return false
}

// ruleAllows checks if any rule allows the request
func (r *RBACAuthorizer) ruleAllows(rules []rbacv1.PolicyRule, attrs authorizer.Attributes) bool {
	for _, rule := range rules {
		if r.ruleMatches(rule, attrs) {
			return true
		}
	}
	return false
}

// ruleMatches checks if a single rule matches the request
func (r *RBACAuthorizer) ruleMatches(rule rbacv1.PolicyRule, attrs authorizer.Attributes) bool {
	if attrs.IsResourceRequest() {
		return r.resourceRuleMatches(rule, attrs)
	}
	return r.nonResourceRuleMatches(rule, attrs)
}

// resourceRuleMatches checks if a rule matches a resource request
func (r *RBACAuthorizer) resourceRuleMatches(rule rbacv1.PolicyRule, attrs authorizer.Attributes) bool {
	// Check API groups
	if !r.matchesGroup(rule.APIGroups, attrs.GetAPIGroup()) {
		return false
	}

	// Check resources
	if !r.matchesResource(rule.Resources, attrs.GetResource(), attrs.GetSubresource()) {
		return false
	}

	// Check verbs
	if !r.matchesVerb(rule.Verbs, attrs.GetVerb()) {
		return false
	}

	// Check resource names. A rule scoped by resourceNames only grants the
	// individually named objects, so a request without a name (list/create, or
	// an unscoped get) must NOT match it. resourceNames have no wildcard
	// semantics in RBAC ("*" is a literal name), hence the exact membership
	// check. This mirrors upstream k8s.io/apiserver rbac.ResourceNameMatches.
	if len(rule.ResourceNames) > 0 && !slices.Contains(rule.ResourceNames, attrs.GetName()) {
		return false
	}

	return true
}

// nonResourceRuleMatches checks if a rule matches a non-resource request
func (r *RBACAuthorizer) nonResourceRuleMatches(rule rbacv1.PolicyRule, attrs authorizer.Attributes) bool {
	if len(rule.NonResourceURLs) == 0 {
		return false
	}

	// Check path
	if !r.matchesPath(rule.NonResourceURLs, attrs.GetPath()) {
		return false
	}

	// Check verbs
	if !r.matchesVerb(rule.Verbs, attrs.GetVerb()) {
		return false
	}

	return true
}

// matchesGroup checks if the group matches
func (r *RBACAuthorizer) matchesGroup(groups []string, group string) bool {
	return r.containsOrWildcard(groups, group)
}

// matchesResource checks if the resource matches (including subresource).
// It mirrors upstream RBAC semantics (k8s.io/kubernetes evaluation helpers):
// a rule resource matches when it is "*", the exact "resource/subresource"
// combination, or "*/subresource". Notably, upstream RBAC does NOT treat
// "resource/*" as a wildcard, and a rule for the bare "resource" does NOT
// grant access to its subresources.
func (r *RBACAuthorizer) matchesResource(resources []string, resource, subresource string) bool {
	combined := resource
	if subresource != "" {
		combined = resource + "/" + subresource
	}

	for _, ruleResource := range resources {
		if ruleResource == "*" || ruleResource == combined {
			return true
		}
		if subresource != "" && ruleResource == "*/"+subresource {
			return true
		}
	}
	return false
}

// matchesVerb checks if the verb matches
func (r *RBACAuthorizer) matchesVerb(verbs []string, verb string) bool {
	return r.containsOrWildcard(verbs, verb)
}

// matchesPath checks if the path matches
func (r *RBACAuthorizer) matchesPath(paths []string, path string) bool {
	for _, p := range paths {
		if p == "*" || p == path {
			return true
		}
		// Support wildcard prefix matching like "/api/*"
		if strings.HasSuffix(p, "*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
	}
	return false
}

// containsOrWildcard checks if the slice contains the value or a wildcard
func (r *RBACAuthorizer) containsOrWildcard(slice []string, value string) bool {
	for _, item := range slice {
		if item == "*" || item == value {
			return true
		}
	}
	return false
}
