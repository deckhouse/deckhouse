/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import (
	"context"
	"os"
	"path/filepath"
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

// writeConfigJSON is a small helper for tests that exercise renewDirectories
// with hand-crafted JSON: it writes the supplied raw JSON into a temp file
// and returns the path. We do not reuse coverage_test.go's writeConfig because
// that helper requires a fully-typed UserAuthzConfig, which is awkward for
// table-driven cases that mix CARs and ARs.
func writeConfigJSON(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// TestEngine_RenewDirectories_AuthorizationRules verifies that AuthorizationRules
// (namespaced) are parsed from config.json and translated into per-namespace
// grants in the engine's directory. This is the regression test for the
// deny-by-default bug where AR-only users lost access to their namespaces.
func TestEngine_RenewDirectories_AuthorizationRules(t *testing.T) {
	tests := []struct {
		name             string
		config           string
		userInfo         *mockUserInfo
		namespace        string
		expectedDecision authorizer.Decision
		expectedAllowed  bool
	}{
		{
			name: "AR-only user gets access to AR's namespace",
			config: `{
				"crds": [],
				"ars": [
					{
						"name": "ar0",
						"namespace": "team-foo",
						"spec": {"subjects": [{"kind": "Group", "name": "developers"}]}
					}
				]
			}`,
			userInfo:         &mockUserInfo{name: "alice", groups: []string{"developers"}},
			namespace:        "team-foo",
			expectedDecision: authorizer.DecisionNoOpinion,
			expectedAllowed:  true,
		},
		{
			name: "AR-only user is still denied outside AR's namespace",
			config: `{
				"crds": [],
				"ars": [
					{
						"name": "ar0",
						"namespace": "team-foo",
						"spec": {"subjects": [{"kind": "Group", "name": "developers"}]}
					}
				]
			}`,
			userInfo:         &mockUserInfo{name: "alice", groups: []string{"developers"}},
			namespace:        "team-bar",
			expectedDecision: authorizer.DecisionDeny,
			expectedAllowed:  false,
		},
		{
			name: "AR with User subject grants namespace access",
			config: `{
				"crds": [],
				"ars": [
					{
						"name": "ar0",
						"namespace": "team-foo",
						"spec": {"subjects": [{"kind": "User", "name": "alice"}]}
					}
				]
			}`,
			userInfo:         &mockUserInfo{name: "alice"},
			namespace:        "team-foo",
			expectedAllowed:  true,
			expectedDecision: authorizer.DecisionNoOpinion,
		},
		{
			name: "AR with ServiceAccount subject defaults SA namespace to AR's namespace",
			config: `{
				"crds": [],
				"ars": [
					{
						"name": "ar0",
						"namespace": "team-foo",
						"spec": {"subjects": [{"kind": "ServiceAccount", "name": "my-sa"}]}
					}
				]
			}`,
			userInfo:         &mockUserInfo{name: "system:serviceaccount:team-foo:my-sa"},
			namespace:        "team-foo",
			expectedAllowed:  true,
			expectedDecision: authorizer.DecisionNoOpinion,
		},
		{
			name: "AR with explicit SA namespace is honoured",
			config: `{
				"crds": [],
				"ars": [
					{
						"name": "ar0",
						"namespace": "team-foo",
						"spec": {"subjects": [{"kind": "ServiceAccount", "name": "my-sa", "namespace": "other-ns"}]}
					}
				]
			}`,
			userInfo:         &mockUserInfo{name: "system:serviceaccount:other-ns:my-sa"},
			namespace:        "team-foo",
			expectedAllowed:  true,
			expectedDecision: authorizer.DecisionNoOpinion,
		},
		{
			name: "AR in a system namespace grants access only to that exact namespace",
			config: `{
				"crds": [],
				"ars": [
					{
						"name": "ar-sys",
						"namespace": "d8-monitoring",
						"spec": {"subjects": [{"kind": "User", "name": "alice"}]}
					}
				]
			}`,
			userInfo:         &mockUserInfo{name: "alice"},
			namespace:        "d8-monitoring",
			expectedAllowed:  true,
			expectedDecision: authorizer.DecisionNoOpinion,
		},
		{
			name: "AR + CAR: namespaces are unioned",
			config: `{
				"crds": [
					{
						"name": "car0",
						"spec": {
							"limitNamespaces": ["ns-a"],
							"subjects": [{"kind": "Group", "name": "developers"}]
						}
					}
				],
				"ars": [
					{
						"name": "ar0",
						"namespace": "ns-b",
						"spec": {"subjects": [{"kind": "Group", "name": "developers"}]}
					}
				]
			}`,
			userInfo:         &mockUserInfo{name: "alice", groups: []string{"developers"}},
			namespace:        "ns-b",
			expectedAllowed:  true,
			expectedDecision: authorizer.DecisionNoOpinion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Engine{
				configPath: writeConfigJSON(t, tt.config),
				directory:  map[string]map[string]DirectoryEntry{},
			}
			e.renewDirectories()

			assert.Equal(t, tt.expectedAllowed, e.IsNamespaceAllowed(tt.userInfo, tt.namespace),
				"IsNamespaceAllowed: namespace=%s", tt.namespace)

			attrs := &mockAttrs{
				userInfo:   tt.userInfo,
				namespace:  tt.namespace,
				resource:   "pods",
				verb:       "get",
				isResource: true,
			}
			decision, _, err := e.Authorize(context.Background(), attrs)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedDecision, decision,
				"Authorize: namespace=%s", tt.namespace)
		})
	}
}

