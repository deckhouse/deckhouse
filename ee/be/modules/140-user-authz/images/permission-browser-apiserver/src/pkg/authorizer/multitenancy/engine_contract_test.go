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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

// consoleResourceScope is the GVR set the console BulkSAR payload actually
// asks about. A miss here is !known and must Deny for a filtered user.
func consoleResourceScope() staticResourceScope {
	return staticResourceScope{
		"/pods":                                  true,
		"/services":                              true,
		"/configmaps":                            true,
		"/secrets":                               true,
		"/namespaces":                            false,
		"/nodes":                                 false,
		"/persistentvolumes":                     false,
		"apps/deployments":                       true,
		"apps/replicasets":                       true,
		"apps/statefulsets":                      true,
		"batch/jobs":                             true,
		"batch/cronjobs":                         true,
		"networking.k8s.io/ingresses":            true,
		"rbac.authorization.k8s.io/roles":        true,
		"rbac.authorization.k8s.io/clusterroles": false,
		"rbac.authorization.k8s.io/clusterrolebindings":  false,
		"apiextensions.k8s.io/customresourcedefinitions": false,
		"deckhouse.io/projects":                          false,
		"metrics.k8s.io/pods":                            true,
		"metrics.k8s.io/nodes":                           false,
	}
}

const (
	editorLabelSelectorCARConfig = `{
		"crds": [{
			"name": "editor-sel",
			"spec": {
				"accessLevel": "Editor",
				"namespaceSelector": {"labelSelector": {"matchLabels": {"pba-perf": "true"}}},
				"subjects": [{"kind": "User", "name": "editor@example.io"}]
			}
		}]
	}`
	editorMatchAnyCARConfig = `{
		"crds": [{
			"name": "editor-any",
			"spec": {
				"accessLevel": "Editor",
				"namespaceSelector": {"matchAny": true},
				"subjects": [{"kind": "User", "name": "editor-any@example.io"}]
			}
		}]
	}`
	superAdminLimitedCARConfig = `{
		"crds": [{
			"name": "super-limited",
			"spec": {
				"accessLevel": "SuperAdmin",
				"allowAccessToSystemNamespaces": true,
				"limitNamespaces": ["ns-in"],
				"subjects": [{"kind": "User", "name": "super-limited@example.io"}]
			}
		}]
	}`
	clusterAdminMatchAnyCARConfig = `{
		"crds": [{
			"name": "ca-any",
			"spec": {
				"accessLevel": "ClusterAdmin",
				"namespaceSelector": {"matchAny": true},
				"subjects": [{"kind": "User", "name": "ca-any@example.io"}]
			}
		}]
	}`
	clusterAdminLimitedCARConfig = `{
		"crds": [{
			"name": "ca-lim",
			"spec": {
				"accessLevel": "ClusterAdmin",
				"limitNamespaces": ["ns-in"],
				"subjects": [{"kind": "User", "name": "ca-lim@example.io"}]
			}
		}]
	}`
	editorNoFilterCARConfig = `{
		"crds": [{
			"name": "editor-wide",
			"spec": {
				"accessLevel": "Editor",
				"subjects": [{"kind": "User", "name": "editor-wide@example.io"}]
			}
		}]
	}`
	editorGroupCARConfig = `{
		"crds": [{
			"name": "editor-group",
			"spec": {
				"accessLevel": "Editor",
				"limitNamespaces": ["ns-in"],
				"subjects": [{"kind": "Group", "name": "editors"}]
			}
		}]
	}`
)

func authorizeAs(t *testing.T, e *Engine, u user.Info, verb, group, resource, namespace string) (authorizer.Decision, string) {
	t.Helper()
	decision, reason, err := e.Authorize(context.Background(), &mockAttrs{
		userInfo:   u,
		verb:       verb,
		apiGroup:   group,
		resource:   resource,
		namespace:  namespace,
		isResource: true,
	})
	require.NoError(t, err)
	return decision, reason
}

func engineWithConsoleScope(t *testing.T, config string) *Engine {
	t.Helper()
	e, err := NewEngine(writeConfigJSON(t, config), nil, nil, consoleResourceScope())
	require.NoError(t, err)
	return e
}

func labeledNamespaces() []runtime.Object {
	return []runtime.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   "ns-in",
			Labels: map[string]string{"pba-perf": "true"},
		}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-out"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-system"}},
	}
}

