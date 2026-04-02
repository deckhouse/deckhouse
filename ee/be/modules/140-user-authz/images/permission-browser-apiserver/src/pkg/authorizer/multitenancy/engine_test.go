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
		groups           []string
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
			name:             "unknown user without CAR - NoOpinion (defers to RBAC)",
			userName:         "unknown-user",
			namespace:        "any-ns",
			expectedDecision: authorizer.DecisionNoOpinion,
		},
		{
			name:             "superadmins group without CAR - NoOpinion (defers to RBAC)",
			userName:         "super-admin",
			groups:           []string{"superadmins"},
			namespace:        "any-ns",
			expectedDecision: authorizer.DecisionNoOpinion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := &mockAttrs{
				userInfo:   &mockUserInfo{name: tt.userName, groups: tt.groups},
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
			name:      "unknown user without CAR - denied (deny-by-default)",
			userInfo:  &mockUserInfo{name: "unknown-user"},
			namespace: "any-ns",
			expected:  false,
		},
		{
			name:      "system:masters user without CAR - allowed (privileged bypass)",
			userInfo:  &mockUserInfo{name: "admin", groups: []string{"system:masters"}},
			namespace: "any-ns",
			expected:  true,
		},
		{
			name:      "system:masters user without CAR - system namespace allowed",
			userInfo:  &mockUserInfo{name: "admin", groups: []string{"system:masters"}},
			namespace: "kube-system",
			expected:  true,
		},
		{
			name:      "kubeadm:cluster-admins user without CAR - allowed (privileged bypass)",
			userInfo:  &mockUserInfo{name: "kubeadm-admin", groups: []string{"kubeadm:cluster-admins"}},
			namespace: "any-ns",
			expected:  true,
		},
		{
			name:      "superadmins user without CAR - allowed (privileged bypass)",
			userInfo:  &mockUserInfo{name: "super-admin", groups: []string{"superadmins"}},
			namespace: "any-ns",
			expected:  true,
		},
		{
			name:      "regular authenticated user without CAR - denied",
			userInfo:  &mockUserInfo{name: "random-user", groups: []string{"system:authenticated"}},
			namespace: "any-ns",
			expected:  false,
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

func TestIsPrivilegedUser(t *testing.T) {
	tests := []struct {
		name     string
		groups   []string
		expected bool
	}{
		{
			name:     "system:masters is privileged",
			groups:   []string{"system:masters"},
			expected: true,
		},
		{
			name:     "kubeadm:cluster-admins is privileged",
			groups:   []string{"kubeadm:cluster-admins"},
			expected: true,
		},
		{
			name:     "superadmins is privileged",
			groups:   []string{"superadmins"},
			expected: true,
		},
		{
			name:     "system:authenticated is not privileged",
			groups:   []string{"system:authenticated"},
			expected: false,
		},
		{
			name:     "random group is not privileged",
			groups:   []string{"developers", "viewers"},
			expected: false,
		},
		{
			name:     "mixed groups with one privileged",
			groups:   []string{"system:authenticated", "system:masters", "developers"},
			expected: true,
		},
		{
			name:     "empty groups",
			groups:   []string{},
			expected: false,
		},
		{
			name:     "nil groups",
			groups:   nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPrivilegedUser(tt.groups)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEngine_GetNamespaceAccessType(t *testing.T) {
	e := &Engine{
		directory: map[string]map[string]DirectoryEntry{
			"User": {
				"restricted-user": {
					LimitNamespaces: []*regexp.Regexp{
						regexp.MustCompile("^allowed-ns$"),
					},
					NamespaceFiltersAbsent: false,
				},
				"unrestricted-user": {
					AllowAccessToSystemNamespaces: true,
					NamespaceFiltersAbsent:        true,
				},
			},
			"Group":          {},
			"ServiceAccount": {},
		},
	}

	tests := []struct {
		name               string
		userInfo           *mockUserInfo
		expectedAccessType NamespaceAccessType
		expectFilter       bool
	}{
		{
			name:               "nil user - all allowed",
			userInfo:           nil,
			expectedAccessType: AllNamespacesAllowed,
			expectFilter:       false,
		},
		{
			name:               "system:masters without CAR - all allowed (privileged bypass)",
			userInfo:           &mockUserInfo{name: "admin", groups: []string{"system:masters"}},
			expectedAccessType: AllNamespacesAllowed,
			expectFilter:       false,
		},
		{
			name:               "superadmins without CAR - all allowed (privileged bypass)",
			userInfo:           &mockUserInfo{name: "super", groups: []string{"superadmins"}},
			expectedAccessType: AllNamespacesAllowed,
			expectFilter:       false,
		},
		{
			name:               "unknown user without CAR - denied (deny-by-default)",
			userInfo:           &mockUserInfo{name: "unknown-user", groups: []string{"system:authenticated"}},
			expectedAccessType: NoNamespacesAllowed,
			expectFilter:       false,
		},
		{
			name:               "restricted user with CAR - filtered access",
			userInfo:           &mockUserInfo{name: "restricted-user"},
			expectedAccessType: FilteredAccess,
			expectFilter:       true,
		},
		{
			name:               "unrestricted user with CAR (no filters) - all allowed",
			userInfo:           &mockUserInfo{name: "unrestricted-user"},
			expectedAccessType: AllNamespacesAllowed,
			expectFilter:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var userInfo user.Info
			if tt.userInfo != nil {
				userInfo = tt.userInfo
			}
			accessType, filter := e.GetNamespaceAccessType(userInfo)
			assert.Equal(t, tt.expectedAccessType, accessType, "unexpected accessType for %s", tt.name)
			if tt.expectFilter {
				assert.NotNil(t, filter, "expected filter for %s", tt.name)
			} else {
				assert.Nil(t, filter, "expected no filter for %s", tt.name)
			}
		})
	}
}

func TestEngine_GetAllowedNamespaces(t *testing.T) {
	e := &Engine{
		directory: map[string]map[string]DirectoryEntry{
			"User": {
				"restricted-user": {
					LimitNamespaces: []*regexp.Regexp{
						regexp.MustCompile("^allowed-ns$"),
					},
					NamespaceFiltersAbsent: false,
				},
				"unrestricted-user": {
					AllowAccessToSystemNamespaces: true,
					NamespaceFiltersAbsent:        true,
				},
			},
			"Group":          {},
			"ServiceAccount": {},
		},
	}

	tests := []struct {
		name               string
		userInfo           *mockUserInfo
		expectedNamespaces []string
		expectedHasRestrictions bool
	}{
		{
			name:               "nil user - all allowed",
			userInfo:           nil,
			expectedNamespaces: nil,
			expectedHasRestrictions: false,
		},
		{
			name:               "system:masters without CAR - all allowed (privileged bypass)",
			userInfo:           &mockUserInfo{name: "admin", groups: []string{"system:masters"}},
			expectedNamespaces: nil,
			expectedHasRestrictions: false,
		},
		{
			name:               "kubeadm:cluster-admins without CAR - all allowed (privileged bypass)",
			userInfo:           &mockUserInfo{name: "kubeadm-admin", groups: []string{"kubeadm:cluster-admins"}},
			expectedNamespaces: nil,
			expectedHasRestrictions: false,
		},
		{
			name:               "unknown user without CAR - empty list (deny-by-default)",
			userInfo:           &mockUserInfo{name: "unknown-user", groups: []string{"system:authenticated"}},
			expectedNamespaces: []string{},
			expectedHasRestrictions: true,
		},
		{
			name:               "restricted user with CAR - has restrictions",
			userInfo:           &mockUserInfo{name: "restricted-user"},
			expectedNamespaces: nil,
			expectedHasRestrictions: true,
		},
		{
			name:               "unrestricted user with CAR (no filters) - all allowed",
			userInfo:           &mockUserInfo{name: "unrestricted-user"},
			expectedNamespaces: nil,
			expectedHasRestrictions: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var userInfo user.Info
			if tt.userInfo != nil {
				userInfo = tt.userInfo
			}
			namespaces, hasRestrictions := e.GetAllowedNamespaces(userInfo)
			assert.Equal(t, tt.expectedNamespaces, namespaces, "unexpected namespaces for %s", tt.name)
			assert.Equal(t, tt.expectedHasRestrictions, hasRestrictions, "unexpected hasRestrictions for %s", tt.name)
		})
	}
}