// CAR: limitNamespaces = ["d8-.*", "team-.*"], allowAccessToSystemNamespaces = false
// AR:  namespace       = "d8-monitoring"
func TestEngine_RenewDirectories_AR_DoesNotLiftSystemGateForCAR(t *testing.T) {
	config := `{
		"crds": [
			{
				"name": "car-d8-wide",
				"spec": {
					"limitNamespaces": ["d8-.*", "team-.*"],
					"subjects": [{"kind": "Group", "name": "developers"}]
				}
			}
		],
		"ars": [
			{
				"name": "ar-monitoring",
				"namespace": "d8-monitoring",
				"spec": {"subjects": [{"kind": "Group", "name": "developers"}]}
			}
		]
	}`

	e := &Engine{
		configPath: writeConfigJSON(t, config),
		directory:  map[string]map[string]DirectoryEntry{},
	}
	e.renewDirectories()

	userInfo := &mockUserInfo{name: "alice", groups: []string{"developers"}}

	// CAR's legitimate non-system grant must keep working.
	assert.True(t, e.IsNamespaceAllowed(userInfo, "team-foo"),
		"non-system namespace matched by CAR must remain accessible")

	// AR explicitly punches a hole only for its own system namespace.
	assert.True(t, e.IsNamespaceAllowed(userInfo, "d8-monitoring"),
		"AR-granted system namespace must be allowed")

	// The priv-esc the reviewer flagged: every other d8-* (and any system NS)
	// matched by CAR's wildcard must stay behind the system gate.
	assert.False(t, e.IsNamespaceAllowed(userInfo, "d8-system"),
		"other d8-* system namespaces matched by CAR must NOT be unlocked by the AR")
	assert.False(t, e.IsNamespaceAllowed(userInfo, "kube-system"),
		"unrelated system namespaces must remain denied")
	assert.False(t, e.IsNamespaceAllowed(userInfo, "default"),
		"`default` is a system namespace and must remain denied")

	// Sanity check: namespaces matched by neither CAR nor AR are denied.
	assert.False(t, e.IsNamespaceAllowed(userInfo, "random-ns"),
		"namespaces outside CAR and AR scope must be denied")
}

