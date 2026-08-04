/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package rbacadapter

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/klog/v2"
)

// ServiceAccountRef identifies a ServiceAccount subject by namespace and name.
type ServiceAccountRef struct {
	Namespace string
	Name      string
}

// WhoCanResult holds the subjects allowed to perform an action, sorted for
// deterministic output.
type WhoCanResult struct {
	Users           []string
	Groups          []string
	ServiceAccounts []ServiceAccountRef
}

// WhoCan answers the reverse-RBAC question: given an action described by attrs
// (verb on a resource, optionally scoped to a namespace/name/subresource, or a
// non-resource URL), it returns the subjects (Users, Groups, ServiceAccounts)
// that are allowed to perform it.
//
// It reuses the same rule-matching logic as the forward authorizer
// (ruleAllows/ruleMatches), so verb/resource/apiGroup wildcards, subresources
// and resourceNames are handled identically. Aggregated ClusterRoles are
// resolved transparently: kube-controller-manager populates ClusterRole.Rules
// for aggregated roles, and we read the rules straight from the informer-backed
// lister, so the aggregated rules are already present.
//
// Computation is bounded by the number of bindings/roles and uses only the
// informer caches (no live API calls). Each ClusterRoleBinding is visited once,
// and each RoleBinding in the target namespace once.
//
// The returned error is non-fatal: WhoCan still returns whatever subjects it
// resolved, but a non-nil error means the result is partial (e.g. an informer
// list failed), which lets callers distinguish a failure from "nobody can".
// ctx is honored for cancellation before any work is done, mirroring Authorize.
func (r *RBACAuthorizer) WhoCan(ctx context.Context, attrs authorizer.Attributes) (WhoCanResult, error) {
	if err := ctx.Err(); err != nil {
		return WhoCanResult{}, err
	}

	users := map[string]struct{}{}
	groups := map[string]struct{}{}
	serviceAccounts := map[ServiceAccountRef]struct{}{}

	var errs []error

	// ClusterRoleBindings grant access cluster-wide, so they always apply
	// (including to the requested namespace, if any).
	if err := r.collectClusterRoleBindingSubjects(attrs, users, groups, serviceAccounts); err != nil {
		errs = append(errs, err)
	}

	// RoleBindings only apply to namespaced requests.
	if attrs.GetNamespace() != "" {
		if err := r.collectRoleBindingSubjects(attrs, users, groups, serviceAccounts); err != nil {
			errs = append(errs, err)
		}
	}

	return buildWhoCanResult(users, groups, serviceAccounts), errors.Join(errs...)
}

// collectClusterRoleBindingSubjects adds subjects of any ClusterRoleBinding
// whose ClusterRole grants the requested action. A failure to list the
// bindings is returned as an error because it makes the whole cluster-scoped
// result unreliable; per-binding role lookups that miss (a dangling RoleRef)
// are expected and only logged.
func (r *RBACAuthorizer) collectClusterRoleBindingSubjects(attrs authorizer.Attributes, users, groups map[string]struct{}, serviceAccounts map[ServiceAccountRef]struct{}) error {
	bindings, err := r.clusterRoleBindingLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("listing ClusterRoleBindings: %w", err)
	}

	var errs []error
	for _, binding := range bindings {
		// ClusterRoleBindings can only reference ClusterRoles; skip anything
		// else defensively rather than looking it up in the ClusterRole cache.
		if binding.RoleRef.Kind != "ClusterRole" {
			continue
		}

		role, err := r.clusterRoleLister.Get(binding.RoleRef.Name)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("getting ClusterRole %q: %w", binding.RoleRef.Name, err))
			}
			klog.V(5).Infof("WhoCan: failed to get ClusterRole %s: %v", binding.RoleRef.Name, err)
			continue
		}

		if !r.ruleAllows(role.Rules, attrs) {
			continue
		}

		// ClusterRoleBinding ServiceAccount subjects must carry an explicit
		// namespace (there is no binding namespace to default to).
		addWhoCanSubjects(binding.Subjects, "", users, groups, serviceAccounts)
	}
	return errors.Join(errs...)
}

