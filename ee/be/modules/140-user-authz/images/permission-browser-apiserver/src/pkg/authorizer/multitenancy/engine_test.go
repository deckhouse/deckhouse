/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// Ensure mockUserInfo implements user.Info for IsNamespaceAllowed tests
var _ user.Info = &mockUserInfo{}

func TestHasAnyFilters(t *testing.T) {
	tests := []struct {
		name     string
		entry    *DirectoryEntry
		expected bool
	}{
		{
			name: "no filters at all",
			entry: &DirectoryEntry{
				NamespaceFiltersAbsent:        true,
				AllowAccessToSystemNamespaces: true,
			},
			expected: false,
		},
		{
			name: "no filters but system namespaces restricted",
			entry: &DirectoryEntry{
				NamespaceFiltersAbsent:        true,
				AllowAccessToSystemNamespaces: false,
			},
			expected: true,
		},
		{
			name: "has limit namespaces",
			entry: &DirectoryEntry{
				LimitNamespaces: []*regexp.Regexp{
					regexp.MustCompile("^myapp-.*$"),
				},
			},
			expected: true,
		},
		{
			name: "wildcard pattern allows all",
			entry: &DirectoryEntry{
				LimitNamespaces: []*regexp.Regexp{
					regexp.MustCompile("^.*$"),
				},
				AllowAccessToSystemNamespaces: true,
			},
			expected: false,
		},
		{
			name: "matchAny selector allows all",
			entry: &DirectoryEntry{
				NamespaceSelectors: []*NamespaceSelector{
					{MatchAny: true},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAnyFilters(tt.entry)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWrapRegex(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"myapp", "^myapp$"},
		{"^myapp", "^myapp$"},
		{"myapp$", "^myapp$"},
		{"^myapp$", "^myapp$"},
		{"myapp-.*", "^myapp-.*$"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := wrapRegex(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCombineDirEntries(t *testing.T) {
	e := &Engine{}

	entries := []DirectoryEntry{
		{
			AllowAccessToSystemNamespaces: false,
			LimitNamespaces: []*regexp.Regexp{
				regexp.MustCompile("^ns1$"),
			},
			NamespaceFiltersAbsent: false,
		},
		{
			AllowAccessToSystemNamespaces: true,
			LimitNamespaces: []*regexp.Regexp{
				regexp.MustCompile("^ns2$"),
			},
			NamespaceFiltersAbsent: true,
		},
	}

	combined := e.combineDirEntries(entries)

	assert.True(t, combined.AllowAccessToSystemNamespaces)
	assert.True(t, combined.NamespaceFiltersAbsent)
	assert.Len(t, combined.LimitNamespaces, 2)
}

func TestIsLabelSelectorApplied(t *testing.T) {
	tests := []struct {
		name     string
		selector *NamespaceSelector
		expected bool
	}{
		{
			name:     "nil selector",
			selector: nil,
			expected: false,
		},
		{
			name:     "nil label selector",
			selector: &NamespaceSelector{LabelSelector: nil},
			expected: false,
		},
		{
			name: "has label selector",
			selector: &NamespaceSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
			},
			expected: true,
		},
		{
			name: "has empty label selector",
			selector: &NamespaceSelector{
				LabelSelector: &metav1.LabelSelector{},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLabelSelectorApplied(tt.selector)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// mockAttrs implements authorizer.Attributes for testing
type mockAttrs struct {
	userInfo    user.Info
	verb        string
	namespace   string
	resource    string
	subresource string
	apiGroup    string
	apiVersion  string
	name        string
	path        string
	isResource  bool
}

func (m *mockAttrs) GetUser() user.Info { return m.userInfo }
func (m *mockAttrs) GetVerb() string    { return m.verb }
func (m *mockAttrs) IsReadOnly() bool {
	return m.verb == "get" || m.verb == "list" || m.verb == "watch"
}
func (m *mockAttrs) GetNamespace() string                           { return m.namespace }
func (m *mockAttrs) GetResource() string                            { return m.resource }
func (m *mockAttrs) GetSubresource() string                         { return m.subresource }
func (m *mockAttrs) GetName() string                                { return m.name }
func (m *mockAttrs) GetAPIGroup() string                            { return m.apiGroup }
func (m *mockAttrs) GetAPIVersion() string                          { return m.apiVersion }
func (m *mockAttrs) IsResourceRequest() bool                        { return m.isResource }
func (m *mockAttrs) GetPath() string                                { return m.path }
func (m *mockAttrs) GetFieldSelector() (fields.Requirements, error) { return nil, nil }
func (m *mockAttrs) GetLabelSelector() (labels.Requirements, error) { return nil, nil }

// mockUserInfo implements user.Info for testing
type mockUserInfo struct {
	name   string
	groups []string
}

func (m *mockUserInfo) GetName() string               { return m.name }
func (m *mockUserInfo) GetUID() string                { return "" }
func (m *mockUserInfo) GetGroups() []string           { return m.groups }
func (m *mockUserInfo) GetExtra() map[string][]string { return nil }

func TestEngine_AuthorizeNamespacedRequest(t *testing.T) {
	e := &Engine{
		directory: map[string]map[string]DirectoryEntry{
			"User": {
				"restricted-user": {
					LimitNamespaces: []*regexp.Regexp{
						regexp.MustCompile("^allowed-ns$"),
						regexp.MustCompile("^app-.*$"),
					},
					AllowAccessToSystemNamespaces: false,
					NamespaceFiltersAbsent:        false,
				},
				"system-user": {
					AllowAccessToSystemNamespaces: true,
					NamespaceFiltersAbsent:        true,
				},
			},
			"Group":          {},
			"ServiceAccount": {},
		},
	}

	tests := []struct {
		name             string
		userName         string
		namespace        string
		expectedDecision authorizer.Decision
	}{
		{
			name:             "allowed namespace",
			userName:         "restricted-user",
			namespace:        "allowed-ns",
			expectedDecision: authorizer.DecisionNoOpinion,
		},
		{
			name:             "allowed namespace with pattern",
			userName:         "restricted-user",
			namespace:        "app-frontend",
			expectedDecision: authorizer.DecisionNoOpinion,
		},
		{
			name:             "denied namespace",
			userName:         "restricted-user",
			namespace:        "other-ns",
			expectedDecision: authorizer.DecisionDeny,
		},
		{
			name:             "system user can access any",
			userName:         "system-user",
			namespace:        "kube-system",
			expectedDecision: authorizer.DecisionNoOpinion,
		},
		{
			name:             "unknown user - no restrictions",
			userName:         "unknown-user",
			namespace:        "any-ns",
			expectedDecision: authorizer.DecisionNoOpinion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := &mockAttrs{
				userInfo:   &mockUserInfo{name: tt.userName},
				namespace:  tt.namespace,
				resource:   "pods",
				verb:       "get",
				isResource: true,
			}

			decision, _, err := e.Authorize(context.Background(), attrs)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedDecision, decision, "unexpected decision for %s", tt.name)
		})
	}
}

func TestEngine_SystemNamespaceRestriction(t *testing.T) {
	e := &Engine{
		directory: map[string]map[string]DirectoryEntry{
			"User": {
				"no-system-access": {
					AllowAccessToSystemNamespaces: false,
					NamespaceFiltersAbsent:        true,
				},
				"with-system-access": {
					AllowAccessToSystemNamespaces: true,
					NamespaceFiltersAbsent:        true,
				},
			},
			"Group":          {},
			"ServiceAccount": {},
		},
	}

	tests := []struct {
		name             string
		userName         string
		namespace        string
		expectedDecision authorizer.Decision
	}{
		{
			name:             "denied kube-system",
			userName:         "no-system-access",
			namespace:        "kube-system",
			expectedDecision: authorizer.DecisionDeny,
		},
		{
			name:             "denied d8-system",
			userName:         "no-system-access",
			namespace:        "d8-system",
			expectedDecision: authorizer.DecisionDeny,
		},
		{
			name:             "allowed regular namespace",
			userName:         "no-system-access",
			namespace:        "my-app",
			expectedDecision: authorizer.DecisionNoOpinion,
		},
		{
			name:             "system access allowed kube-system",
			userName:         "with-system-access",
			namespace:        "kube-system",
			expectedDecision: authorizer.DecisionNoOpinion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := &mockAttrs{
				userInfo:   &mockUserInfo{name: tt.userName},
				namespace:  tt.namespace,
				resource:   "pods",
				verb:       "get",
				isResource: true,
			}

			decision, _, err := e.Authorize(context.Background(), attrs)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedDecision, decision)
		})
	}
}

func TestEngine_GroupBasedRestrictions(t *testing.T) {
	e := &Engine{
		directory: map[string]map[string]DirectoryEntry{
			"User": {},
			"Group": {
				"developers": {
					LimitNamespaces: []*regexp.Regexp{
						regexp.MustCompile("^dev-.*$"),
					},
					NamespaceFiltersAbsent: false,
				},
			},
			"ServiceAccount": {},
		},
	}

	tests := []struct {
		name             string
		groups           []string
		namespace        string
		expectedDecision authorizer.Decision
	}{
		{
			name:             "developer can access dev namespace",
			groups:           []string{"developers"},
			namespace:        "dev-frontend",
			expectedDecision: authorizer.DecisionNoOpinion,
		},
		{
			name:             "developer denied prod namespace",
			groups:           []string{"developers"},
			namespace:        "prod-backend",
			expectedDecision: authorizer.DecisionDeny,
		},
		{
			name:             "non-developer no restrictions",
			groups:           []string{"viewers"},
			namespace:        "any-ns",
			expectedDecision: authorizer.DecisionNoOpinion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := &mockAttrs{
				userInfo:   &mockUserInfo{name: "test-user", groups: tt.groups},
				namespace:  tt.namespace,
				resource:   "pods",
				verb:       "get",
				isResource: true,
			}

			decision, _, err := e.Authorize(context.Background(), attrs)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedDecision, decision)
		})
	}
}

func TestEngine_NonResourceRequest(t *testing.T) {
	e := &Engine{
		directory: map[string]map[string]DirectoryEntry{
			"User": {
				"restricted-user": {
					LimitNamespaces:        []*regexp.Regexp{regexp.MustCompile("^allowed$")},
					NamespaceFiltersAbsent: false,
				},
			},
			"Group":          {},
			"ServiceAccount": {},
		},
	}

	attrs := &mockAttrs{
		userInfo:   &mockUserInfo{name: "restricted-user"},
		path:       "/healthz",
		verb:       "get",
		isResource: false,
	}

	decision, _, err := e.Authorize(context.Background(), attrs)
	require.NoError(t, err)
	assert.Equal(t, authorizer.DecisionNoOpinion, decision, "non-resource requests should not be restricted")
}

func TestEngine_IsNamespaceAllowed(t *testing.T) {
	e := &Engine{
		directory: map[string]map[string]DirectoryEntry{
			"User": {
				"restricted-user": {
					LimitNamespaces: []*regexp.Regexp{
						regexp.MustCompile("^allowed-ns$"),
						regexp.MustCompile("^app-.*$"),
					},
					AllowAccessToSystemNamespaces: false,
					NamespaceFiltersAbsent:        false,
				},
				"system-user": {
					AllowAccessToSystemNamespaces: true,
					NamespaceFiltersAbsent:        true,
				},
				"unrestricted-user": {
					AllowAccessToSystemNamespaces: true,
					LimitNamespaces: []*regexp.Regexp{
						regexp.MustCompile("^.*$"),
					},
					NamespaceFiltersAbsent: false,
				},
			},
			"Group": {
				"developers": {
					LimitNamespaces: []*regexp.Regexp{
						regexp.MustCompile("^dev-.*$"),
					},
					NamespaceFiltersAbsent: false,
				},
			},
			"ServiceAccount": {},
		},
	}

	tests := []struct {
		name      string
		userInfo  *mockUserInfo
		namespace string
		expected  bool
	}{
		{
			name:      "restricted user - allowed namespace exact match",
			userInfo:  &mockUserInfo{name: "restricted-user"},
			namespace: "allowed-ns",
			expected:  true,
		},
		{
			name:      "restricted user - allowed namespace pattern match",
			userInfo:  &mockUserInfo{name: "restricted-user"},
			namespace: "app-frontend",
			expected:  true,
		},
		{
			name:      "restricted user - denied namespace",
			userInfo:  &mockUserInfo{name: "restricted-user"},
			namespace: "other-ns",
			expected:  false,
		},
		{
			name:      "restricted user - system namespace denied",
			userInfo:  &mockUserInfo{name: "restricted-user"},
			namespace: "kube-system",
			expected:  false,
		},
		{
			name:      "system user - system namespace allowed",
			userInfo:  &mockUserInfo{name: "system-user"},
			namespace: "kube-system",
			expected:  true,
		},
		{
			name:      "system user - any namespace allowed",
			userInfo:  &mockUserInfo{name: "system-user"},
			namespace: "random-ns",
			expected:  true,
		},
		{
			name:      "unrestricted user - all namespaces allowed",
			userInfo:  &mockUserInfo{name: "unrestricted-user"},
			namespace: "any-namespace",
			expected:  true,
		},
		{
			name:      "unrestricted user - system namespace allowed",
			userInfo:  &mockUserInfo{name: "unrestricted-user"},
			namespace: "kube-system",
			expected:  true,
		},
		{
			name:      "unknown user - no restrictions",
			userInfo:  &mockUserInfo{name: "unknown-user"},
			namespace: "any-ns",
			expected:  true,
		},
		{
			name:      "group member - allowed namespace",
			userInfo:  &mockUserInfo{name: "alice", groups: []string{"developers"}},
			namespace: "dev-frontend",
			expected:  true,
		},
		{
			name:      "group member - denied namespace",
			userInfo:  &mockUserInfo{name: "alice", groups: []string{"developers"}},
			namespace: "prod-backend",
			expected:  false,
		},
		{
			name:      "nil user - no restrictions",
			userInfo:  nil,
			namespace: "any-ns",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var userInfo user.Info
			if tt.userInfo != nil {
				userInfo = tt.userInfo
			}
			result := e.IsNamespaceAllowed(userInfo, tt.namespace)
			assert.Equal(t, tt.expected, result, "unexpected result for %s", tt.name)
		})
	}
}
