/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// Package resolver provides algorithms for resolving user access to namespaces.
package resolver

import (
	"fmt"
	"sort"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/klog/v2"

	"permission-browser-apiserver/pkg/authorizer/multitenancy"
)

// NamespaceResolver resolves which namespaces a user has access to.
// It combines RBAC analysis with multi-tenancy filtering.
type NamespaceResolver struct {
	nsLister                 corev1listers.NamespaceLister
	roleLister               rbaclisters.RoleLister
	roleBindingLister        rbaclisters.RoleBindingLister
	clusterRoleLister        rbaclisters.ClusterRoleLister
	clusterRoleBindingLister rbaclisters.ClusterRoleBindingLister
	scopeCache               *ResourceScopeCache
	mtEngine                 *multitenancy.Engine
}

// NewNamespaceResolver creates a new namespace resolver.
func NewNamespaceResolver(
	nsLister corev1listers.NamespaceLister,
	roleLister rbaclisters.RoleLister,
	roleBindingLister rbaclisters.RoleBindingLister,
	clusterRoleLister rbaclisters.ClusterRoleLister,
	clusterRoleBindingLister rbaclisters.ClusterRoleBindingLister,
	scopeCache *ResourceScopeCache,
	mtEngine *multitenancy.Engine,
) *NamespaceResolver {
	return &NamespaceResolver{
		nsLister:                 nsLister,
		roleLister:               roleLister,
		roleBindingLister:        roleBindingLister,
		clusterRoleLister:        clusterRoleLister,
		clusterRoleBindingLister: clusterRoleBindingLister,
		scopeCache:               scopeCache,
		mtEngine:                 mtEngine,
	}
}

// ResolveAccessibleNamespaces returns a sorted list of namespace names that
// the given user has access to (any namespaced RBAC permission AND multi-tenancy allows).
func (r *NamespaceResolver) ResolveAccessibleNamespaces(userInfo user.Info) ([]string, error) {
	if userInfo == nil {
		return nil, nil
	}

	userName := userInfo.GetName()
	userGroups := userInfo.GetGroups()

	// Step 1: Check if user has global namespaced access via ClusterRoleBindings
	globalAccess, err := r.hasGlobalNamespacedAccess(userName, userGroups)
	if err != nil {
		klog.V(4).Infof("Error checking global namespaced access: %v", err)
		// Continue with RoleBinding-based resolution
	}

	var candidateNamespaces map[string]struct{}

	if globalAccess {
		// User has cluster-wide namespaced access, get all namespaces
		candidateNamespaces, err = r.getAllNamespaces()
		if err != nil {
			return nil, fmt.Errorf("failed to list namespaces: %w", err)
		}
		klog.V(5).Infof("User %s has global namespaced access, candidate namespaces: %d", userName, len(candidateNamespaces))
	} else {
		// Scan RoleBindings to find namespaces with access
		candidateNamespaces, err = r.getNamespacesFromRoleBindings(userName, userGroups)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve namespaces from RoleBindings: %w", err)
		}
		klog.V(5).Infof("User %s has access via RoleBindings to %d namespaces", userName, len(candidateNamespaces))
	}

	// Step 2: Filter by multi-tenancy rules
	result := r.filterByMultitenancy(userInfo, candidateNamespaces)

	// Step 3: Sort for deterministic output
	sort.SliceStable(result, func(i, j int) bool {
		return result[i] < result[j]
	})

	klog.V(4).Infof("User %s has access to %d namespaces after multi-tenancy filtering", userName, len(result))
	return result, nil
}

// IsNamespaceAccessible checks if a specific namespace is accessible to the user.
// This is used for GET requests to avoid existence disclosure.
func (r *NamespaceResolver) IsNamespaceAccessible(userInfo user.Info, namespace string) (bool, error) {
	if userInfo == nil {
		return false, nil
	}

	// First check if namespace exists
	_, err := r.nsLister.Get(namespace)
	if err != nil {
		// Return false without error to avoid existence disclosure
		return false, nil
	}

	// Check multi-tenancy first (fast path for denial)
	if !r.isNamespaceAllowedByMultitenancy(userInfo, namespace) {
		return false, nil
	}

	userName := userInfo.GetName()
	userGroups := userInfo.GetGroups()

	// Check global access via ClusterRoleBindings.
	// Error is intentionally not returned here: this is a fail-open check.
	// If we can't determine global access (e.g., informer cache issue), we fall through
	// to RoleBinding check which may still grant access. Returning error here would
	// deny access to users who have valid RoleBindings in the namespace.
	globalAccess, err := r.hasGlobalNamespacedAccess(userName, userGroups)
	if err != nil {
		klog.V(4).Infof("Error checking global access (continuing with RoleBinding check): %v", err)
	}
	if globalAccess {
		return true, nil
	}

	// Check RoleBindings in the specific namespace
	return r.hasAccessViaRoleBindings(userName, userGroups, namespace)
}