func engineWithNamespaces(t *testing.T, config string, objs ...runtime.Object) *Engine {
	t.Helper()
	client := fake.NewSimpleClientset(objs...)
	factory := informers.NewSharedInformerFactory(client, 0)
	nsInformer := factory.Core().V1().Namespaces()
	lister := nsInformer.Lister() // register before Start
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	factory.Start(stop)
	for typ, ok := range factory.WaitForCacheSync(stop) {
		require.True(t, ok, "informer %v failed to sync", typ)
	}

	e, err := NewEngine(writeConfigJSON(t, config), lister, func() bool { return true }, consoleResourceScope())
	require.NoError(t, err)
	return e
}

// TestEngine_Authorize_AccessLevelDoesNotGrant locks that SuperAdmin/Editor/
// ClusterAdmin names are irrelevant to MT. Filters decide: matchAny is
// NoOpinion, a real namespace limit Denies the same cluster-scoped list.
func TestEngine_Authorize_AccessLevelDoesNotGrant(t *testing.T) {
	editorLimited := engineWithConsoleScope(t, editorCARConfig)
	editorAny := engineWithConsoleScope(t, editorMatchAnyCARConfig)
	superLimited := engineWithConsoleScope(t, superAdminLimitedCARConfig)
	superAny := engineWithConsoleScope(t, superAdminCARConfig)
	caLimited := engineWithConsoleScope(t, clusterAdminLimitedCARConfig)
	caAny := engineWithConsoleScope(t, clusterAdminMatchAnyCARConfig)

	tests := []struct {
		name   string
		engine *Engine
		user   string
		want   authorizer.Decision
	}{
		{name: "Editor + limitNamespaces denies cluster-scoped list pods", engine: editorLimited, user: "editor@example.io", want: authorizer.DecisionDeny},
		{name: "Editor + matchAny is NoOpinion on cluster-scoped list pods", engine: editorAny, user: "editor-any@example.io", want: authorizer.DecisionNoOpinion},
		{name: "SuperAdmin + limitNamespaces denies cluster-scoped list pods", engine: superLimited, user: "super-limited@example.io", want: authorizer.DecisionDeny},
		{name: "SuperAdmin + matchAny is NoOpinion on cluster-scoped list pods", engine: superAny, user: "super@example.io", want: authorizer.DecisionNoOpinion},
		{name: "ClusterAdmin + limitNamespaces denies cluster-scoped list pods", engine: caLimited, user: "ca-lim@example.io", want: authorizer.DecisionDeny},
		{name: "ClusterAdmin + matchAny is NoOpinion on cluster-scoped list pods", engine: caAny, user: "ca-any@example.io", want: authorizer.DecisionNoOpinion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := authorizeAs(t, tt.engine, &mockUserInfo{name: tt.user}, "list", "", "pods", "")
			assert.Equal(t, tt.want, got, "reason=%q", reason)
			if tt.want == authorizer.DecisionDeny {
				assert.Equal(t, namespaceLimitedAccessReason, reason)
			}
		})
	}
}

