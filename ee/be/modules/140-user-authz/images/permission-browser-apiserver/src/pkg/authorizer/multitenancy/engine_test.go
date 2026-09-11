/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/binding"
	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"

	"permission-browser-apiserver/pkg/authorizer/multitenancy/mttest"
)

// Ensure mockUserInfo implements user.Info for namespace-access tests
var _ user.Info = &mockUserInfo{}

// nsAllowed mirrors how the namespace resolver consumes the engine:
// classify the user via GetNamespaceAccessType, then apply the returned
// filter. This is the only supported per-namespace check path.
func nsAllowed(e *Engine, userInfo user.Info, namespace string) bool {
	accessType, filter := e.GetNamespaceAccessType(userInfo)
	switch accessType {
	case AllNamespacesAllowed:
		return true
	case NoNamespacesAllowed:
		return false
	default:
		return e.IsNamespaceAllowedWithFilter(namespace, filter)
	}
}

// swappableRules is a RulesProvider whose directory can be replaced while the engine serves, the
// way the informer-backed source replaces it on every ClusterAuthorizationRule change. A nil
// directory stands for a source that has not listed the rules yet.
type swappableRules struct {
	dir atomic.Pointer[rules.Directory]
}

func newSwappableRules(rs ...rules.Rule) *swappableRules {
	s := &swappableRules{}
	s.Set(rs...)
	return s
}

// Set replaces the directory with one built from the rules.
func (s *swappableRules) Set(rs ...rules.Rule) {
	s.dir.Store(mttest.Rules(rs...).Directory())
}

func (s *swappableRules) Directory() *rules.Directory { return s.dir.Load() }
func (s *swappableRules) HasSynced() bool             { return s.dir.Load() != nil }

// ruleBinding is a ClusterRoleBinding as user-authz-controller creates it for a rule: the
// user-authz:<rule>:<postfix> name and the module labels make it a rule binding.
func ruleBinding(name string, subjects ...rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				binding.LabelHeritage:  binding.HeritageValue,
				binding.LabelModule:    binding.ModuleName,
				binding.LabelManagedBy: binding.ManagedByValue,
			},
		},
		Subjects: subjects,
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
		rules: mttest.Rules(
			rules.Rule{
				Name:            "restricted-user",
				Subjects:        []rules.Subject{{Kind: "User", Name: "restricted-user"}},
				LimitNamespaces: []string{"allowed-ns", "app-.*"},
			},
			rules.Rule{
				Name:                          "system-user",
				Subjects:                      []rules.Subject{{Kind: "User", Name: "system-user"}},
				AllowAccessToSystemNamespaces: true,
			},
		),
		bindings: mttest.NoBindings(),
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
		rules: mttest.Rules(
			rules.Rule{
				Name:     "no-system-access",
				Subjects: []rules.Subject{{Kind: "User", Name: "no-system-access"}},
			},
			rules.Rule{
				Name:                          "with-system-access",
				Subjects:                      []rules.Subject{{Kind: "User", Name: "with-system-access"}},
				AllowAccessToSystemNamespaces: true,
			},
		),
		bindings: mttest.NoBindings(),
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
		rules: mttest.Rules(
			rules.Rule{
				Name:            "developers",
				Subjects:        []rules.Subject{{Kind: "Group", Name: "developers"}},
				LimitNamespaces: []string{"dev-.*"},
			},
		),
		bindings: mttest.NoBindings(),
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
		rules: mttest.Rules(
			rules.Rule{
				Name:            "restricted-user",
				Subjects:        []rules.Subject{{Kind: "User", Name: "restricted-user"}},
				LimitNamespaces: []string{"allowed"},
			},
		),
		bindings: mttest.NoBindings(),
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