// hasGlobalNamespacedAccess checks if the user has any ClusterRoleBinding that
// grants access to namespaced resources cluster-wide.
func (r *NamespaceResolver) hasGlobalNamespacedAccess(userName string, userGroups []string) (bool, error) {
	crbs, err := r.clusterRoleBindingLister.List(labels.Everything())
	if err != nil {
		return false, fmt.Errorf("failed to list ClusterRoleBindings: %w", err)
	}

	for _, crb := range crbs {
		if !r.subjectMatches(crb.Subjects, userName, userGroups, "") {
			continue
		}

		// Get the ClusterRole
		clusterRole, err := r.clusterRoleLister.Get(crb.RoleRef.Name)
		if err != nil {
			klog.V(5).Infof("Failed to get ClusterRole %s: %v", crb.RoleRef.Name, err)
			continue
		}

		// Check if any rule grants namespaced access
		if r.hasNamespacedRules(clusterRole.Rules) {
			klog.V(5).Infof("User has global namespaced access via ClusterRoleBinding %s -> ClusterRole %s",
				crb.Name, clusterRole.Name)
			return true, nil
		}
	}

	return false, nil
}

// getAllNamespaces returns all namespace names from the informer cache.
func (r *NamespaceResolver) getAllNamespaces() (map[string]struct{}, error) {
	namespaces, err := r.nsLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	result := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		result[ns.Name] = struct{}{}
	}
	return result, nil
}

// getNamespacesFromRoleBindings finds all namespaces where the user has
// RoleBindings that grant namespaced access.
func (r *NamespaceResolver) getNamespacesFromRoleBindings(userName string, userGroups []string) (map[string]struct{}, error) {
	result := make(map[string]struct{})

	// List all RoleBindings across all namespaces
	rbs, err := r.roleBindingLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list RoleBindings: %w", err)
	}

	for _, rb := range rbs {
		// Check if subject matches (with SA namespace defaulting)
		if !r.subjectMatchesWithNamespaceDefault(rb.Subjects, userName, userGroups, rb.Namespace) {
			continue
		}

		// Get the Role or ClusterRole
		var rules []rbacv1.PolicyRule
		if rb.RoleRef.Kind == "ClusterRole" {
			cr, err := r.clusterRoleLister.Get(rb.RoleRef.Name)
			if err != nil {
				klog.V(5).Infof("Failed to get ClusterRole %s for RoleBinding %s/%s: %v",
					rb.RoleRef.Name, rb.Namespace, rb.Name, err)
				continue
			}
			rules = cr.Rules
		} else {
			role, err := r.roleLister.Roles(rb.Namespace).Get(rb.RoleRef.Name)
			if err != nil {
				klog.V(5).Infof("Failed to get Role %s/%s: %v", rb.Namespace, rb.RoleRef.Name, err)
				continue
			}
			rules = role.Rules
		}

		// Check if rules grant namespaced access
		if r.hasNamespacedRules(rules) {
			result[rb.Namespace] = struct{}{}
		}
	}

	return result, nil
}

// hasAccessViaRoleBindings checks if user has any RoleBinding in the namespace
// that grants namespaced access.
func (r *NamespaceResolver) hasAccessViaRoleBindings(userName string, userGroups []string, namespace string) (bool, error) {
	rbs, err := r.roleBindingLister.RoleBindings(namespace).List(labels.Everything())
	if err != nil {
		return false, fmt.Errorf("failed to list RoleBindings in namespace %s: %w", namespace, err)
	}

	for _, rb := range rbs {
		if !r.subjectMatchesWithNamespaceDefault(rb.Subjects, userName, userGroups, rb.Namespace) {
			continue
		}

		var rules []rbacv1.PolicyRule
		if rb.RoleRef.Kind == "ClusterRole" {
			cr, err := r.clusterRoleLister.Get(rb.RoleRef.Name)
			if err != nil {
				continue
			}
			rules = cr.Rules
		} else {
			role, err := r.roleLister.Roles(namespace).Get(rb.RoleRef.Name)
			if err != nil {
				continue
			}
			rules = role.Rules
		}

		if r.hasNamespacedRules(rules) {
			return true, nil
		}
	}

	return false, nil
}

