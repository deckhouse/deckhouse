/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// staticResourceScope is a test ResourceScope. Missing keys are unknown, and
// an empty map stands for a snapshot discovery never filled.
type staticResourceScope map[string]bool

func (s staticResourceScope) Scope(group, resource string) (namespaced, known bool) {
	namespaced, known = s[group+"/"+resource]
	return namespaced, known
}

func (s staticResourceScope) HasData() bool { return len(s) > 0 }

func coreResourceScope() staticResourceScope {
	return staticResourceScope{
		"/pods":  true,
		"/nodes": false,
	}
}

const (
	editorCARConfig = `{
		"crds": [{
			"name": "editor",
			"spec": {
				"accessLevel": "Editor",
				"limitNamespaces": ["ns-in"],
				"subjects": [{"kind": "User", "name": "editor@example.io"}]
			}
		}]
	}`
	superAdminCARConfig = `{
		"crds": [{
			"name": "super",
			"spec": {
				"accessLevel": "SuperAdmin",
				"allowAccessToSystemNamespaces": true,
				"namespaceSelector": {"matchAny": true},
				"subjects": [{"kind": "User", "name": "super@example.io"}]
			}
		}]
	}`
)

func engineWithKnownScopes(t *testing.T, config string) *Engine {
	t.Helper()
	e, err := NewEngine(writeConfigJSON(t, config), nil, nil, coreResourceScope())
	require.NoError(t, err)
	return e
}

func authorizeResource(t *testing.T, e *Engine, userName, verb, resource, namespace string) authorizer.Decision {
	t.Helper()
	decision, _, err := e.Authorize(context.Background(), &mockAttrs{
		userInfo:   &mockUserInfo{name: userName},
		verb:       verb,
		resource:   resource,
		namespace:  namespace,
		isResource: true,
	})
	require.NoError(t, err)
	return decision
}

