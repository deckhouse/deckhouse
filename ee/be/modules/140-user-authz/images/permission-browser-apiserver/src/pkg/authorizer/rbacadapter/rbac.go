/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package rbacadapter

import (
	"context"
	"fmt"
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

// checkClusterRoleBindings checks if the user has access via ClusterRoleBindings
func (r *RBACAuthorizer) checkClusterRoleBindings(attrs authorizer.Attributes, userName string, userGroups []string) (bool, string) {
	bindings, err := r.clusterRoleBindingLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list ClusterRoleBindings: %v", err)
		return false, ""
	}

	for _, binding := range bindings {
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
func (r *RBACAuthorizer) subjectMatches(subjects []rbacv1.Subject, userName string, userGroups []string, namespace string) bool {
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

	// Check resource names (if specified)
	if len(rule.ResourceNames) > 0 && attrs.GetName() != "" {
		if !r.containsOrWildcard(rule.ResourceNames, attrs.GetName()) {
			return false
		}
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

// matchesResource checks if the resource matches (including subresource)
func (r *RBACAuthorizer) matchesResource(resources []string, resource, subresource string) bool {
	if subresource != "" {
		// Check for exact match with subresource
		combined := resource + "/" + subresource
		if r.containsOrWildcard(resources, combined) {
			return true
		}
		// Check for wildcard subresource
		if r.containsOrWildcard(resources, resource+"/*") {
			return true
		}
		// Check for */subresource
		if r.containsOrWildcard(resources, "*/"+subresource) {
			return true
		}
	}
	return r.containsOrWildcard(resources, resource)
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