// TestEngine_Authorize_ConsoleResourceMatrix pins MT answers for every
// decision class the console payload hits. RBAC is not in this test: Deny
// stays Deny, NoOpinion is "RBAC decides".
func TestEngine_Authorize_ConsoleResourceMatrix(t *testing.T) {
	editor := engineWithConsoleScope(t, editorCARConfig)
	super := engineWithConsoleScope(t, superAdminCARConfig)
	wide := engineWithConsoleScope(t, editorNoFilterCARConfig)

	type req struct {
		verb, group, resource, ns string
	}
	filteredEditor := []struct {
		name string
		req  req
		want authorizer.Decision
	}{
		{name: "list pods cluster-scoped", req: req{verb: "list", resource: "pods"}, want: authorizer.DecisionDeny},
		{name: "watch pods cluster-scoped", req: req{verb: "watch", resource: "pods"}, want: authorizer.DecisionDeny},
		{name: "create pods cluster-scoped", req: req{verb: "create", resource: "pods"}, want: authorizer.DecisionDeny},
		{name: "list services cluster-scoped", req: req{verb: "list", resource: "services"}, want: authorizer.DecisionDeny},
		{name: "list secrets cluster-scoped", req: req{verb: "list", resource: "secrets"}, want: authorizer.DecisionDeny},
		{name: "list configmaps cluster-scoped", req: req{verb: "list", resource: "configmaps"}, want: authorizer.DecisionDeny},
		{name: "list deployments cluster-scoped", req: req{verb: "list", group: "apps", resource: "deployments"}, want: authorizer.DecisionDeny},
		{name: "list jobs cluster-scoped", req: req{verb: "list", group: "batch", resource: "jobs"}, want: authorizer.DecisionDeny},
		{name: "list ingresses cluster-scoped", req: req{verb: "list", group: "networking.k8s.io", resource: "ingresses"}, want: authorizer.DecisionDeny},
		{name: "list roles cluster-scoped", req: req{verb: "list", group: "rbac.authorization.k8s.io", resource: "roles"}, want: authorizer.DecisionDeny},
		{name: "list metrics pods cluster-scoped", req: req{verb: "list", group: "metrics.k8s.io", resource: "pods"}, want: authorizer.DecisionDeny},
		{name: "get nodes", req: req{verb: "get", resource: "nodes"}, want: authorizer.DecisionNoOpinion},
		{name: "list nodes", req: req{verb: "list", resource: "nodes"}, want: authorizer.DecisionNoOpinion},
		{name: "list namespaces", req: req{verb: "list", resource: "namespaces"}, want: authorizer.DecisionNoOpinion},
		{name: "list clusterroles", req: req{verb: "list", group: "rbac.authorization.k8s.io", resource: "clusterroles"}, want: authorizer.DecisionNoOpinion},
		{name: "list persistentvolumes", req: req{verb: "list", resource: "persistentvolumes"}, want: authorizer.DecisionNoOpinion},
		{name: "list CRDs", req: req{verb: "list", group: "apiextensions.k8s.io", resource: "customresourcedefinitions"}, want: authorizer.DecisionNoOpinion},
		{name: "list projects", req: req{verb: "list", group: "deckhouse.io", resource: "projects"}, want: authorizer.DecisionNoOpinion},
		{name: "list metrics nodes", req: req{verb: "list", group: "metrics.k8s.io", resource: "nodes"}, want: authorizer.DecisionNoOpinion},
		{name: "get pods in CAR ns", req: req{verb: "get", resource: "pods", ns: "ns-in"}, want: authorizer.DecisionNoOpinion},
		{name: "list pods in CAR ns", req: req{verb: "list", resource: "pods", ns: "ns-in"}, want: authorizer.DecisionNoOpinion},
		{name: "create pods in CAR ns", req: req{verb: "create", resource: "pods", ns: "ns-in"}, want: authorizer.DecisionNoOpinion},
		{name: "delete deployments in CAR ns", req: req{verb: "delete", group: "apps", resource: "deployments", ns: "ns-in"}, want: authorizer.DecisionNoOpinion},
		{name: "get pods outside CAR ns", req: req{verb: "get", resource: "pods", ns: "ns-out"}, want: authorizer.DecisionDeny},
		{name: "create secrets outside CAR ns", req: req{verb: "create", resource: "secrets", ns: "ns-out"}, want: authorizer.DecisionDeny},
		{name: "get pods in kube-system", req: req{verb: "get", resource: "pods", ns: "kube-system"}, want: authorizer.DecisionDeny},
		{name: "get pods in d8-system", req: req{verb: "get", resource: "pods", ns: "d8-system"}, want: authorizer.DecisionDeny},
		{name: "unknown GVR cluster-scoped", req: req{verb: "list", group: "example.com", resource: "newcrds"}, want: authorizer.DecisionDeny},
	}

	for _, tt := range filteredEditor {
		t.Run("editor/"+tt.name, func(t *testing.T) {
			got, reason := authorizeAs(t, editor, &mockUserInfo{name: "editor@example.io"}, tt.req.verb, tt.req.group, tt.req.resource, tt.req.ns)
			assert.Equal(t, tt.want, got, "reason=%q", reason)
			if tt.want == authorizer.DecisionDeny && tt.req.ns == "" {
				assert.Equal(t, namespaceLimitedAccessReason, reason)
			}
			if tt.want == authorizer.DecisionDeny && tt.req.ns != "" {
				assert.Equal(t, noNamespaceAccessReason, reason)
			}
		})
		t.Run("super/"+tt.name, func(t *testing.T) {
			got, reason := authorizeAs(t, super, &mockUserInfo{name: "super@example.io"}, tt.req.verb, tt.req.group, tt.req.resource, tt.req.ns)
			assert.Equal(t, authorizer.DecisionNoOpinion, got, "SuperAdmin matchAny must never Deny; reason=%q", reason)
			assert.Empty(t, reason)
		})
	}

	t.Run("editor without namespace filters still denies cluster-scoped namespaced list", func(t *testing.T) {
		got, reason := authorizeAs(t, wide, &mockUserInfo{name: "editor-wide@example.io"}, "list", "", "pods", "")
		assert.Equal(t, authorizer.DecisionDeny, got)
		assert.Equal(t, namespaceLimitedAccessReason, reason)
	})
	t.Run("editor without namespace filters allows non-system ns and denies kube-system", func(t *testing.T) {
		got, _ := authorizeAs(t, wide, &mockUserInfo{name: "editor-wide@example.io"}, "get", "", "pods", "ns-out")
		assert.Equal(t, authorizer.DecisionNoOpinion, got)
		got, reason := authorizeAs(t, wide, &mockUserInfo{name: "editor-wide@example.io"}, "get", "", "pods", "kube-system")
		assert.Equal(t, authorizer.DecisionDeny, got)
		assert.Equal(t, noNamespaceAccessReason, reason)
	})
}

