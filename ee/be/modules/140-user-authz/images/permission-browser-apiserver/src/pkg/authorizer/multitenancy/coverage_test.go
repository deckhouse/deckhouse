/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"

	"permission-browser-apiserver/pkg/authorizer/multitenancy/mttest"
)

// coverageEngine builds an engine over the rules, with no namespace lister (none of the scenarios
// below exercise a namespaceSelector's matchLabels) and the core resource scope.
func coverageEngine(t *testing.T, rs ...rules.Rule) *Engine {
	t.Helper()
	engine, err := NewEngine(mttest.Rules(rs...), mttest.NoBindings(), nil, nil, coreResourceScope())
	require.NoError(t, err)
	return engine
}

// TestEngine_InitialConfigLoad tests that the rules of a subject reach the directory.
func TestEngine_InitialConfigLoad(t *testing.T) {
	engine := coverageEngine(t, rules.Rule{
		Name:            "test-rule",
		LimitNamespaces: []string{"allowed-ns"},
		Subjects:        []rules.Subject{{Kind: "User", Name: "testuser"}},
	})

	entries := engine.affectedEntries("testuser", nil)
	assert.Len(t, entries, 1, "should have one entry for testuser")
}

// TestEngine_SystemNamespaces tests restrictions on system namespaces
func TestEngine_SystemNamespaces(t *testing.T) {
	// Configuration WITHOUT access to system namespaces
	engine := coverageEngine(t, rules.Rule{
		Name:                          "limited-user-rule",
		AllowAccessToSystemNamespaces: false,
		Subjects:                      []rules.Subject{{Kind: "User", Name: "limited-user"}},
	})

	systemNamespaces := []string{"kube-system", "kube-public", "d8-system", "default"}

	for _, ns := range systemNamespaces {
		t.Run("system_namespace_"+ns, func(t *testing.T) {
			attrs := &testAttrs{
				user:            &user.DefaultInfo{Name: "limited-user"},
				verb:            "get",
				namespace:       ns,
				resource:        "pods",
				resourceRequest: true,
			}

			decision, reason, err := engine.Authorize(context.Background(), attrs)
			require.NoError(t, err)
			assert.Equal(t, authorizer.DecisionDeny, decision, "system namespace %s should be denied", ns)
			assert.Contains(t, reason, "no access")
		})
	}
}

// TestEngine_GroupBasedRules tests group-based authorization rules
func TestEngine_GroupBasedRules(t *testing.T) {
	engine := coverageEngine(t, rules.Rule{
		Name:            "developers-rule",
		LimitNamespaces: []string{"dev-.*"},
		Subjects:        []rules.Subject{{Kind: "Group", Name: "developers"}},
	})

	tests := []struct {
		name       string
		user       string
		groups     []string
		namespace  string
		expectDeny bool
	}{
		{"developer in dev-team", "alice", []string{"developers"}, "dev-team", false},
		{"developer in dev-frontend", "bob", []string{"developers"}, "dev-frontend", false},
		{"developer in prod", "alice", []string{"developers"}, "prod", true},
		{"non-developer in dev-team", "charlie", []string{"guests"}, "dev-team", false}, // no rule for guests
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := &testAttrs{
				user:            &user.DefaultInfo{Name: tt.user, Groups: tt.groups},
				verb:            "get",
				namespace:       tt.namespace,
				resource:        "pods",
				resourceRequest: true,
			}

			decision, _, err := engine.Authorize(context.Background(), attrs)
			require.NoError(t, err)

			if tt.expectDeny {
				assert.Equal(t, authorizer.DecisionDeny, decision)
			} else {
				assert.NotEqual(t, authorizer.DecisionDeny, decision)
			}
		})
	}
}