// collectRoleBindingSubjects adds subjects of any RoleBinding in the target
// namespace whose referenced Role/ClusterRole grants the requested action. As
// with the cluster-scoped collector, a list failure is returned while expected
// per-binding misses (a dangling RoleRef) are only logged.
func (r *RBACAuthorizer) collectRoleBindingSubjects(attrs authorizer.Attributes, users, groups map[string]struct{}, serviceAccounts map[ServiceAccountRef]struct{}) error {
	namespace := attrs.GetNamespace()
	bindings, err := r.roleBindingLister.RoleBindings(namespace).List(labels.Everything())
	if err != nil {
		return fmt.Errorf("listing RoleBindings in namespace %q: %w", namespace, err)
	}

	var errs []error
	for _, binding := range bindings {
		var rules []rbacv1.PolicyRule
		if binding.RoleRef.Kind == "ClusterRole" {
			role, err := r.clusterRoleLister.Get(binding.RoleRef.Name)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					errs = append(errs, fmt.Errorf("getting ClusterRole %q: %w", binding.RoleRef.Name, err))
				}
				klog.V(5).Infof("WhoCan: failed to get ClusterRole %s: %v", binding.RoleRef.Name, err)
				continue
			}
			rules = role.Rules
		} else {
			role, err := r.roleLister.Roles(namespace).Get(binding.RoleRef.Name)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					errs = append(errs, fmt.Errorf("getting Role %q/%q: %w", namespace, binding.RoleRef.Name, err))
				}
				klog.V(5).Infof("WhoCan: failed to get Role %s/%s: %v", namespace, binding.RoleRef.Name, err)
				continue
			}
			rules = role.Rules
		}

		if !r.ruleAllows(rules, attrs) {
			continue
		}

		// For RoleBindings, a ServiceAccount subject with an empty namespace
		// defaults to the RoleBinding's namespace.
		addWhoCanSubjects(binding.Subjects, namespace, users, groups, serviceAccounts)
	}
	return errors.Join(errs...)
}

// addWhoCanSubjects classifies and accumulates binding subjects into the result sets.
func addWhoCanSubjects(subjects []rbacv1.Subject, defaultNamespace string, users, groups map[string]struct{}, serviceAccounts map[ServiceAccountRef]struct{}) {
	for _, subject := range subjects {
		switch subject.Kind {
		case rbacv1.UserKind:
			users[subject.Name] = struct{}{}
		case rbacv1.GroupKind:
			groups[subject.Name] = struct{}{}
		case rbacv1.ServiceAccountKind:
			namespace := subject.Namespace
			if namespace == "" {
				namespace = defaultNamespace
			}
			serviceAccounts[ServiceAccountRef{Namespace: namespace, Name: subject.Name}] = struct{}{}
		}
	}
}

// buildWhoCanResult converts the dedup sets into sorted slices.
func buildWhoCanResult(users, groups map[string]struct{}, serviceAccounts map[ServiceAccountRef]struct{}) WhoCanResult {
	result := WhoCanResult{
		Users:           make([]string, 0, len(users)),
		Groups:          make([]string, 0, len(groups)),
		ServiceAccounts: make([]ServiceAccountRef, 0, len(serviceAccounts)),
	}

	for u := range users {
		result.Users = append(result.Users, u)
	}
	for g := range groups {
		result.Groups = append(result.Groups, g)
	}
	for sa := range serviceAccounts {
		result.ServiceAccounts = append(result.ServiceAccounts, sa)
	}

	slices.Sort(result.Users)
	slices.Sort(result.Groups)
	slices.SortFunc(result.ServiceAccounts, func(a, b ServiceAccountRef) int {
		if c := strings.Compare(a.Namespace, b.Namespace); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})

	return result
}