// TestEngine_Authorize_LimitNamespacesEqualsLabelSelector locks that the two
// CAR filter forms produce the same Deny/NoOpinion/reason for the same ns set.
func TestEngine_Authorize_LimitNamespacesEqualsLabelSelector(t *testing.T) {
	limited := engineWithNamespaces(t, editorCARConfig, labeledNamespaces()...)
	selected := engineWithNamespaces(t, editorLabelSelectorCARConfig, labeledNamespaces()...)
	user := &mockUserInfo{name: "editor@example.io"}

	reqs := []struct {
		name                      string
		verb, group, resource, ns string
		want                      authorizer.Decision
	}{
		{name: "list pods", verb: "list", resource: "pods", want: authorizer.DecisionDeny},
		{name: "list deployments", verb: "list", group: "apps", resource: "deployments", want: authorizer.DecisionDeny},
		{name: "get nodes", verb: "get", resource: "nodes", want: authorizer.DecisionNoOpinion},
		{name: "get pods ns-in", verb: "get", resource: "pods", ns: "ns-in", want: authorizer.DecisionNoOpinion},
		{name: "get pods ns-out", verb: "get", resource: "pods", ns: "ns-out", want: authorizer.DecisionDeny},
		{name: "get pods kube-system", verb: "get", resource: "pods", ns: "kube-system", want: authorizer.DecisionDeny},
		{name: "unknown GVR", verb: "list", group: "example.com", resource: "newcrds", want: authorizer.DecisionDeny},
	}

	for _, tt := range reqs {
		t.Run(tt.name, func(t *testing.T) {
			gotL, reasonL := authorizeAs(t, limited, user, tt.verb, tt.group, tt.resource, tt.ns)
			gotS, reasonS := authorizeAs(t, selected, user, tt.verb, tt.group, tt.resource, tt.ns)
			assert.Equal(t, tt.want, gotL, "limitNamespaces reason=%q", reasonL)
			assert.Equal(t, gotL, gotS, "labelSelector diverged: limit=%v (%q) selector=%v (%q)", gotL, reasonL, gotS, reasonS)
			assert.Equal(t, reasonL, reasonS)
		})
	}
}

func TestEngine_Authorize_WebhookReasonStrings(t *testing.T) {
	editor := engineWithConsoleScope(t, editorCARConfig)
	user := &mockUserInfo{name: "editor@example.io"}

	_, reason := authorizeAs(t, editor, user, "list", "", "pods", "")
	assert.Equal(t, "making cluster-scoped requests for namespaced resources is not allowed", reason)

	_, reason = authorizeAs(t, editor, user, "get", "", "pods", "ns-out")
	assert.Equal(t, "either you have no access to the namespace or the namespace does not exist", reason)

	_, reason = authorizeAs(t, editor, user, "get", "", "pods", "missing-ns")
	assert.Equal(t, "either you have no access to the namespace or the namespace does not exist", reason,
		"a closed namespace must not be distinguishable from a missing one")
}

