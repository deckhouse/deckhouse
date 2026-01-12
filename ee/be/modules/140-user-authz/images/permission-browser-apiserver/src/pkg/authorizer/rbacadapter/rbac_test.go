/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package rbacadapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
)

func TestRuleMatches_Resource(t *testing.T) {
	r := &RBACAuthorizer{}

	tests := []struct {
		name     string
		rule     rbacv1.PolicyRule
		attrs    *mockAttrs
		expected bool
	}{
		{
			name: "exact match",
			rule: rbacv1.PolicyRule{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			},
			attrs: &mockAttrs{
				verb:       "get",
				resource:   "pods",
				apiGroup:   "",
				isResource: true,
			},
			expected: true,
		},
		{
			name: "wildcard verb",
			rule: rbacv1.PolicyRule{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"*"},
			},
			attrs: &mockAttrs{
				verb:       "delete",
				resource:   "pods",
				apiGroup:   "",
				isResource: true,
			},
			expected: true,
		},
		{
			name: "wildcard resource",
			rule: rbacv1.PolicyRule{
				APIGroups: []string{""},
				Resources: []string{"*"},
				Verbs:     []string{"get"},
			},
			attrs: &mockAttrs{
				verb:       "get",
				resource:   "secrets",
				apiGroup:   "",
				isResource: true,
			},
			expected: true,
		},
		{
			name: "wildcard api group",
			rule: rbacv1.PolicyRule{
				APIGroups: []string{"*"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get"},
			},
			attrs: &mockAttrs{
				verb:       "get",
				resource:   "deployments",
				apiGroup:   "apps",
				isResource: true,
			},
			expected: true,
		},
		{
			name: "subresource match",
			rule: rbacv1.PolicyRule{
				APIGroups: []string{""},
				Resources: []string{"pods/log"},
				Verbs:     []string{"get"},
			},
			attrs: &mockAttrs{
				verb:        "get",
				resource:    "pods",
				subresource: "log",
				apiGroup:    "",
				isResource:  true,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.ruleMatches(tt.rule, tt.attrs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesPath(t *testing.T) {
	r := &RBACAuthorizer{}

	tests := []struct {
		paths    []string
		path     string
		expected bool
	}{
		{[]string{"/api/*"}, "/api/v1", true},
		{[]string{"/api/*"}, "/apis/apps/v1", false},
		{[]string{"/api/v1"}, "/api/v1", true},
		{[]string{"/api/v1"}, "/api/v2", false},
		{[]string{"*"}, "/anything", true},
		{[]string{"/healthz", "/livez"}, "/healthz", true},
		{[]string{"/healthz", "/livez"}, "/readyz", false},
	}

	for _, tt := range tests {
		result := r.matchesPath(tt.paths, tt.path)
		assert.Equal(t, tt.expected, result)
	}
}

func TestContainsOrWildcard(t *testing.T) {
	r := &RBACAuthorizer{}

	tests := []struct {
		slice    []string
		value    string
		expected bool
	}{
		{[]string{"a", "b", "c"}, "a", true},
		{[]string{"a", "b", "c"}, "d", false},
		{[]string{"*"}, "anything", true},
		{[]string{"a", "*"}, "anything", true},
	}

	for _, tt := range tests {
		result := r.containsOrWildcard(tt.slice, tt.value)
		assert.Equal(t, tt.expected, result)
	}
}

// mockAttrs implements authorizer.Attributes for testing
type mockAttrs struct {
	user        user.Info
	verb        string
	resource    string
	subresource string
	apiGroup    string
	apiVersion  string
	namespace   string
	name        string
	path        string
	isResource  bool
}

func (m *mockAttrs) GetUser() user.Info { return m.user }
func (m *mockAttrs) GetVerb() string    { return m.verb }
func (m *mockAttrs) IsReadOnly() bool {
	return m.verb == "get" || m.verb == "list" || m.verb == "watch"
}
func (m *mockAttrs) GetNamespace() string                              { return m.namespace }
func (m *mockAttrs) GetResource() string                               { return m.resource }
func (m *mockAttrs) GetSubresource() string                            { return m.subresource }
func (m *mockAttrs) GetName() string                                   { return m.name }
func (m *mockAttrs) GetAPIGroup() string                               { return m.apiGroup }
func (m *mockAttrs) GetAPIVersion() string                             { return m.apiVersion }
func (m *mockAttrs) IsResourceRequest() bool                           { return m.isResource }
func (m *mockAttrs) GetPath() string                                   { return m.path }
func (m *mockAttrs) GetFieldSelector() (fields.Requirements, error)    { return nil, nil }
func (m *mockAttrs) GetLabelSelector() (labels.Requirements, error)    { return nil, nil }

// mockUser implements user.Info for testing
type mockUser struct {
	name   string
	groups []string
}

func (m *mockUser) GetName() string               { return m.name }
func (m *mockUser) GetUID() string                { return "" }
func (m *mockUser) GetGroups() []string           { return m.groups }
func (m *mockUser) GetExtra() map[string][]string { return nil }
