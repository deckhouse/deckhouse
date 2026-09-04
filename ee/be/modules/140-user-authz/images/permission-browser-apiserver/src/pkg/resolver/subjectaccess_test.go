/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
	"permission-browser-apiserver/pkg/authorizer/multitenancy"
)

func setupSubjectAccessResolver(t *testing.T, objs []runtime.Object, mtEngine *multitenancy.Engine) *SubjectAccessResolver {
	t.Helper()

	client := fake.NewSimpleClientset(objs...)
	informerFactory := informers.NewSharedInformerFactory(client, 0)

	// Listers must be requested before Start so the informers get registered.
	rbacInformers := informerFactory.Rbac().V1()
	roleLister := rbacInformers.Roles().Lister()
	roleBindingLister := rbacInformers.RoleBindings().Lister()
	clusterRoleLister := rbacInformers.ClusterRoles().Lister()
	clusterRoleBindingLister := rbacInformers.ClusterRoleBindings().Lister()

	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })

	informerFactory.Start(stopCh)
	for informerType, ok := range informerFactory.WaitForCacheSync(stopCh) {
		if !ok {
			t.Fatalf("informer %v failed to sync", informerType)
		}
	}

	return NewSubjectAccessResolver(
		roleLister,
		roleBindingLister,
		clusterRoleLister,
		clusterRoleBindingLister,
		newTestScopeCache(),
		mtEngine,
		nil,
	)
}

func clusterRole(name string, labels map[string]string, rules ...rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Rules:      rules,
	}
}

func clusterRoleBinding(name, roleName string, subjects ...rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: roleName},
		Subjects:   subjects,
	}
}

func roleBinding(name, namespace, roleKind, roleName string, subjects ...rbacv1.Subject) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: roleKind, Name: roleName},
		Subjects:   subjects,
	}
}

func rule(apiGroups, resources, verbs []string) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{APIGroups: apiGroups, Resources: resources, Verbs: verbs}
}

func userRequest(name string, groups ...string) SubjectAccessRequest {
	return SubjectAccessRequest{
		Subject:         v1alpha1.SubjectReference{Kind: v1alpha1.SubjectKindUser, Name: name},
		ExtraGroups:     groups,
		ExpandWildcards: true,
	}
}

// findScope returns the cluster-wide scope or the scope covering the namespace.
func findScope(t *testing.T, status v1alpha1.SubjectAccessReportStatus, namespace string) v1alpha1.AccessScope {
	t.Helper()

	for _, scope := range status.Scopes {
		if namespace == "" && scope.Cluster {
			return scope
		}
		for _, candidate := range scope.Namespaces {
			if candidate == namespace {
				return scope
			}
		}
	}

	t.Fatalf("no scope for namespace %q in %+v", namespace, status.Scopes)

	return v1alpha1.AccessScope{}
}

func findRow(scope v1alpha1.AccessScope, group, resource string) (v1alpha1.ResourceAccess, bool) {
	for _, row := range scope.Resources {
		if row.Group == group && row.Resource == resource {
			return row, true
		}
	}

	return v1alpha1.ResourceAccess{}, false
}

