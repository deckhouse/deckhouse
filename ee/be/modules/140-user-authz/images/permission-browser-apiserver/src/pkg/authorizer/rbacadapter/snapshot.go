/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package rbacadapter

import (
	"context"
	"fmt"
	"slices"
	"sync"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/klog/v2"
)

type subjectRulesKey struct{}

// BindSubject attaches a one-request snapshot of u's RBAC rules to ctx.
// Authorize and AllowsIndependently reuse it instead of listing
// ClusterRoleBindings again. Do not bind the review subject before a
// non-self gate that authorizes the caller.
func (r *RBACAuthorizer) BindSubject(ctx context.Context, u user.Info) context.Context {
	if u == nil {
		return ctx
	}
	return context.WithValue(ctx, subjectRulesKey{}, r.Snapshot(u))
}

func subjectRulesFrom(ctx context.Context) *SubjectRules {
	s, _ := ctx.Value(subjectRulesKey{}).(*SubjectRules)
	return s
}

type boundRuleSet struct {
	rules  []rbacv1.PolicyRule
	reason string
}

// SubjectRules is a per-request snapshot of the ClusterRoleBindings that
// match a subject, plus a lazy per-namespace RoleBinding cache. It is not
// shared across HTTP requests.
type SubjectRules struct {
	userName   string
	userGroups []string
	authorizer *RBACAuthorizer

	cluster            []boundRuleSet
	independentCluster []boundRuleSet

	nsMu    sync.Mutex
	nsRules map[string][]boundRuleSet
}

// Snapshot walks ClusterRoleBindings once and records two rule sets: every
// matching binding, and the subset that is not CAR-managed (for
// AllowsIndependently). RoleBindings are loaded later, per namespace.
func (r *RBACAuthorizer) Snapshot(u user.Info) *SubjectRules {
	s := &SubjectRules{
		userName:   u.GetName(),
		userGroups: u.GetGroups(),
		authorizer: r,
		nsRules:    make(map[string][]boundRuleSet),
	}
	if r.clusterRoleBindingLister == nil {
		return s
	}

	bindings, err := r.clusterRoleBindingLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list ClusterRoleBindings: %v", err)
		return s
	}

	for _, binding := range bindings {
		if !r.subjectMatches(binding.Subjects, s.userName, s.userGroups, "") {
			continue
		}

		role, err := r.clusterRoleLister.Get(binding.RoleRef.Name)
		if err != nil {
			klog.V(5).Infof("Failed to get ClusterRole %s: %v", binding.RoleRef.Name, err)
			continue
		}

		set := boundRuleSet{
			rules: role.Rules,
			reason: fmt.Sprintf("RBAC: allowed by ClusterRoleBinding %q of ClusterRole %q to user %q",
				binding.Name, role.Name, s.userName),
		}
		s.cluster = append(s.cluster, set)
		if !IsCARManagedClusterRoleBinding(binding) {
			s.independentCluster = append(s.independentCluster, set)
		}
	}

	return s
}

func (r *RBACAuthorizer) rulesFor(ctx context.Context, u user.Info) *SubjectRules {
	if s := subjectRulesFrom(ctx); s != nil && s.matches(u) {
		return s
	}
	return r.Snapshot(u)
}

// matches reports whether the snapshot was taken for exactly this subject.
// Groups participate: two subjects can share a name and still resolve to
// different bindings, and answering one from the other's snapshot would
// report an access level the subject does not have.
func (s *SubjectRules) matches(u user.Info) bool {
	return s.userName == u.GetName() && slices.Equal(s.userGroups, u.GetGroups())
}

func (s *SubjectRules) allows(attrs authorizer.Attributes, independent bool) (bool, string) {
	sets := s.cluster
	if independent {
		sets = s.independentCluster
	}
	for _, set := range sets {
		if s.authorizer.ruleAllows(set.rules, attrs) {
			return true, set.reason
		}
	}

	namespace := attrs.GetNamespace()
	if namespace == "" {
		return false, ""
	}
	for _, set := range s.namespaceRules(namespace) {
		if s.authorizer.ruleAllows(set.rules, attrs) {
			return true, set.reason
		}
	}
	return false, ""
}

func (s *SubjectRules) namespaceRules(namespace string) []boundRuleSet {
	s.nsMu.Lock()
	defer s.nsMu.Unlock()

	if rules, ok := s.nsRules[namespace]; ok {
		return rules
	}
	rules := s.authorizer.loadRoleBindingRules(namespace, s.userName, s.userGroups)
	s.nsRules[namespace] = rules
	return rules
}

func (r *RBACAuthorizer) loadRoleBindingRules(namespace, userName string, userGroups []string) []boundRuleSet {
	if r.roleBindingLister == nil {
		return nil
	}

	bindings, err := r.roleBindingLister.RoleBindings(namespace).List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list RoleBindings in namespace %s: %v", namespace, err)
		return nil
	}

	var out []boundRuleSet
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

		out = append(out, boundRuleSet{
			rules: rules,
			reason: fmt.Sprintf("RBAC: allowed by RoleBinding %q of %s %q to user %q",
				binding.Name, binding.RoleRef.Kind, binding.RoleRef.Name, userName),
		})
	}
	return out
}