// TestEngine_ServiceAccountRules tests ServiceAccount authorization rules
func TestEngine_ServiceAccountRules(t *testing.T) {
	engine := coverageEngine(t, rules.Rule{
		Name:            "sa-rule",
		LimitNamespaces: []string{"app-ns"},
		Subjects:        []rules.Subject{{Kind: "ServiceAccount", Name: "my-sa", Namespace: "app-ns"}},
	})

	// ServiceAccount user format: system:serviceaccount:<namespace>:<name>
	saUser := "system:serviceaccount:app-ns:my-sa"

	tests := []struct {
		name       string
		namespace  string
		expectDeny bool
	}{
		{"SA in allowed namespace", "app-ns", false},
		{"SA in other namespace", "other-ns", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := &testAttrs{
				user:            &user.DefaultInfo{Name: saUser},
				verb:            "get",
				namespace:       tt.namespace,
				resource:        "pods",
				resourceRequest: true,
			}

			decision, _, err := engine.Authorize(context.Background(), attrs)
			require.NoError(t, err)

			if tt.expectDeny {
				assert.Equal(t, authorizer.DecisionDeny, decision)
			} else {
				assert.NotEqual(t, authorizer.DecisionDeny, decision)
			}
		})
	}
}

// TestEngine_MatchAnySelector tests MatchAny selector behavior
func TestEngine_MatchAnySelector(t *testing.T) {
	// Rule with MatchAny = true - allows all namespaces
	engine := coverageEngine(t, rules.Rule{
		Name:              "match-any-rule",
		NamespaceSelector: &rules.NamespaceSelector{MatchAny: true},
		Subjects:          []rules.Subject{{Kind: "User", Name: "super-user"}},
	})

	// With MatchAny=true user should not be blocked in any namespace
	attrs := &testAttrs{
		user:            &user.DefaultInfo{Name: "super-user"},
		verb:            "get",
		namespace:       "any-namespace",
		resource:        "pods",
		resourceRequest: true,
	}

	decision, _, err := engine.Authorize(context.Background(), attrs)
	require.NoError(t, err)
	assert.Equal(t, authorizer.DecisionNoOpinion, decision, "MatchAny should allow all namespaces")
}

// TestEngine_NonResourceRequestSkipped tests that non-resource requests are skipped
func TestEngine_NonResourceRequestSkipped(t *testing.T) {
	engine := coverageEngine(t)

	attrs := &testAttrs{
		user:            &user.DefaultInfo{Name: "anyone"},
		verb:            "get",
		path:            "/healthz",
		resourceRequest: false,
	}

	decision, _, err := engine.Authorize(context.Background(), attrs)
	require.NoError(t, err)
	assert.Equal(t, authorizer.DecisionNoOpinion, decision, "non-resource requests should be skipped")
}

// === Helpers ===

type testAttrs struct {
	user            user.Info
	verb            string
	namespace       string
	resource        string
	subresource     string
	name            string
	apiGroup        string
	apiVersion      string
	path            string
	resourceRequest bool
}

func (t *testAttrs) GetUser() user.Info { return t.user }
func (t *testAttrs) GetVerb() string    { return t.verb }
func (t *testAttrs) IsReadOnly() bool {
	return t.verb == "get" || t.verb == "list" || t.verb == "watch"
}
func (t *testAttrs) GetNamespace() string                           { return t.namespace }
func (t *testAttrs) GetResource() string                            { return t.resource }
func (t *testAttrs) GetSubresource() string                         { return t.subresource }
func (t *testAttrs) GetName() string                                { return t.name }
func (t *testAttrs) GetAPIGroup() string                            { return t.apiGroup }
func (t *testAttrs) GetAPIVersion() string                          { return t.apiVersion }
func (t *testAttrs) IsResourceRequest() bool                        { return t.resourceRequest }
func (t *testAttrs) GetPath() string                                { return t.path }
func (t *testAttrs) GetFieldSelector() (fields.Requirements, error) { return nil, nil }
func (t *testAttrs) GetLabelSelector() (labels.Requirements, error) { return nil, nil }