func TestEngine_Authorize_GroupSubjectUsesSameFilters(t *testing.T) {
	e := engineWithConsoleScope(t, editorGroupCARConfig)
	u := &mockUserInfo{name: "someone@example.io", groups: []string{"editors", "system:authenticated"}}

	got, reason := authorizeAs(t, e, u, "list", "", "pods", "")
	assert.Equal(t, authorizer.DecisionDeny, got)
	assert.Equal(t, namespaceLimitedAccessReason, reason)

	got, _ = authorizeAs(t, e, u, "get", "", "pods", "ns-in")
	assert.Equal(t, authorizer.DecisionNoOpinion, got)

	got, reason = authorizeAs(t, e, u, "get", "", "pods", "ns-out")
	assert.Equal(t, authorizer.DecisionDeny, got)
	assert.Equal(t, noNamespaceAccessReason, reason)
}

func TestEngine_Authorize_SubresourceUsesParentScope(t *testing.T) {
	editor := engineWithConsoleScope(t, editorCARConfig)
	user := &mockUserInfo{name: "editor@example.io"}

	decision, reason, err := editor.Authorize(context.Background(), &mockAttrs{
		userInfo:    user,
		verb:        "get",
		resource:    "pods",
		subresource: "log",
		isResource:  true,
	})
	require.NoError(t, err)
	assert.Equal(t, authorizer.DecisionDeny, decision,
		"pods/log is still the namespaced pods GVR; reason=%q", reason)
	assert.Equal(t, namespaceLimitedAccessReason, reason)

	decision, _, err = editor.Authorize(context.Background(), &mockAttrs{
		userInfo:    user,
		verb:        "get",
		resource:    "pods",
		subresource: "log",
		namespace:   "ns-in",
		isResource:  true,
	})
	require.NoError(t, err)
	assert.Equal(t, authorizer.DecisionNoOpinion, decision)
}

func TestEngine_Authorize_NonResourceIsNoOpinion(t *testing.T) {
	editor := engineWithConsoleScope(t, editorCARConfig)
	decision, reason, err := editor.Authorize(context.Background(), &mockAttrs{
		userInfo:   &mockUserInfo{name: "editor@example.io"},
		verb:       "get",
		path:       "/healthz",
		isResource: false,
	})
	require.NoError(t, err)
	assert.Equal(t, authorizer.DecisionNoOpinion, decision)
	assert.Empty(t, reason)
}

func TestEngine_Authorize_CARCRBDoesNotSkipFilter(t *testing.T) {
	editor := engineWithConsoleScope(t, editorCARConfig)
	editor.SetIndependentRBACChecker(&clusterIndependentChecker{})

	got, reason := authorizeAs(t, editor, &mockUserInfo{name: "editor@example.io"}, "list", "", "pods", "")
	assert.Equal(t, authorizer.DecisionDeny, got,
		"AllowsIndependently=false must not rescue a cluster-scoped namespaced list")
	assert.Equal(t, namespaceLimitedAccessReason, reason)
}

func TestEngine_GetNamespaceAccessType_PrivilegedDoesNotOverrideCAR(t *testing.T) {
	editor := engineWithConsoleScope(t, editorCARConfig)

	access, filter := editor.GetNamespaceAccessType(&mockUserInfo{
		name:   "editor@example.io",
		groups: []string{"system:masters"},
	})
	assert.Equal(t, FilteredAccess, access,
		"a CAR on the user still filters even when the user is also in system:masters")
	require.NotNil(t, filter)
	assert.False(t, editor.IsNamespaceAllowedWithFilter("ns-out", filter))

	access, filter = editor.GetNamespaceAccessType(&mockUserInfo{
		name:   "nobody@example.io",
		groups: []string{"system:masters"},
	})
	assert.Equal(t, AllNamespacesAllowed, access)
	assert.Nil(t, filter)
}

func TestEngine_Authorize_TwoCARsUnionNamespaces(t *testing.T) {
	e := engineWithConsoleScope(t, `{
		"crds": [
			{
				"name": "a",
				"spec": {
					"limitNamespaces": ["ns-a"],
					"subjects": [{"kind": "User", "name": "dual@example.io"}]
				}
			},
			{
				"name": "b",
				"spec": {
					"limitNamespaces": ["ns-b"],
					"subjects": [{"kind": "User", "name": "dual@example.io"}]
				}
			}
		]
	}`)
	u := &mockUserInfo{name: "dual@example.io"}
	got, _ := authorizeAs(t, e, u, "get", "", "pods", "ns-a")
	assert.Equal(t, authorizer.DecisionNoOpinion, got)
	got, _ = authorizeAs(t, e, u, "get", "", "pods", "ns-b")
	assert.Equal(t, authorizer.DecisionNoOpinion, got)
	got, reason := authorizeAs(t, e, u, "get", "", "pods", "ns-out")
	assert.Equal(t, authorizer.DecisionDeny, got)
	assert.Equal(t, noNamespaceAccessReason, reason)
	got, reason = authorizeAs(t, e, u, "list", "", "pods", "")
	assert.Equal(t, authorizer.DecisionDeny, got)
	assert.Equal(t, namespaceLimitedAccessReason, reason)
}

