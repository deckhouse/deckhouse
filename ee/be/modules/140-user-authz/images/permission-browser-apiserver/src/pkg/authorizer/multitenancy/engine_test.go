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

// fakeIndependentChecker simulates the CAR-independent RBAC check: it grants
// requests only in the listed namespaces (e.g. as an AR's RoleBinding or a
// plain RoleBinding would).
type fakeIndependentChecker struct {
	grantedNamespaces map[string]struct{}
}

func newFakeIndependentChecker(namespaces ...string) *fakeIndependentChecker {
	granted := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		granted[ns] = struct{}{}
	}
	return &fakeIndependentChecker{grantedNamespaces: granted}
}

func (f *fakeIndependentChecker) AllowsIndependently(_ context.Context, attrs authorizer.Attributes) bool {
	if attrs.GetNamespace() == "" {
		return false
	}
	_, ok := f.grantedNamespaces[attrs.GetNamespace()]
	return ok
}

// TestEngine_Authorize_IndependentRBACWithCAR verifies that for a user WITH a
// namespace-limited CAR, requests outside the CAR scope are denied UNLESS
// CAR-independent RBAC (a RoleBinding in the namespace, e.g. one created by an
// AuthorizationRule, or a non-CAR ClusterRoleBinding) grants them. This is the
// union semantics: the CAR limit must not shadow independent RBAC grants, and
// conversely the CAR's cluster-wide accessLevel must not leak into namespaces
// where only an independent grant exists.
func TestEngine_Authorize_IndependentRBACWithCAR(t *testing.T) {
	config := `{
		"crds": [
			{
				"name": "car0",
				"spec": {
					"limitNamespaces": ["ns-a"],
					"subjects": [{"kind": "User", "name": "alice"}]
				}
			}
		]
	}`

	e := &Engine{
		configPath: writeConfigJSON(t, config),
		directory:  map[string]map[string]DirectoryEntry{},
	}
	e.SetIndependentRBACChecker(newFakeIndependentChecker("ns-b"))
	e.renewDirectories()

	tests := []struct {
		name             string
		namespace        string
		expectedDecision authorizer.Decision
	}{
		{
			name:             "namespace inside CAR limit is NoOpinion (RBAC decides)",
			namespace:        "ns-a",
			expectedDecision: authorizer.DecisionNoOpinion,
		},
		{
			name:             "namespace outside CAR limit with an independent grant is NoOpinion",
			namespace:        "ns-b",
			expectedDecision: authorizer.DecisionNoOpinion,
		},
		{
			name:             "namespace outside CAR limit without any independent grant is denied",
			namespace:        "ns-c",
			expectedDecision: authorizer.DecisionDeny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := &mockAttrs{
				userInfo:   &mockUserInfo{name: "alice"},
				namespace:  tt.namespace,
				resource:   "pods",
				verb:       "get",
				isResource: true,
			}
			decision, _, err := e.Authorize(context.Background(), attrs)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedDecision, decision, "namespace=%s", tt.namespace)
		})
	}
}