func TestEngine_NamespaceAccessFiltering(t *testing.T) {
	e := &Engine{
		rules: mttest.Rules(
			rules.Rule{
				Name:            "restricted-user",
				Subjects:        []rules.Subject{{Kind: "User", Name: "restricted-user"}},
				LimitNamespaces: []string{"allowed-ns", "app-.*"},
			},
			rules.Rule{
				Name:                          "system-user",
				Subjects:                      []rules.Subject{{Kind: "User", Name: "system-user"}},
				AllowAccessToSystemNamespaces: true,
			},
			rules.Rule{
				Name:                          "unrestricted-user",
				Subjects:                      []rules.Subject{{Kind: "User", Name: "unrestricted-user"}},
				LimitNamespaces:               []string{".*"},
				AllowAccessToSystemNamespaces: true,
			},
			rules.Rule{
				Name:            "developers",
				Subjects:        []rules.Subject{{Kind: "Group", Name: "developers"}},
				LimitNamespaces: []string{"dev-.*"},
			},
		),
		bindings: mttest.NoBindings(),
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
			result := nsAllowed(e, userInfo, tt.namespace)
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
		rules: mttest.Rules(
			rules.Rule{
				Name:            "restricted-user",
				Subjects:        []rules.Subject{{Kind: "User", Name: "restricted-user"}},
				LimitNamespaces: []string{"allowed-ns"},
			},
			rules.Rule{
				Name:                          "unrestricted-user",
				Subjects:                      []rules.Subject{{Kind: "User", Name: "unrestricted-user"}},
				AllowAccessToSystemNamespaces: true,
			},
		),
		bindings: mttest.NoBindings(),
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

// TestEngine_InvalidRegexQuarantinesOnlyItsRule locks that a limitNamespaces
// pattern that does not compile costs only the rule that carries it: the
// sibling rule keeps its scope, and the subject of the broken rule gets a
// maximally restricted entry rather than no entry (which would hand it to
// RBAC unfiltered) or a wider one.
func TestEngine_InvalidRegexQuarantinesOnlyItsRule(t *testing.T) {
	e, err := NewEngine(mttest.LegacyJSON(t, `{
		"crds": [
			{
				"name": "broken",
				"spec": {
					"limitNamespaces": ["["],
					"subjects": [{"kind": "User", "name": "bob"}]
				}
			},
			{
				"name": "healthy",
				"spec": {
					"limitNamespaces": ["team-a"],
					"subjects": [{"kind": "User", "name": "alice"}]
				}
			}
		]
	}`), mttest.NoBindings(), nil, nil, nil)
	require.NoError(t, err)

	assert.Len(t, e.affectedEntries("alice", nil), 1, "a healthy CAR must survive a broken sibling")

	entries := e.affectedEntries("bob", nil)
	require.Len(t, entries, 1, "the subject of the broken CAR still gets an entry")
	assert.True(t, entries[0].Quarantined)
	assert.Empty(t, entries[0].LimitNamespaces, "the broken pattern is left out, not widened")
	assert.False(t, entries[0].NamespaceFiltersAbsent)

	for _, ns := range []string{"team-a", "any-ns"} {
		decision, reason, err := e.Authorize(context.Background(), &mockAttrs{
			userInfo:   &mockUserInfo{name: "bob"},
			verb:       "get",
			resource:   "pods",
			namespace:  ns,
			isResource: true,
		})
		require.NoError(t, err)
		assert.Equal(t, authorizer.DecisionDeny, decision, "bob must be denied in %s", ns)
		assert.Equal(t, noNamespaceAccessReason, reason)
	}

	accessType, filter := e.GetNamespaceAccessType(&mockUserInfo{name: "bob"})
	assert.Equal(t, FilteredAccess, accessType)
	require.NotNil(t, filter)
	assert.False(t, e.IsNamespaceAllowedWithFilter("team-a", filter), "a quarantined rule opens no namespace")
}

// TestEngine_MalformedSelectorStaysLocal locks that an uncompilable
// namespaceSelector costs only the rule that carries it. One bad CAR must not
// turn into a cluster-wide loss of multi-tenancy: the library quarantines the
// rule and keeps serving the others.
func TestEngine_MalformedSelectorStaysLocal(t *testing.T) {
	e, err := NewEngine(mttest.LegacyJSON(t, `{
		"crds": [
			{
				"name": "broken",
				"spec": {
					"namespaceSelector": {"labelSelector": {"matchExpressions": [{"key": "team", "operator": "Exists", "values": ["nope"]}]}},
					"subjects": [{"kind": "User", "name": "bob"}]
				}
			},
			{
				"name": "healthy",
				"spec": {
					"limitNamespaces": ["team-a"],
					"subjects": [{"kind": "User", "name": "alice"}]
				}
			}
		]
	}`), mttest.NoBindings(), nil, nil, nil)
	require.NoError(t, err)

	require.Len(t, e.affectedEntries("alice", nil), 1, "a healthy CAR must survive a broken sibling")

	entries := e.affectedEntries("bob", nil)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Quarantined)
	require.Len(t, entries[0].NamespaceSelectors, 1)
	assert.Nil(t, entries[0].NamespaceSelectors[0].Labels,
		"the malformed selector is left uncompiled, not dropped")
	assert.False(t, entries[0].NamespaceSelectors[0].MatchAny)
}

// TestEngine_Authorize_MalformedSelectorDeniesOnlyItsSubject is the Authorize
// view of the same config: alice keeps her filter and bob, whose only rule
// cannot be evaluated, is denied rather than waved through.
func TestEngine_Authorize_MalformedSelectorDeniesOnlyItsSubject(t *testing.T) {
	e, err := NewEngine(mttest.LegacyJSON(t, `{
		"crds": [
			{
				"name": "broken",
				"spec": {
					"namespaceSelector": {"labelSelector": {"matchExpressions": [{"key": "team", "operator": "Exists", "values": ["nope"]}]}},
					"subjects": [{"kind": "User", "name": "bob"}]
				}
			},
			{
				"name": "healthy",
				"spec": {
					"limitNamespaces": ["team-a"],
					"subjects": [{"kind": "User", "name": "alice"}]
				}
			}
		]
	}`), mttest.NoBindings(), nil, nil, nil)
	require.NoError(t, err)

	tests := []struct {
		name string
		user string
		ns   string
		want authorizer.Decision
	}{
		{name: "alice inside her limitNamespaces", user: "alice", ns: "team-a", want: authorizer.DecisionNoOpinion},
		{name: "alice outside her limitNamespaces", user: "alice", ns: "team-b", want: authorizer.DecisionDeny},
		{name: "bob under the malformed selector", user: "bob", ns: "team-a", want: authorizer.DecisionDeny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, _, err := e.Authorize(context.Background(), &mockAttrs{
				userInfo:   &mockUserInfo{name: tt.user},
				verb:       "get",
				resource:   "pods",
				namespace:  tt.ns,
				isResource: true,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.want, decision)
		})
	}
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

	e, err := NewEngine(mttest.LegacyJSON(t, config), mttest.NoBindings(), nil, nil, nil)
	require.NoError(t, err)
	e.SetIndependentRBACChecker(newFakeIndependentChecker("ns-b"))

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

// TestEngine_IgnoresAuthorizationRules verifies that the engine builds its
// directory from ClusterAuthorizationRules (CARs) ONLY and ignores namespaced
// AuthorizationRules ("ars") entirely, mirroring the real kube-apiserver
// user-authz webhook authorizer (images/webhook), whose config parses "crds"
// and never "ars".
//
// Because the engine is deny-only, ignoring ARs means an AR-only user:
//   - gets NO directory entry (so the engine never treats the AR as a deny-filter),
//   - has Authorize return NoOpinion (the engine defers to RBAC, which honors the
//     RoleBinding each AR creates),
//   - is classified NoNamespacesAllowed for discovery filtering (no CAR), so the
//     AccessibleNamespaces list comes from the RBAC candidate path, not the engine.
func TestEngine_IgnoresAuthorizationRules(t *testing.T) {
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
			e, err := NewEngine(mttest.LegacyJSON(t, tt.config), mttest.NoBindings(), nil, nil, nil)
			require.NoError(t, err)

			// The AR must not create any directory entry for the subject.
			assert.Empty(t, e.affectedEntries(tt.userInfo.GetName(), tt.userInfo.GetGroups()),
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

// TestEngine_CARSystemGateIgnoresARs verifies the directory is CAR-only: a CAR
// with a wildcard limitNamespaces keeps gating system namespaces, and a
// co-located AR for the same subject changes nothing. Previously the AR was
// translated into a literal LimitNamespaces grant that punched a hole in the
// system gate for its own namespace; the engine now ignores it, matching the
// webhook.
//
// CAR: limitNamespaces = ["d8-.*", "team-.*"], allowAccessToSystemNamespaces = false
// AR:  namespace       = "d8-monitoring" (ignored)
func TestEngine_CARSystemGateIgnoresARs(t *testing.T) {
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

	e, err := NewEngine(mttest.LegacyJSON(t, config), mttest.NoBindings(), nil, nil, nil)
	require.NoError(t, err)

	userInfo := &mockUserInfo{name: "alice", groups: []string{"developers"}}

	// CAR's legitimate non-system grant must keep working.
	assert.True(t, nsAllowed(e, userInfo, "team-foo"),
		"non-system namespace matched by CAR must remain accessible")

	// The AR is ignored: it must NOT unlock its own (system) namespace. This mirrors
	// the webhook, which would deny a namespaced request to d8-monitoring under this CAR.
	assert.False(t, nsAllowed(e, userInfo, "d8-monitoring"),
		"AR must not unlock a system namespace; the engine ignores ARs")
	assert.False(t, nsAllowed(e, userInfo, "d8-system"),
		"other d8-* system namespaces matched by CAR must stay behind the system gate")
	assert.False(t, nsAllowed(e, userInfo, "kube-system"),
		"unrelated system namespaces must remain denied")
	assert.False(t, nsAllowed(e, userInfo, "default"),
		"`default` is a system namespace and must remain denied")

	// Sanity check: namespaces matched by neither CAR nor AR are denied.
	assert.False(t, nsAllowed(e, userInfo, "random-ns"),
		"namespaces outside CAR scope must be denied")
}

// TestEngine_CAROnlyUser_ResolverScenario covers the resolver's path for the
// accessiblenamespaces API: a user with a CAR that limits namespaces is
// classified FilteredAccess, and the returned filter honors only the CAR's
// namespaces (ARs are ignored).
func TestEngine_CAROnlyUser_ResolverScenario(t *testing.T) {
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

	e, err := NewEngine(mttest.LegacyJSON(t, config), mttest.NoBindings(), nil, nil, nil)
	require.NoError(t, err)

	userInfo := &mockUserInfo{name: "alice", groups: []string{"developers"}}

	accessType, filter := e.GetNamespaceAccessType(userInfo)
	require.Equal(t, FilteredAccess, accessType, "user with a namespace-limited CAR must be FilteredAccess")
	require.NotNil(t, filter, "filter must be returned for FilteredAccess")

	assert.True(t, e.IsNamespaceAllowedWithFilter("team-foo", filter), "CAR namespace must be allowed")
	assert.True(t, e.IsNamespaceAllowedWithFilter("team-bar", filter), "second CAR namespace must be allowed")
	assert.False(t, e.IsNamespaceAllowedWithFilter("team-baz", filter),
		"AR-only namespace must NOT be in the engine filter; the engine ignores ARs")
}

// TestEngine_OrderingGuard_BindingBeforeRule locks the ordering guard:
// user-authz-controller creates the ClusterRoleBindings of a rule seconds after
// the rule, and the two reach this apiserver over independent watches. A
// subject bound by a rule binding whose rule the directory does not know yet is
// treated as maximally restricted, so the report never shows more than the
// webhook will allow. Once the rule arrives, its own scope applies, and a
// subject nobody binds is never touched by the guard.
func TestEngine_OrderingGuard_BindingBeforeRule(t *testing.T) {
	bindings := binding.NewIndex()
	bindings.Upsert(ruleBinding("user-authz:late:admin",
		rbacv1.Subject{Kind: rbacv1.UserKind, Name: "late-user"}))

	lateUser := &mockUserInfo{name: "late-user"}
	nobody := &mockUserInfo{name: "nobody"}

	assertRestricted := func(t *testing.T, e *Engine) {
		t.Helper()
		decision, reason, err := e.Authorize(context.Background(), &mockAttrs{
			userInfo:   lateUser,
			verb:       "get",
			resource:   "pods",
			namespace:  "team-a",
			isResource: true,
		})
		require.NoError(t, err)
		assert.Equal(t, authorizer.DecisionDeny, decision, "a namespaced request is denied until the rule arrives")
		assert.Equal(t, rules.NoNamespaceAccessReason, reason)

		decision, reason, err = e.Authorize(context.Background(), &mockAttrs{
			userInfo:   lateUser,
			verb:       "list",
			resource:   "pods",
			isResource: true,
		})
		require.NoError(t, err)
		assert.Equal(t, authorizer.DecisionDeny, decision, "a cluster-scoped request is denied until the rule arrives")
		assert.Equal(t, rules.NamespaceLimitedAccessReason, reason)

		accessType, filter := e.GetNamespaceAccessType(lateUser)
		assert.Equal(t, FilteredAccess, accessType)
		require.NotNil(t, filter)
		for _, ns := range []string{"team-a", "team-b", "kube-system"} {
			assert.False(t, e.IsNamespaceAllowedWithFilter(ns, filter), "the restricted entry opens no namespace (%s)", ns)
		}

		decision, _, err = e.Authorize(context.Background(), &mockAttrs{
			userInfo:   nobody,
			verb:       "get",
			resource:   "pods",
			namespace:  "team-a",
			isResource: true,
		})
		require.NoError(t, err)
		assert.Equal(t, authorizer.DecisionNoOpinion, decision, "a subject without bindings is left to RBAC")
	}

	t.Run("rules synced but the rule is not in the directory yet", func(t *testing.T) {
		provider := newSwappableRules()
		e := &Engine{rules: provider, bindings: bindings, resourceScope: coreResourceScope()}
		require.True(t, provider.HasSynced())

		assertRestricted(t, e)

		// The rule arrives: its own scope applies from now on.
		provider.Set(rules.Rule{
			Name:            "late",
			Subjects:        []rules.Subject{{Kind: "User", Name: "late-user"}},
			LimitNamespaces: []string{"team-a"},
		})

		decision, _, err := e.Authorize(context.Background(), &mockAttrs{
			userInfo:   lateUser,
			verb:       "get",
			resource:   "pods",
			namespace:  "team-a",
			isResource: true,
		})
		require.NoError(t, err)
		assert.Equal(t, authorizer.DecisionNoOpinion, decision, "inside limitNamespaces once the rule is known")

		decision, reason, err := e.Authorize(context.Background(), &mockAttrs{
			userInfo:   lateUser,
			verb:       "get",
			resource:   "pods",
			namespace:  "team-b",
			isResource: true,
		})
		require.NoError(t, err)
		assert.Equal(t, authorizer.DecisionDeny, decision, "outside limitNamespaces once the rule is known")
		assert.Equal(t, noNamespaceAccessReason, reason)

		decision, _, err = e.Authorize(context.Background(), &mockAttrs{
			userInfo:   lateUser,
			verb:       "list",
			resource:   "nodes",
			isResource: true,
		})
		require.NoError(t, err)
		assert.Equal(t, authorizer.DecisionNoOpinion, decision, "a cluster-scoped resource is not limited by namespaces")

		accessType, filter := e.GetNamespaceAccessType(lateUser)
		assert.Equal(t, FilteredAccess, accessType)
		require.NotNil(t, filter)
		assert.True(t, e.IsNamespaceAllowedWithFilter("team-a", filter))
		assert.False(t, e.IsNamespaceAllowedWithFilter("team-b", filter))
	})

	t.Run("rules not listed yet", func(t *testing.T) {
		e := &Engine{rules: mttest.Unsynced(), bindings: bindings, resourceScope: coreResourceScope()}
		require.False(t, e.rules.HasSynced())

		assertRestricted(t, e)

		accessType, filter := e.GetNamespaceAccessType(nobody)
		assert.Equal(t, NoNamespacesAllowed, accessType, "no CAR and not privileged: deny-by-default for discovery")
		assert.Nil(t, filter)
	})

	t.Run("a binding whose rule the directory knows adds nothing", func(t *testing.T) {
		e := &Engine{
			rules: mttest.Rules(rules.Rule{
				Name:              "late",
				Subjects:          []rules.Subject{{Kind: "User", Name: "late-user"}},
				NamespaceSelector: &rules.NamespaceSelector{MatchAny: true},
			}),
			bindings:      bindings,
			resourceScope: coreResourceScope(),
		}

		decision, _, err := e.Authorize(context.Background(), &mockAttrs{
			userInfo:   lateUser,
			verb:       "list",
			resource:   "pods",
			isResource: true,
		})
		require.NoError(t, err)
		assert.Equal(t, authorizer.DecisionNoOpinion, decision, "matchAny leaves the cluster-scoped list to RBAC")

		accessType, filter := e.GetNamespaceAccessType(lateUser)
		assert.Equal(t, AllNamespacesAllowed, accessType)
		assert.Nil(t, filter)
	})
}