func TestEngine_Authorize_MatchAnyGroupOverridesUserLimit(t *testing.T) {
	e := engineWithConsoleScope(t, `{
		"crds": [
			{
				"name": "user-limit",
				"spec": {
					"limitNamespaces": ["ns-in"],
					"subjects": [{"kind": "User", "name": "union@example.io"}]
				}
			},
			{
				"name": "ops-any",
				"spec": {
					"namespaceSelector": {"matchAny": true},
					"subjects": [{"kind": "Group", "name": "ops"}]
				}
			}
		]
	}`)
	userOnly := &mockUserInfo{name: "union@example.io"}
	got, _ := authorizeAs(t, e, userOnly, "list", "", "pods", "")
	assert.Equal(t, authorizer.DecisionDeny, got)

	withOps := &mockUserInfo{name: "union@example.io", groups: []string{"ops"}}
	got, reason := authorizeAs(t, e, withOps, "list", "", "pods", "")
	assert.Equal(t, authorizer.DecisionNoOpinion, got, "reason=%q", reason)
	got, _ = authorizeAs(t, e, withOps, "get", "", "pods", "kube-system")
	assert.Equal(t, authorizer.DecisionNoOpinion, got)
}

func TestEngine_Authorize_SelectorIgnoresSiblingLimitNamespaces(t *testing.T) {
	e := engineWithNamespaces(t, `{
		"crds": [{
			"name": "both",
			"spec": {
				"limitNamespaces": ["ns-out"],
				"namespaceSelector": {"labelSelector": {"matchLabels": {"pba-perf": "true"}}},
				"subjects": [{"kind": "User", "name": "editor@example.io"}]
			}
		}]
	}`, labeledNamespaces()...)
	u := &mockUserInfo{name: "editor@example.io"}
	got, _ := authorizeAs(t, e, u, "get", "", "pods", "ns-in")
	assert.Equal(t, authorizer.DecisionNoOpinion, got)
	got, _ = authorizeAs(t, e, u, "get", "", "pods", "ns-out")
	assert.Equal(t, authorizer.DecisionDeny, got,
		"limitNamespaces must be ignored when the same CAR sets namespaceSelector")
}

func TestEngine_Authorize_LimitNamespacesRegex(t *testing.T) {
	e := engineWithConsoleScope(t, `{
		"crds": [{
			"name": "re",
			"spec": {
				"limitNamespaces": ["app-.*"],
				"subjects": [{"kind": "User", "name": "regex@example.io"}]
			}
		}]
	}`)
	u := &mockUserInfo{name: "regex@example.io"}
	got, _ := authorizeAs(t, e, u, "get", "", "pods", "app-prod")
	assert.Equal(t, authorizer.DecisionNoOpinion, got)
	got, _ = authorizeAs(t, e, u, "get", "", "pods", "ns-in")
	assert.Equal(t, authorizer.DecisionDeny, got)
}

func TestEngine_Authorize_MatchExpressionsEqualsMatchLabels(t *testing.T) {
	labelsEngine := engineWithNamespaces(t, editorLabelSelectorCARConfig, labeledNamespaces()...)
	exprEngine := engineWithNamespaces(t, `{
		"crds": [{
			"name": "editor-expr",
			"spec": {
				"accessLevel": "Editor",
				"namespaceSelector": {"labelSelector": {"matchExpressions": [{"key": "pba-perf", "operator": "In", "values": ["true"]}]}},
				"subjects": [{"kind": "User", "name": "editor@example.io"}]
			}
		}]
	}`, labeledNamespaces()...)
	u := &mockUserInfo{name: "editor@example.io"}
	for _, ns := range []string{"ns-in", "ns-out", "kube-system"} {
		gotL, reasonL := authorizeAs(t, labelsEngine, u, "get", "", "pods", ns)
		gotE, reasonE := authorizeAs(t, exprEngine, u, "get", "", "pods", ns)
		assert.Equal(t, gotL, gotE, "ns=%s", ns)
		assert.Equal(t, reasonL, reasonE, "ns=%s", ns)
	}
}

