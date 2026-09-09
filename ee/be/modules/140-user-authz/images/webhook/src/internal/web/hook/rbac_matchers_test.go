/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
)

// These matchers decide whether CAR-independent RBAC grants a request, and that answer LIFTS the
// multi-tenancy denial. The file they live in says it plainly: being stricter than upstream is
// acceptable, being looser is not. Until this test existed, `resourceNameMatches` could be replaced
// with `return true` and the whole suite stayed green - which is the one direction that matters,
// because a loose match hands a subject a resource its rule was meant to keep out of reach.

func TestResourceNameMatches(t *testing.T) {
	cases := []struct {
		name      string
		ruleNames []string
		requested string
		want      bool
	}{
		{"a rule without resourceNames covers every name", nil, "anything", true},
		{"...including a request that carries no name at all", nil, "", true},
		{"an exact name matches", []string{"kube-root-ca.crt"}, "kube-root-ca.crt", true},
		{"one of several matches", []string{"a", "b", "c"}, "b", true},
		{"a different name does not", []string{"kube-root-ca.crt"}, "my-secret", false},
		// This is the case that makes the difference: list and watch carry no name, and upstream
		// RBAC does not let a resourceNames rule grant them. A looser answer here would turn a
		// grant on one named object into a grant on the whole collection.
		{"a named rule does not cover a request with no name", []string{"kube-root-ca.crt"}, "", false},
		{"the empty string is not a wildcard", []string{""}, "kube-root-ca.crt", false},
		{"a wildcard is a literal name, not a pattern", []string{"*"}, "kube-root-ca.crt", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := resourceNameMatches(tc.ruleNames, tc.requested); got != tc.want {
				t.Errorf("resourceNameMatches(%q, %q) = %v, want %v", tc.ruleNames, tc.requested, got, tc.want)
			}
		})
	}
}

func TestSubjectsMatch(t *testing.T) {
	spec := func(user string, groups ...string) *WebhookResourceSpec {
		return &WebhookResourceSpec{User: user, Group: groups}
	}

	cases := []struct {
		name             string
		subjects         []rbacv1.Subject
		spec             *WebhookResourceSpec
		defaultNamespace string
		want             bool
	}{
		{"a user by name", []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "alice"}}, spec("alice"), "", true},
		{"a different user", []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "alice"}}, spec("bob"), "", false},
		{"a group the request carries", []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: "devs"}}, spec("alice", "ops", "devs"), "", true},
		{"a group it does not", []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: "devs"}}, spec("alice", "ops"), "", false},
		{"the second subject can be the match", []rbacv1.Subject{
			{Kind: rbacv1.UserKind, Name: "bob"},
			{Kind: rbacv1.GroupKind, Name: "devs"},
		}, spec("alice", "devs"), "", true},

		// A ServiceAccount subject with no namespace takes the binding's namespace. Defaulting it
		// to the wrong namespace would match a same-named ServiceAccount from somewhere else.
		{"a service account with an explicit namespace", []rbacv1.Subject{
			{Kind: rbacv1.ServiceAccountKind, Name: "builder", Namespace: "ci"},
		}, spec("system:serviceaccount:ci:builder"), "other", true},
		{"a service account defaulted to the binding's namespace", []rbacv1.Subject{
			{Kind: rbacv1.ServiceAccountKind, Name: "builder"},
		}, spec("system:serviceaccount:ci:builder"), "ci", true},
		{"the same name in another namespace does not match", []rbacv1.Subject{
			{Kind: rbacv1.ServiceAccountKind, Name: "builder", Namespace: "ci"},
		}, spec("system:serviceaccount:prod:builder"), "ci", false},
		{"a defaulted service account does not match another namespace", []rbacv1.Subject{
			{Kind: rbacv1.ServiceAccountKind, Name: "builder"},
		}, spec("system:serviceaccount:prod:builder"), "ci", false},

		// A kind this code does not know must not match. Silently treating it as a user would let
		// an unexpected subject lift the multi-tenancy denial.
		{"an unknown subject kind matches nothing", []rbacv1.Subject{
			{Kind: "Robot", Name: "alice"},
		}, spec("alice"), "", false},
		{"no subjects at all", nil, spec("alice", "devs"), "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := subjectsMatch(tc.subjects, tc.spec, tc.defaultNamespace); got != tc.want {
				t.Errorf("subjectsMatch() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRuleAllows(t *testing.T) {
	attrs := func(verb, group, resource, subresource, name string) *WebhookResourceAttributes {
		return &WebhookResourceAttributes{
			Verb: verb, Group: group, Resource: resource, Subresource: subresource, Name: name,
		}
	}

	cases := []struct {
		name  string
		rule  rbacv1.PolicyRule
		attrs *WebhookResourceAttributes
		want  bool
	}{
		{"everything matches a full wildcard", rbacv1.PolicyRule{
			Verbs: []string{rbacv1.VerbAll}, APIGroups: []string{rbacv1.APIGroupAll}, Resources: []string{rbacv1.ResourceAll},
		}, attrs("get", "apps", "deployments", "", "web"), true},

		{"a verb outside the rule", rbacv1.PolicyRule{
			Verbs: []string{"get", "list"}, APIGroups: []string{""}, Resources: []string{"pods"},
		}, attrs("delete", "", "pods", "", ""), false},

		{"a group outside the rule", rbacv1.PolicyRule{
			Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"pods"},
		}, attrs("get", "apps", "pods", "", ""), false},

		{"a subresource is not covered by the resource", rbacv1.PolicyRule{
			Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"pods"},
		}, attrs("get", "", "pods", "log", ""), false},

		{"a subresource named explicitly is", rbacv1.PolicyRule{
			Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"pods/log"},
		}, attrs("get", "", "pods", "log", ""), true},

		// The combination the multi-tenancy layer cares about: a grant on one named object must
		// not read as a grant on the collection.
		{"a named rule grants that name", rbacv1.PolicyRule{
			Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"configmaps"},
			ResourceNames: []string{"kube-root-ca.crt"},
		}, attrs("get", "", "configmaps", "", "kube-root-ca.crt"), true},
		{"and no other", rbacv1.PolicyRule{
			Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"configmaps"},
			ResourceNames: []string{"kube-root-ca.crt"},
		}, attrs("get", "", "configmaps", "", "app-config"), false},
		{"and not the collection", rbacv1.PolicyRule{
			Verbs: []string{"list"}, APIGroups: []string{""}, Resources: []string{"configmaps"},
			ResourceNames: []string{"kube-root-ca.crt"},
		}, attrs("list", "", "configmaps", "", ""), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rule := tc.rule
			if got := ruleAllows(&rule, tc.attrs); got != tc.want {
				t.Errorf("ruleAllows() = %v, want %v", got, tc.want)
			}
		})
	}
}