// TestEngine_RenewDirectories_NamespaceRegexMetacharsAreEscaped is a
// defense-in-depth check: even if a string with regex metacharacters slipped
// through Kubernetes' DNS-1123 validation into config.json, the engine must
// match it literally rather than as a wildcard pattern.
func TestEngine_RenewDirectories_NamespaceRegexMetacharsAreEscaped(t *testing.T) {
	// "te.am" contains '.' which is a regex wildcard; the literal namespace
	// "teXam" must NOT be allowed by an AR scoped to "te.am".
	config := `{
		"crds": [],
		"ars": [
			{
				"name": "ar-meta",
				"namespace": "te.am",
				"spec": {"subjects": [{"kind": "User", "name": "alice"}]}
			}
		]
	}`

	e := &Engine{
		configPath: writeConfigJSON(t, config),
		directory:  map[string]map[string]DirectoryEntry{},
	}
	e.renewDirectories()

	userInfo := &mockUserInfo{name: "alice"}

	assert.True(t, e.IsNamespaceAllowed(userInfo, "te.am"),
		"literal namespace must still match")
	assert.False(t, e.IsNamespaceAllowed(userInfo, "teXam"),
		"regex metacharacters must be escaped: 'te.am' must not match 'teXam'")
	assert.False(t, e.IsNamespaceAllowed(userInfo, "team"),
		"regex metacharacters must be escaped: 'te.am' must not match 'team'")
}

// TestEngine_RenewDirectories_AROnlyUser_ResolverScenario simulates the resolver's
// path for the accessiblenamespaces API: the user has only ARs and the engine
// must classify them as FilteredAccess (not NoNamespacesAllowed) so that the
// AR-derived RoleBinding candidates pass through.
func TestEngine_RenewDirectories_AROnlyUser_ResolverScenario(t *testing.T) {
	config := `{
		"crds": [],
		"ars": [
			{
				"name": "ar0",
				"namespace": "team-foo",
				"spec": {"subjects": [{"kind": "Group", "name": "developers"}]}
			},
			{
				"name": "ar1",
				"namespace": "team-bar",
				"spec": {"subjects": [{"kind": "Group", "name": "developers"}]}
			}
		]
	}`

	e := &Engine{
		configPath: writeConfigJSON(t, config),
		directory:  map[string]map[string]DirectoryEntry{},
	}
	e.renewDirectories()

	userInfo := &mockUserInfo{name: "alice", groups: []string{"developers"}}

	accessType, filter := e.GetNamespaceAccessType(userInfo)
	require.Equal(t, FilteredAccess, accessType, "AR-only user must be FilteredAccess, not NoNamespacesAllowed")
	require.NotNil(t, filter, "filter must be returned for FilteredAccess")

	assert.True(t, e.IsNamespaceAllowedWithFilter("team-foo", filter), "AR namespace must be allowed")
	assert.True(t, e.IsNamespaceAllowedWithFilter("team-bar", filter), "second AR namespace must be allowed")
	assert.False(t, e.IsNamespaceAllowedWithFilter("team-baz", filter), "namespaces outside ARs must be denied")
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
		name                    string
		userInfo                *mockUserInfo
		expectedNamespaces      []string
		expectedHasRestrictions bool
	}{
		{
			name:                    "nil user - all allowed",
			userInfo:                nil,
			expectedNamespaces:      nil,
			expectedHasRestrictions: false,
		},
		{
			name:                    "system:masters without CAR - all allowed (privileged bypass)",
			userInfo:                &mockUserInfo{name: "admin", groups: []string{"system:masters"}},
			expectedNamespaces:      nil,
			expectedHasRestrictions: false,
		},
		{
			name:                    "kubeadm:cluster-admins without CAR - all allowed (privileged bypass)",
			userInfo:                &mockUserInfo{name: "kubeadm-admin", groups: []string{"kubeadm:cluster-admins"}},
			expectedNamespaces:      nil,
			expectedHasRestrictions: false,
		},
		{
			name:                    "unknown user without CAR - empty list (deny-by-default)",
			userInfo:                &mockUserInfo{name: "unknown-user", groups: []string{"system:authenticated"}},
			expectedNamespaces:      []string{},
			expectedHasRestrictions: true,
		},
		{
			name:                    "restricted user with CAR - has restrictions",
			userInfo:                &mockUserInfo{name: "restricted-user"},
			expectedNamespaces:      nil,
			expectedHasRestrictions: true,
		},
		{
			name:                    "unrestricted user with CAR (no filters) - all allowed",
			userInfo:                &mockUserInfo{name: "unrestricted-user"},
			expectedNamespaces:      nil,
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