func TestReport_ClusterAndNamespaceScopes(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("cluster-reader", nil, rule([]string{""}, []string{"pods"}, []string{"get", "list"})),
		clusterRoleBinding("cluster-reader-binding", "cluster-reader", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
		clusterRole("ns-editor", nil, rule([]string{"apps"}, []string{"deployments"}, []string{"create", "update"})),
		roleBinding("ns-editor-binding", "team-a", "ClusterRole", "ns-editor", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	status, err := resolver.Report(context.Background(), userRequest("alice"))
	require.NoError(t, err)
	require.Empty(t, status.EvaluationError)

	cluster := findScope(t, status, "")
	row, ok := findRow(cluster, "", "pods")
	require.True(t, ok, "cluster scope must contain pods")
	assert.Equal(t, []string{"get", "list"}, row.Verbs)

	// Cluster-wide grants are reported once, not repeated per namespace.
	_, leaked := findRow(findScope(t, status, "team-a"), "", "pods")
	assert.False(t, leaked, "namespace scope must show only local access")

	namespaced := findScope(t, status, "team-a")
	deployments, ok := findRow(namespaced, "apps", "deployments")
	require.True(t, ok)
	assert.Equal(t, []string{"create", "update"}, deployments.Verbs)
	assert.Equal(t, []string{"team-a"}, namespaced.Namespaces)
}

func TestReport_ProvenanceDistinguishesDirectAndGroupGrants(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("direct", nil, rule([]string{""}, []string{"pods"}, []string{"get"})),
		clusterRoleBinding("direct-binding", "direct", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
		clusterRole("through-group", nil, rule([]string{""}, []string{"pods"}, []string{"delete"})),
		clusterRoleBinding("group-binding", "through-group", rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "netops"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	status, err := resolver.Report(context.Background(), userRequest("alice", "netops"))
	require.NoError(t, err)

	row, ok := findRow(findScope(t, status, ""), "", "pods")
	require.True(t, ok)
	assert.Equal(t, []string{"get", "delete"}, row.Verbs)

	byMatch := map[string][]string{}
	for _, source := range row.Sources {
		byMatch[source.MatchedBy.Kind+"/"+source.MatchedBy.Name] = source.Verbs
	}

	// The whole point of the provenance: a client can drop group-derived grants
	// without asking the server again.
	assert.Equal(t, []string{"get"}, byMatch["User/alice"])
	assert.Equal(t, []string{"delete"}, byMatch["Group/netops"])
}

func TestReport_GroupSubjectIgnoresUserBindings(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("for-user", nil, rule([]string{""}, []string{"secrets"}, []string{"get"})),
		clusterRoleBinding("user-binding", "for-user", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "netops"}),
		clusterRole("for-group", nil, rule([]string{""}, []string{"configmaps"}, []string{"get"})),
		clusterRoleBinding("group-binding", "for-group", rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "netops"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	status, err := resolver.Report(context.Background(), SubjectAccessRequest{
		Subject:         v1alpha1.SubjectReference{Kind: v1alpha1.SubjectKindGroup, Name: "netops"},
		ExpandWildcards: true,
	})
	require.NoError(t, err)

	cluster := findScope(t, status, "")
	_, hasConfigMaps := findRow(cluster, "", "configmaps")
	assert.True(t, hasConfigMaps, "group binding must apply")

	// A User subject that happens to share the group's name must not leak in.
	_, hasSecrets := findRow(cluster, "", "secrets")
	assert.False(t, hasSecrets, "user binding must not apply to a group subject")
}

func TestReport_ServiceAccountSubject(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("sa-role", nil, rule([]string{""}, []string{"pods"}, []string{"get"})),
		// A ServiceAccount subject without a namespace defaults to the binding's.
		roleBinding("sa-binding", "team-a", "ClusterRole", "sa-role", rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "builder"}),
		clusterRole("all-sa", nil, rule([]string{""}, []string{"events"}, []string{"create"})),
		clusterRoleBinding("all-sa-binding", "all-sa", rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:team-a"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	status, err := resolver.Report(context.Background(), SubjectAccessRequest{
		Subject: v1alpha1.SubjectReference{
			Kind:      v1alpha1.SubjectKindServiceAccount,
			Name:      "builder",
			Namespace: "team-a",
		},
		ExpandWildcards: true,
	})
	require.NoError(t, err)

	_, ok := findRow(findScope(t, status, "team-a"), "", "pods")
	assert.True(t, ok, "RoleBinding to the SA must apply")

	_, ok = findRow(findScope(t, status, ""), "", "events")
	assert.True(t, ok, "grant to the SA pseudo-group must apply")

	groups := make([]string, 0, len(status.Subject.Groups))
	for _, group := range status.Subject.Groups {
		groups = append(groups, group.Name)
	}
	assert.Contains(t, groups, "system:serviceaccounts")
	assert.Contains(t, groups, "system:serviceaccounts:team-a")
	assert.Contains(t, groups, "system:authenticated")
}

func TestReport_WildcardExpansion(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("admin", nil, rule([]string{"*"}, []string{"*"}, []string{"*"})),
		clusterRoleBinding("admin-binding", "admin", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "root"}),
		roleBinding("ns-admin-binding", "team-a", "ClusterRole", "admin", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "root"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	status, err := resolver.Report(context.Background(), userRequest("root"))
	require.NoError(t, err)

	cluster := findScope(t, status, "")
	pods, ok := findRow(cluster, "", "pods")
	require.True(t, ok, "wildcard must expand to concrete resources")
	assert.True(t, pods.ViaWildcard)
	assert.Equal(t, standardVerbs, pods.Verbs)

	_, hasNodes := findRow(cluster, "", "nodes")
	assert.True(t, hasNodes, "cluster scope covers cluster-scoped resources")

	// A RoleBinding cannot grant cluster-scoped resources, so they must not be
	// reported as if it did.
	namespaced := findScope(t, status, "team-a")
	_, nodesInNamespace := findRow(namespaced, "", "nodes")
	assert.False(t, nodesInNamespace, "namespace scope must stay namespaced")
	_, podsInNamespace := findRow(namespaced, "", "pods")
	assert.True(t, podsInNamespace)
}

func TestReport_WildcardKeptWhenExpansionDisabled(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("admin", nil, rule([]string{"*"}, []string{"*"}, []string{"*"})),
		clusterRoleBinding("admin-binding", "admin", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "root"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	request := userRequest("root")
	request.ExpandWildcards = false

	status, err := resolver.Report(context.Background(), request)
	require.NoError(t, err)

	cluster := findScope(t, status, "")
	require.Len(t, cluster.Resources, 1)
	assert.Equal(t, "*", cluster.Resources[0].Resource)
	assert.Equal(t, "*", cluster.Resources[0].Group)
}

func TestReport_NamespacesWithIdenticalAccessAreMerged(t *testing.T) {
	viewer := clusterRole("viewer", nil, rule([]string{""}, []string{"pods"}, []string{"get"}))
	objs := []runtime.Object{
		viewer,
		roleBinding("viewer-binding", "team-a", "ClusterRole", "viewer", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
		roleBinding("viewer-binding", "team-b", "ClusterRole", "viewer", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
		clusterRole("editor", nil, rule([]string{""}, []string{"pods"}, []string{"get", "delete"})),
		roleBinding("editor-binding", "team-c", "ClusterRole", "editor", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	status, err := resolver.Report(context.Background(), userRequest("alice"))
	require.NoError(t, err)

	merged := findScope(t, status, "team-a")
	assert.Equal(t, []string{"team-a", "team-b"}, merged.Namespaces, "identical access must collapse into one scope")

	separate := findScope(t, status, "team-c")
	assert.Equal(t, []string{"team-c"}, separate.Namespaces)
}

func TestReport_SuperadminCaveat(t *testing.T) {
	superadminLabels := map[string]string{
		labelRoleKind:  "role",
		labelRoleScope: "namespace",
	}
	objs := []runtime.Object{
		clusterRole("d8:namespace:superadmin", superadminLabels, rule([]string{""}, []string{"pods"}, []string{"get", "delete"})),
		clusterRole("d8:namespace:admin", map[string]string{labelRoleKind: "role", labelRoleScope: "namespace"},
			rule([]string{""}, []string{"pods"}, []string{"get", "delete"})),
		roleBinding("super", "team-a", "ClusterRole", "d8:namespace:superadmin", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
		roleBinding("admin", "team-b", "ClusterRole", "d8:namespace:admin", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	status, err := resolver.Report(context.Background(), userRequest("alice"))
	require.NoError(t, err)

	superScope := findScope(t, status, "team-a")
	assert.True(t, superScope.Caveat.ProtectedVerbs)
	assert.True(t, superScope.Caveat.Superadmin)
	assert.Equal(t, []string{"team-a"}, superScope.Caveat.SuperadminNamespaces)
	assert.Empty(t, superScope.Caveat.RestrictedNamespaces)

	adminScope := findScope(t, status, "team-b")
	assert.True(t, adminScope.Caveat.ProtectedVerbs)
	assert.False(t, adminScope.Caveat.Superadmin)
	assert.Equal(t, []string{"team-b"}, adminScope.Caveat.RestrictedNamespaces)
}

// The webhook drops its checks for the administrator groups before it looks at any binding, so a
// member of system:masters is unrestricted no matter which roles they hold -- including none.
// Reporting a restriction the webhook never applies is the failure direction that matters here.
func TestReport_BypassGroupIsSuperadminEverywhere(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("d8:namespace:admin", map[string]string{labelRoleKind: "role", labelRoleScope: "namespace"},
			rule([]string{""}, []string{"pods"}, []string{"get", "delete"})),
		roleBinding("admin", "team-b", "ClusterRole", "d8:namespace:admin", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	status, err := resolver.Report(context.Background(), userRequest("alice", "system:masters"))
	require.NoError(t, err)

	scope := findScope(t, status, "team-b")
	assert.True(t, scope.Caveat.Superadmin)
	assert.Empty(t, scope.Caveat.RestrictedNamespaces)
}

// Discovery goes incomplete whenever an aggregated APIService flaps, and the snapshot then answers
// "unknown" for whole API groups. Unknown must not read as cluster-scoped here: dropping those rows
// would quietly shrink the report, and the caller has no way to tell an omission from a real absence.
func TestReport_UnknownResourceSurvivesAPartialSnapshot(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("widget-reader", nil,
			rule([]string{"flapping.example.com"}, []string{"widgets"}, []string{"get"}),
			rule([]string{""}, []string{"nodes"}, []string{"get"})),
		roleBinding("widgets", "team-a", "ClusterRole", "widget-reader", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)
	resolver.scopeCache.mu.Lock()
	resolver.scopeCache.partial = true
	resolver.scopeCache.mu.Unlock()

	status, err := resolver.Report(context.Background(), userRequest("alice"))
	require.NoError(t, err)

	scope := findScope(t, status, "team-a")
	assert.True(t, hasResource(scope, "flapping.example.com", "widgets"), "resource unknown to discovery was dropped")
	// nodes is positively known to be cluster-scoped, so a RoleBinding grant on it stays unreportable.
	assert.False(t, hasResource(scope, "", "nodes"), "cluster-scoped resource leaked into a namespace scope")

	assert.Contains(t, status.Notes, "the discovery snapshot is incomplete: wildcard rules may have been expanded to fewer resources than they actually grant")
}

// A subresource is judged by its base resource. Discovery lists no subresources at all -- every name
// with a slash is skipped -- so asking the snapshot about "nodes/proxy" answers "unknown" forever,
// and the row would survive although it can no more be exercised in a namespace than "nodes" itself.
func TestReport_SubresourceIsJudgedByItsBaseResource(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("prober", nil,
			rule([]string{""}, []string{"nodes/proxy"}, []string{"get"}),
			rule([]string{""}, []string{"pods/log"}, []string{"get"}),
			rule([]string{"flapping.example.com"}, []string{"widgets/status"}, []string{"get"})),
		roleBinding("probe", "team-a", "ClusterRole", "prober", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
	}

	status, err := setupSubjectAccessResolver(t, objs, nil).Report(context.Background(), userRequest("alice"))
	require.NoError(t, err)

	scope := findScope(t, status, "team-a")
	assert.False(t, hasResource(scope, "", "nodes/proxy"), "a subresource of a cluster-scoped resource is not exercisable in a namespace")
	assert.True(t, hasResource(scope, "", "pods/log"), "a subresource of a namespaced resource was dropped")
	// The base is unknown to the snapshot, so the row is kept -- an audit report over-reports rather
	// than omits.
	assert.True(t, hasResource(scope, "flapping.example.com", "widgets/status"), "a subresource of an unknown resource was dropped")
}

func hasResource(scope v1alpha1.AccessScope, group, resource string) bool {
	for _, access := range scope.Resources {
		if access.Group == group && access.Resource == resource {
			return true
		}
	}

	return false
}

func TestReport_RoleDescriptorIsAttached(t *testing.T) {
	role := clusterRole("d8:subsystem:networking:manager", map[string]string{
		labelRoleKind:  "role",
		labelRoleScope: "subsystem",
		labelSubsystem: "networking",
	}, rule([]string{""}, []string{"pods"}, []string{"get"}))
	role.Annotations = map[string]string{
		"en.meta.deckhouse.io/title": "Networking Manager",
		"ru.meta.deckhouse.io/title": "Networking Manager [ru]",
	}

	objs := []runtime.Object{
		role,
		clusterRoleBinding("net-binding", "d8:subsystem:networking:manager", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	status, err := resolver.Report(context.Background(), userRequest("alice"))
	require.NoError(t, err)

	require.Len(t, status.RoleAssignments, 1)
	descriptor := status.RoleAssignments[0].Role
	assert.Equal(t, "subsystem", descriptor.Scope)
	assert.Equal(t, "networking", descriptor.Subsystem)
	assert.Equal(t, "manager", descriptor.Level)
	assert.Equal(t, "Networking Manager", descriptor.Titles["en"])
	assert.Equal(t, "Networking Manager [ru]", descriptor.Titles["ru"])
}

func TestReport_ResourceNamesAndSubresources(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("named", nil,
			rbacv1.PolicyRule{
				APIGroups:     []string{""},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{"settings"},
				Verbs:         []string{"get"},
			},
			rule([]string{""}, []string{"pods/log"}, []string{"get"}),
		),
		roleBinding("named-binding", "team-a", "ClusterRole", "named", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	status, err := resolver.Report(context.Background(), userRequest("alice"))
	require.NoError(t, err)

	scope := findScope(t, status, "team-a")

	configmaps, ok := findRow(scope, "", "configmaps")
	require.True(t, ok)
	assert.Equal(t, []string{"settings"}, configmaps.ResourceNames)

	logs, ok := findRow(scope, "", "pods/log")
	require.True(t, ok, "explicitly named subresources must be reported")
	assert.Equal(t, []string{"get"}, logs.Verbs)
}

func TestReport_NonResourceURLsOnlyFromClusterBindings(t *testing.T) {
	nonResource := rbacv1.PolicyRule{NonResourceURLs: []string{"/healthz"}, Verbs: []string{"get"}}
	objs := []runtime.Object{
		clusterRole("health", nil, nonResource),
		clusterRoleBinding("health-binding", "health", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
		clusterRole("health-ns", nil, nonResource),
		roleBinding("health-ns-binding", "team-a", "ClusterRole", "health-ns", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	status, err := resolver.Report(context.Background(), userRequest("alice"))
	require.NoError(t, err)

	cluster := findScope(t, status, "")
	require.Len(t, cluster.NonResourceRules, 1)
	assert.Equal(t, "/healthz", cluster.NonResourceRules[0].Path)

	// A RoleBinding never grants non-resource access, so the namespace scope
	// stays empty and is not reported at all.
	for _, scope := range status.Scopes {
		if !scope.Cluster {
			assert.Empty(t, scope.NonResourceRules)
		}
	}
}

func TestReport_NamespaceFilter(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("viewer", nil, rule([]string{""}, []string{"pods"}, []string{"get"})),
		roleBinding("a", "team-a", "ClusterRole", "viewer", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
		roleBinding("b", "team-b", "ClusterRole", "viewer", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	request := userRequest("alice")
	request.Namespaces = []string{"team-b"}

	status, err := resolver.Report(context.Background(), request)
	require.NoError(t, err)

	for _, scope := range status.Scopes {
		assert.NotContains(t, scope.Namespaces, "team-a")
	}
	findScope(t, status, "team-b")
}

func TestReport_DanglingRoleRefIsSkipped(t *testing.T) {
	objs := []runtime.Object{
		clusterRoleBinding("dangling", "missing-role", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	status, err := resolver.Report(context.Background(), userRequest("alice"))
	require.NoError(t, err)
	assert.Empty(t, status.EvaluationError, "a dangling reference is expected, not an evaluation failure")
	assert.Empty(t, status.Scopes)
}

func TestReport_TruncatedWhenLimitsHit(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("admin", nil, rule([]string{"*"}, []string{"*"}, []string{"get"})),
		clusterRoleBinding("admin-binding", "admin", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "root"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)
	resolver.limits = ReportLimits{MaxNamespaces: 10, MaxResourceRows: 3, MaxSourcesPerRow: 10}

	status, err := resolver.Report(context.Background(), userRequest("root"))
	require.NoError(t, err)

	assert.True(t, status.Truncated)
	assert.Len(t, findScope(t, status, "").Resources, 3)
}

func TestReport_CARNarrowingIsReported(t *testing.T) {
	// A ClusterRoleBinding rendered from a ClusterAuthorizationRule is
	// cluster-wide in RBAC but limited by the rule.
	carBinding := clusterRoleBinding("user-authz:limited:user", "car-role", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"})
	carBinding.Labels = map[string]string{"heritage": "deckhouse", "module": "user-authz"}

	objs := []runtime.Object{
		clusterRole("car-role", nil, rule([]string{""}, []string{"pods"}, []string{"get"})),
		carBinding,
		clusterRole("local", nil, rule([]string{""}, []string{"secrets"}, []string{"get"})),
		roleBinding("local-binding", "team-a", "ClusterRole", "local", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
	}

	engine := newMTEngineFromConfig(t, `{"crds":[{"name":"limited","spec":{"accessLevel":"User","limitNamespaces":["^allowed-.*$"],"subjects":[{"kind":"User","name":"alice"}]}}]}`)
	resolver := setupSubjectAccessResolver(t, objs, engine)

	status, err := resolver.Report(context.Background(), userRequest("alice"))
	require.NoError(t, err)

	require.NotEmpty(t, status.Notes)
	assert.Contains(t, status.Notes[0], "ClusterAuthorizationRule")
	assert.Contains(t, status.Notes[0], "allowed-")

	// The RoleBinding grant exists independently of the rule and must survive.
	_, ok := findRow(findScope(t, status, "team-a"), "", "secrets")
	assert.True(t, ok, "CAR-independent access must not be filtered out")
}

func TestReport_ContextCancellation(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("viewer", nil, rule([]string{""}, []string{"pods"}, []string{"get"})),
		clusterRoleBinding("viewer-binding", "viewer", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := resolver.Report(ctx, userRequest("alice"))
	require.ErrorIs(t, err, context.Canceled)
}

func TestExpandVerbs(t *testing.T) {
	tests := []struct {
		name         string
		verbs        []string
		expected     []string
		wildcardFlag bool
	}{
		{
			name:  "wildcard expands to the standard set",
			verbs: []string{"*"},
			// The standard set is a readable sample, not the whole truth: the same rule also grants
			// escalate, bind, impersonate and proxy. The flag is what keeps that from being silent.
			expected:     standardVerbs,
			wildcardFlag: true,
		},
		{
			name:     "explicit verbs keep the reading order",
			verbs:    []string{"delete", "get", "list"},
			expected: []string{"get", "list", "delete"},
		},
		{
			name:     "non-standard verbs are preserved after the standard ones",
			verbs:    []string{"impersonate", "get", "bind"},
			expected: []string{"get", "bind", "impersonate"},
		},
		{
			name:         "duplicates collapse",
			verbs:        []string{"get", "get", "*"},
			expected:     standardVerbs,
			wildcardFlag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verbs, viaWildcard := expandVerbs(tt.verbs)
			assert.Equal(t, tt.expected, verbs)
			assert.Equal(t, tt.wildcardFlag, viaWildcard)
		})
	}
}

// An eight-verb row from verbs: ["*"] must not read like an explicit eight-verb grant: the wildcard
// also covers escalate, bind, impersonate and proxy, and an audit is usually run for exactly those.
func TestReport_VerbWildcardIsMarked(t *testing.T) {
	objs := []runtime.Object{
		clusterRole("all-verbs", nil, rule([]string{"rbac.authorization.k8s.io"}, []string{"clusterroles"}, []string{"*"})),
		clusterRole("named-verbs", nil, rule([]string{""}, []string{"pods"}, []string{"get", "list"})),
		clusterRoleBinding("all", "all-verbs", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
		clusterRoleBinding("named", "named-verbs", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
	}

	resolver := setupSubjectAccessResolver(t, objs, nil)

	status, err := resolver.Report(context.Background(), userRequest("alice"))
	require.NoError(t, err)

	scope := findScope(t, status, "")
	for _, access := range scope.Resources {
		switch access.Resource {
		case "clusterroles":
			assert.True(t, access.ViaVerbWildcard)
			require.NotEmpty(t, access.Sources)
			assert.True(t, access.Sources[0].ViaVerbWildcard)
		case "pods":
			assert.False(t, access.ViaVerbWildcard)
		}
	}
}