// hasNamespacedRules checks if any rule in the list grants access to namespaced resources.
func (r *NamespaceResolver) hasNamespacedRules(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		// NonResourceURLs only apply to cluster-scoped requests
		if len(rule.NonResourceURLs) > 0 && len(rule.Resources) == 0 {
			continue
		}

		// Check if any verb is present (empty verbs = no access)
		if len(rule.Verbs) == 0 {
			continue
		}

		// Wildcard resources: assume namespaced access exists
		for _, res := range rule.Resources {
			if res == "*" {
				return true
			}
		}

		// Wildcard API groups with any resource: assume namespaced access
		hasWildcardGroup := false
		for _, group := range rule.APIGroups {
			if group == "*" {
				hasWildcardGroup = true
				break
			}
		}
		if hasWildcardGroup && len(rule.Resources) > 0 {
			// With wildcard group, any resource could be namespaced
			return true
		}

		// For specific resources, check if any is namespaced
		for _, group := range rule.APIGroups {
			for _, resource := range rule.Resources {
				// Strip subresource if present
				baseResource := strings.Split(resource, "/")[0]
				if r.isResourceNamespaced(group, baseResource) {
					return true
				}
			}
		}
	}

	return false
}

// isResourceNamespaced checks if a resource is namespaced using the scope cache.
//
// IMPORTANT: This function is used to decide whether the user has ANY namespaced access
// via ClusterRoleBindings / RoleBindings. A false positive here results in listing
// *all* namespaces as accessible (info leak). Therefore, for unknown resources
// we fail CLOSED (assume cluster-scoped).
func (r *NamespaceResolver) isResourceNamespaced(group, resource string) bool {
	if r.scopeCache == nil {
		klog.V(5).Infof("No scope cache, assuming %s/%s is cluster-scoped", group, resource)
		return false
	}
	return r.scopeCache.IsNamespaced(group, resource)
}

// subjectMatches checks if any subject matches the user (for ClusterRoleBindings).
func (r *NamespaceResolver) subjectMatches(subjects []rbacv1.Subject, userName string, userGroups []string, namespace string) bool {
	for _, subject := range subjects {
		if r.singleSubjectMatches(subject, userName, userGroups, namespace) {
			return true
		}
	}
	return false
}

// subjectMatchesWithNamespaceDefault handles the case where ServiceAccount
// subject.namespace is empty - it defaults to the RoleBinding's namespace.
func (r *NamespaceResolver) subjectMatchesWithNamespaceDefault(subjects []rbacv1.Subject, userName string, userGroups []string, rbNamespace string) bool {
	for _, subject := range subjects {
		// For ServiceAccount with empty namespace, use RoleBinding's namespace
		if subject.Kind == rbacv1.ServiceAccountKind && subject.Namespace == "" {
			subject = *subject.DeepCopy()
			subject.Namespace = rbNamespace
		}
		if r.singleSubjectMatches(subject, userName, userGroups, rbNamespace) {
			return true
		}
	}
	return false
}

// singleSubjectMatches checks if a single subject matches the user.
func (r *NamespaceResolver) singleSubjectMatches(subject rbacv1.Subject, userName string, userGroups []string, namespace string) bool {
	switch subject.Kind {
	case rbacv1.UserKind:
		return subject.Name == userName
	case rbacv1.GroupKind:
		for _, group := range userGroups {
			if subject.Name == group {
				return true
			}
		}
	case rbacv1.ServiceAccountKind:
		// ServiceAccount user format: system:serviceaccount:<namespace>:<name>
		saNamespace := subject.Namespace
		if saNamespace == "" {
			saNamespace = namespace
		}
		expectedName := fmt.Sprintf("system:serviceaccount:%s:%s", saNamespace, subject.Name)
		return expectedName == userName
	}
	return false
}

// filterByMultitenancy filters the candidate namespaces using multi-tenancy rules.
func (r *NamespaceResolver) filterByMultitenancy(userInfo user.Info, candidates map[string]struct{}) []string {
	if r.mtEngine == nil {
		// No multi-tenancy engine, return all candidates
		result := make([]string, 0, len(candidates))
		for ns := range candidates {
			result = append(result, ns)
		}
		return result
	}

	result := make([]string, 0, len(candidates))
	for ns := range candidates {
		if r.isNamespaceAllowedByMultitenancy(userInfo, ns) {
			result = append(result, ns)
		}
	}
	return result
}

// isNamespaceAllowedByMultitenancy checks if multi-tenancy allows access to the namespace.
func (r *NamespaceResolver) isNamespaceAllowedByMultitenancy(userInfo user.Info, namespace string) bool {
	if r.mtEngine == nil {
		return true
	}

	// Use the engine's exported method if available, or use Authorize
	return r.mtEngine.IsNamespaceAllowed(userInfo, namespace)
}