// TestEngine_RenewDirectories_IgnoresAuthorizationRules verifies that the engine
// builds its directory from ClusterAuthorizationRules (CARs) ONLY and ignores
// namespaced AuthorizationRules ("ars") entirely, mirroring the real
// kube-apiserver user-authz webhook authorizer (images/webhook), whose config
// parses "crds" and never "ars".
//
// Because the engine is deny-only, ignoring ARs means an AR-only user:
//   - gets NO directory entry (so the engine never treats the AR as a deny-filter),
//   - has Authorize return NoOpinion (the engine defers to RBAC, which honors the
//     RoleBinding each AR creates),
//   - is classified NoNamespacesAllowed for discovery filtering (no CAR), so the
//     AccessibleNamespaces list comes from the RBAC candidate path, not the engine.
func TestEngine_RenewDirectories_IgnoresAuthorizationRules(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		userInfo  *mockUserInfo
		namespace string
	}{
		{
			name: "AR with Group subject is ignored",
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
			userInfo:  &mockUserInfo{name: "alice", groups: []string{"developers"}},
			namespace: "team-foo",
		},
		{
			name: "AR with User subject is ignored",
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
			userInfo:  &mockUserInfo{name: "alice"},
			namespace: "team-foo",
		},
		{
			name: "AR with ServiceAccount subject is ignored",
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
			userInfo:  &mockUserInfo{name: "system:serviceaccount:team-foo:my-sa"},
			namespace: "team-foo",
		},
		{
			name: "AR in a system namespace is ignored",
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
			userInfo:  &mockUserInfo{name: "alice"},
			namespace: "d8-monitoring",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Engine{
				configPath: writeConfigJSON(t, tt.config),
				directory:  map[string]map[string]DirectoryEntry{},
			}
			e.renewDirectories()

			// The AR must not create any directory entry for the subject.
			assert.Empty(t, e.affectedDirs(tt.userInfo.GetName(), tt.userInfo.GetGroups()),
				"AR must not produce a directory entry (CAR-only directory)")

			// Authorize must defer to RBAC (NoOpinion), not treat the AR as a deny-filter.
			for _, ns := range []string{tt.namespace, "some-other-ns"} {
				attrs := &mockAttrs{
					userInfo:   tt.userInfo,
					namespace:  ns,
					resource:   "pods",
					verb:       "get",
					isResource: true,
				}
				decision, _, err := e.Authorize(context.Background(), attrs)
				require.NoError(t, err)
				assert.Equal(t, authorizer.DecisionNoOpinion, decision,
					"Authorize must be NoOpinion for AR-only user (namespace=%s)", ns)
			}

			// For discovery filtering, a non-privileged AR-only user (no CAR) is
			// NoNamespacesAllowed: the namespaces come from RBAC candidates, not the engine.
			accessType, filter := e.GetNamespaceAccessType(tt.userInfo)
			assert.Equal(t, NoNamespacesAllowed, accessType,
				"AR-only user must be NoNamespacesAllowed (engine ignores ARs)")
			assert.Nil(t, filter)
		})
	}
}

// TestEngine_RenewDirectories_CARSystemGateIgnoresARs verifies the directory is
// CAR-only: a CAR with a wildcard limitNamespaces keeps gating system namespaces,
// and a co-located AR for the same subject changes nothing. Previously the AR was
// translated into a literal LimitNamespaces grant that punched a hole in the system
// gate for its own namespace; the engine now ignores it, matching the webhook.
//
// CAR: limitNamespaces = ["d8-.*", "team-.*"], allowAccessToSystemNamespaces = false
// AR:  namespace       = "d8-monitoring" (ignored)
func TestEngine_RenewDirectories_CARSystemGateIgnoresARs(t *testing.T) {
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

	// The AR is ignored: it must NOT unlock its own (system) namespace. This mirrors
	// the webhook, which would deny a namespaced request to d8-monitoring under this CAR.
	assert.False(t, e.IsNamespaceAllowed(userInfo, "d8-monitoring"),
		"AR must not unlock a system namespace; the engine ignores ARs")
	assert.False(t, e.IsNamespaceAllowed(userInfo, "d8-system"),
		"other d8-* system namespaces matched by CAR must stay behind the system gate")
	assert.False(t, e.IsNamespaceAllowed(userInfo, "kube-system"),
		"unrelated system namespaces must remain denied")
	assert.False(t, e.IsNamespaceAllowed(userInfo, "default"),
		"`default` is a system namespace and must remain denied")

	// Sanity check: namespaces matched by neither CAR nor AR are denied.
	assert.False(t, e.IsNamespaceAllowed(userInfo, "random-ns"),
		"namespaces outside CAR scope must be denied")
}

// TestEngine_RenewDirectories_CAROnlyUser_ResolverScenario covers the resolver's
// path for the accessiblenamespaces API: a user with a CAR that limits namespaces
// is classified FilteredAccess, and the returned filter honors only the CAR's
// namespaces (ARs are ignored).
func TestEngine_RenewDirectories_CAROnlyUser_ResolverScenario(t *testing.T) {
	config := `{
		"crds": [
			{
				"name": "car0",
				"spec": {
					"limitNamespaces": ["team-foo", "team-bar"],
					"subjects": [{"kind": "Group", "name": "developers"}]
				}
			}
		],
		"ars": [
			{
				"name": "ar0",
				"namespace": "team-baz",
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
	require.Equal(t, FilteredAccess, accessType, "user with a namespace-limited CAR must be FilteredAccess")
	require.NotNil(t, filter, "filter must be returned for FilteredAccess")

	assert.True(t, e.IsNamespaceAllowedWithFilter("team-foo", filter), "CAR namespace must be allowed")
	assert.True(t, e.IsNamespaceAllowedWithFilter("team-bar", filter), "second CAR namespace must be allowed")
	assert.False(t, e.IsNamespaceAllowedWithFilter("team-baz", filter),
		"AR-only namespace must NOT be in the engine filter; the engine ignores ARs")
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