func TestEngine_Authorize_SystemNamespaceSet(t *testing.T) {
	editor := engineWithConsoleScope(t, editorCARConfig)
	u := &mockUserInfo{name: "editor@example.io"}
	for _, ns := range []string{"default", "kube-system", "kube-public", "d8-system", "d8-monitoring"} {
		got, reason := authorizeAs(t, editor, u, "get", "", "pods", ns)
		assert.Equal(t, authorizer.DecisionDeny, got, "ns=%s", ns)
		assert.Equal(t, noNamespaceAccessReason, reason, "ns=%s", ns)
	}
}

func TestEngine_Authorize_LimitDefaultStillDeniedWithoutSystemAccess(t *testing.T) {
	e := engineWithConsoleScope(t, `{
		"crds": [{
			"name": "def",
			"spec": {
				"limitNamespaces": ["default"],
				"subjects": [{"kind": "User", "name": "def@example.io"}]
			}
		}]
	}`)
	got, reason := authorizeAs(t, e, &mockUserInfo{name: "def@example.io"}, "get", "", "pods", "default")
	assert.Equal(t, authorizer.DecisionDeny, got)
	assert.Equal(t, noNamespaceAccessReason, reason,
		"listing default in limitNamespaces does not grant a system namespace")
}

func TestEngine_Authorize_MatchAnyWithoutSystemAccessIsNoOpinion(t *testing.T) {
	e := engineWithConsoleScope(t, `{
		"crds": [{
			"name": "any",
			"spec": {
				"accessLevel": "Editor",
				"namespaceSelector": {"matchAny": true},
				"subjects": [{"kind": "User", "name": "any@example.io"}]
			}
		}]
	}`)
	u := &mockUserInfo{name: "any@example.io"}
	got, _ := authorizeAs(t, e, u, "list", "", "pods", "")
	assert.Equal(t, authorizer.DecisionNoOpinion, got)
	got, _ = authorizeAs(t, e, u, "get", "", "pods", "kube-system")
	assert.Equal(t, authorizer.DecisionNoOpinion, got,
		"MatchAny short-circuits hasAnyFilters even without allowAccessToSystemNamespaces")
}

func TestEngine_Authorize_VerbsDoNotChangeClusterScopedFilter(t *testing.T) {
	editor := engineWithConsoleScope(t, editorCARConfig)
	u := &mockUserInfo{name: "editor@example.io"}
	for _, verb := range []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"} {
		got, reason := authorizeAs(t, editor, u, verb, "", "pods", "")
		assert.Equal(t, authorizer.DecisionDeny, got, "verb=%s", verb)
		assert.Equal(t, namespaceLimitedAccessReason, reason, "verb=%s", verb)
		got, _ = authorizeAs(t, editor, u, verb, "", "nodes", "")
		assert.Equal(t, authorizer.DecisionNoOpinion, got, "verb=%s nodes", verb)
	}
}

func TestEngine_GetNamespaceAccessType_MatchAnyVsLimited(t *testing.T) {
	any := engineWithConsoleScope(t, editorMatchAnyCARConfig)
	limited := engineWithConsoleScope(t, editorCARConfig)
	superLimited := engineWithConsoleScope(t, superAdminLimitedCARConfig)

	access, filter := any.GetNamespaceAccessType(&mockUserInfo{name: "editor-any@example.io"})
	assert.Equal(t, AllNamespacesAllowed, access)
	assert.Nil(t, filter)

	access, filter = limited.GetNamespaceAccessType(&mockUserInfo{name: "editor@example.io"})
	assert.Equal(t, FilteredAccess, access)
	require.NotNil(t, filter)

	access, filter = superLimited.GetNamespaceAccessType(&mockUserInfo{name: "super-limited@example.io"})
	assert.Equal(t, FilteredAccess, access,
		"SuperAdmin + limitNamespaces is still FilteredAccess")
	require.NotNil(t, filter)
	assert.True(t, superLimited.IsNamespaceAllowedWithFilter("ns-in", filter))
	assert.False(t, superLimited.IsNamespaceAllowedWithFilter("ns-out", filter))
	assert.False(t, superLimited.IsNamespaceAllowedWithFilter("kube-system", filter),
		"allowAccessToSystemNamespaces does not add a namespace that is outside limitNamespaces")
}