// TestEngine_Authorize_ClusterScopedFilterContract locks the deny-only MT
// answers that BulkSAR and the webhook must keep agreeing on for well-known
// resources. Scope is injected via ResourceScope so this does not depend on
// live discovery.
func TestEngine_Authorize_ClusterScopedFilterContract(t *testing.T) {
	editor := engineWithKnownScopes(t, editorCARConfig)
	super := engineWithKnownScopes(t, superAdminCARConfig)

	tests := []struct {
		name     string
		engine   *Engine
		user     string
		verb     string
		resource string
		ns       string
		want     authorizer.Decision
	}{
		{
			name:     "editor cluster-scoped list of namespaced pods is denied",
			engine:   editor,
			user:     "editor@example.io",
			verb:     "list",
			resource: "pods",
			want:     authorizer.DecisionDeny,
		},
		{
			name:     "editor cluster-scoped watch of namespaced pods is denied",
			engine:   editor,
			user:     "editor@example.io",
			verb:     "watch",
			resource: "pods",
			want:     authorizer.DecisionDeny,
		},
		{
			name:     "editor get of cluster-scoped nodes is NoOpinion",
			engine:   editor,
			user:     "editor@example.io",
			verb:     "get",
			resource: "nodes",
			want:     authorizer.DecisionNoOpinion,
		},
		{
			name:     "editor get pods in CAR namespace is NoOpinion",
			engine:   editor,
			user:     "editor@example.io",
			verb:     "get",
			resource: "pods",
			ns:       "ns-in",
			want:     authorizer.DecisionNoOpinion,
		},
		{
			name:     "editor get pods outside CAR namespace is denied",
			engine:   editor,
			user:     "editor@example.io",
			verb:     "get",
			resource: "pods",
			ns:       "ns-out",
			want:     authorizer.DecisionDeny,
		},
		{
			name:     "editor get pods in kube-system is denied (no system access)",
			engine:   editor,
			user:     "editor@example.io",
			verb:     "get",
			resource: "pods",
			ns:       "kube-system",
			want:     authorizer.DecisionDeny,
		},
		{
			name:     "superadmin cluster-scoped list of pods is NoOpinion (no MT filters)",
			engine:   super,
			user:     "super@example.io",
			verb:     "list",
			resource: "pods",
			want:     authorizer.DecisionNoOpinion,
		},
		{
			name:     "superadmin get pods in any namespace is NoOpinion",
			engine:   super,
			user:     "super@example.io",
			verb:     "get",
			resource: "pods",
			ns:       "ns-out",
			want:     authorizer.DecisionNoOpinion,
		},
		{
			name:     "superadmin get pods in kube-system is NoOpinion",
			engine:   super,
			user:     "super@example.io",
			verb:     "get",
			resource: "pods",
			ns:       "kube-system",
			want:     authorizer.DecisionNoOpinion,
		},
		{
			name:     "unknown user is NoOpinion (RBAC decides)",
			engine:   editor,
			user:     "nobody@example.io",
			verb:     "list",
			resource: "pods",
			want:     authorizer.DecisionNoOpinion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := authorizeResource(t, tt.engine, tt.user, tt.verb, tt.resource, tt.ns)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestEngine_Authorize_ClusterScopedIndependentRBAC locks the documented
// exception: a non-CAR ClusterRoleBinding that grants cluster-wide list of a
// namespaced resource must not be denied by the CAR namespace filter.
func TestEngine_Authorize_ClusterScopedIndependentRBAC(t *testing.T) {
	editor := engineWithKnownScopes(t, editorCARConfig)
	editor.SetIndependentRBACChecker(&clusterIndependentChecker{allowClusterScoped: true})

	got := authorizeResource(t, editor, "editor@example.io", "list", "pods", "")
	assert.Equal(t, authorizer.DecisionNoOpinion, got,
		"independent cluster-wide grant must skip the namespaced-resource deny")
}

type clusterIndependentChecker struct {
	allowClusterScoped bool
	namespaces         map[string]struct{}
}

func (c *clusterIndependentChecker) AllowsIndependently(_ context.Context, attrs authorizer.Attributes) bool {
	if attrs.GetNamespace() == "" {
		return c.allowClusterScoped
	}
	_, ok := c.namespaces[attrs.GetNamespace()]
	return ok
}

// authorizeGroupedResource is authorizeResource for a resource that lives in
// an API group. The webhook resolves group and core resources differently, so
// the unknown-resource rows must state which one they mean.
func authorizeGroupedResource(t *testing.T, e *Engine, userName, group, resource string) authorizer.Decision {
	t.Helper()
	decision, _, err := e.Authorize(context.Background(), &mockAttrs{
		userInfo:   &mockUserInfo{name: userName},
		verb:       "list",
		apiGroup:   group,
		resource:   resource,
		isResource: true,
	})
	require.NoError(t, err)
	return decision
}

func TestEngine_Authorize_UnknownGroupedResourceIsDenied(t *testing.T) {
	editor := engineWithKnownScopes(t, editorCARConfig)

	got := authorizeGroupedResource(t, editor, "editor@example.io", "example.io", "doesnotexist")
	assert.Equal(t, authorizer.DecisionDeny, got,
		"a grouped resource absent from the scope snapshot must not fail-open")
}

// TestEngine_Authorize_UnknownCoreResourceMatchesWebhook locks parity with
// the enforcement webhook, which permits an unknown core resource so RBAC can
// answer instead of denying it. Reporting Deny here would show a restriction
// the cluster does not apply.
func TestEngine_Authorize_UnknownCoreResourceMatchesWebhook(t *testing.T) {
	editor := engineWithKnownScopes(t, editorCARConfig)

	got := authorizeGroupedResource(t, editor, "editor@example.io", "", "doesnotexist")
	assert.Equal(t, authorizer.DecisionNoOpinion, got,
		"an unknown core resource is left to RBAC, exactly as the webhook does")
}

// TestEngine_Authorize_EmptySnapshotDeniesCoreResource guards the other side
// of that carve-out: with no snapshot at all we cannot tell a missing
// resource from missing discovery, and the webhook denies too.
func TestEngine_Authorize_EmptySnapshotDeniesCoreResource(t *testing.T) {
	editor, err := NewEngine(writeConfigJSON(t, editorCARConfig), nil, nil, staticResourceScope{})
	require.NoError(t, err)

	got := authorizeGroupedResource(t, editor, "editor@example.io", "", "pods")
	assert.Equal(t, authorizer.DecisionDeny, got,
		"an unpopulated snapshot must not turn every core resource into a pass")
}

func TestEngine_Authorize_NilScopeIsDenied(t *testing.T) {
	editor, err := NewEngine(writeConfigJSON(t, editorCARConfig), nil, nil, nil)
	require.NoError(t, err)

	got := authorizeResource(t, editor, "editor@example.io", "list", "pods", "")
	assert.Equal(t, authorizer.DecisionDeny, got,
		"nil ResourceScope is !known and must Deny for a filtered user")
}

func TestEngine_Authorize_UnknownResourceIndependentRBAC(t *testing.T) {
	editor := engineWithKnownScopes(t, editorCARConfig)
	editor.SetIndependentRBACChecker(&clusterIndependentChecker{allowClusterScoped: true})

	got := authorizeGroupedResource(t, editor, "editor@example.io", "example.io", "newcrds")
	assert.Equal(t, authorizer.DecisionNoOpinion, got,
		"independent cluster-wide grant still skips the unknown-resource deny")
}

func TestEngine_GetNamespaceAccessType_SuperAdminVsEditor(t *testing.T) {
	editor := engineWithKnownScopes(t, editorCARConfig)
	super := engineWithKnownScopes(t, superAdminCARConfig)

	access, filter := editor.GetNamespaceAccessType(&mockUserInfo{name: "editor@example.io"})
	assert.Equal(t, FilteredAccess, access)
	require.NotNil(t, filter)
	assert.True(t, editor.IsNamespaceAllowedWithFilter("ns-in", filter))
	assert.False(t, editor.IsNamespaceAllowedWithFilter("ns-out", filter))
	assert.False(t, editor.IsNamespaceAllowedWithFilter("kube-system", filter))

	access, filter = super.GetNamespaceAccessType(&mockUserInfo{name: "super@example.io"})
	assert.Equal(t, AllNamespacesAllowed, access)
	assert.Nil(t, filter)
}
