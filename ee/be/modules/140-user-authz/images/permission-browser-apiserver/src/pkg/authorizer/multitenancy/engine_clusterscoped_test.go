/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/discovery"
)

// stubDiscovery answers only the two methods Authorize uses on a cache miss.
// Unused DiscoveryInterface methods panic via the nil embed — that is
// deliberate: a new live call on the hot path must fail the test, not hide.
type stubDiscovery struct {
	discovery.DiscoveryInterface
	err  error
	list *metav1.APIResourceList
}

func (s *stubDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.list != nil {
		return s.list, nil
	}
	return &metav1.APIResourceList{GroupVersion: "v1"}, nil
}

func (s *stubDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &metav1.APIGroupList{}, nil
}

func gvrCacheKey(group, version, resource string) string {
	return schema.GroupVersionResource{Group: group, Version: version, Resource: resource}.String()
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

func engineWithKnownScopes(t *testing.T, config string, disco discovery.DiscoveryInterface) *Engine {
	t.Helper()
	e, err := NewEngine(writeConfigJSON(t, config), nil, nil, disco)
	require.NoError(t, err)
	e.namespacedCacheMu.Lock()
	e.namespacedCache[gvrCacheKey("", "", "pods")] = true
	e.namespacedCache[gvrCacheKey("", "", "nodes")] = false
	e.namespacedCacheMu.Unlock()
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
// resources. Scope is injected via namespacedCache so this does not depend on
// live discovery. A later ResourceScopeCache wiring must not change these
// rows for pods/nodes.
func TestEngine_Authorize_ClusterScopedFilterContract(t *testing.T) {
	editor := engineWithKnownScopes(t, editorCARConfig, nil)
	super := engineWithKnownScopes(t, superAdminCARConfig, nil)

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
	editor := engineWithKnownScopes(t, editorCARConfig, nil)
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

// TestEngine_Authorize_DiscoveryMissFailsOpen documents today's fail-open
// on a live discovery error: MT returns NoOpinion, so a CAR Editor CRB can
// report Allow for cluster-scoped list of an unknown resource. The webhook
// Denies the same miss. The ResourceScopeCache wiring is allowed to change
// this one case to Deny; do not treat it as filter-strictness.
func TestEngine_Authorize_DiscoveryMissFailsOpen(t *testing.T) {
	disco := &stubDiscovery{err: errors.New("discovery unavailable")}
	editor, err := NewEngine(writeConfigJSON(t, editorCARConfig), nil, nil, disco)
	require.NoError(t, err)

	got := authorizeResource(t, editor, "editor@example.io", "list", "newcrds", "")
	assert.Equal(t, authorizer.DecisionNoOpinion, got,
		"current behavior: discovery error is NoOpinion (fail-open)")
}

func TestEngine_Authorize_UnknownResourceTreatedAsClusterScoped(t *testing.T) {
	disco := &stubDiscovery{list: &metav1.APIResourceList{GroupVersion: "v1"}}
	editor, err := NewEngine(writeConfigJSON(t, editorCARConfig), nil, nil, disco)
	require.NoError(t, err)

	got := authorizeResource(t, editor, "editor@example.io", "list", "doesnotexist", "")
	assert.Equal(t, authorizer.DecisionNoOpinion, got,
		"current behavior: resource absent from the GV list is treated as cluster-scoped")
}

func TestEngine_GetNamespaceAccessType_SuperAdminVsEditor(t *testing.T) {
	editor := engineWithKnownScopes(t, editorCARConfig, nil)
	super := engineWithKnownScopes(t, superAdminCARConfig, nil)

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
