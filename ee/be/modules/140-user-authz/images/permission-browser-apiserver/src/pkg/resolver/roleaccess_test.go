/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
)

func setupRoleAccessResolver(t *testing.T, objs []runtime.Object) *RoleAccessResolver {
	t.Helper()

	client := fake.NewSimpleClientset(objs...)
	informerFactory := informers.NewSharedInformerFactory(client, 0)

	// The lister must be requested before Start so the informer gets registered.
	clusterRoleLister := informerFactory.Rbac().V1().ClusterRoles().Lister()

	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })

	informerFactory.Start(stopCh)
	for informerType, ok := range informerFactory.WaitForCacheSync(stopCh) {
		if !ok {
			t.Fatalf("informer %v failed to sync", informerType)
		}
	}

	resolver := NewRoleAccessResolver(clusterRoleLister, newTestScopeCache())
	// A fixed clock, so a test can assert that two reports of the same cluster
	// are identical without the timestamp getting in the way.
	resolver.now = func() time.Time { return time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC) }

	return resolver
}

// capability builds a ClusterRole the role model calls a capability.
func capability(name, scope string, rules ...rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				labelRoleKind:  "capability",
				labelRoleScope: scope,
				"rbac.deckhouse.io/aggregate-to-" + scope + "-as": "admin",
			},
			Annotations: map[string]string{
				"en.meta.deckhouse.io/title": name + " (en)",
				"ru.meta.deckhouse.io/title": name + " (ru)",
			},
		},
		Rules: rules,
	}
}

// aggregatingRole builds a ClusterRole that collects capabilities of a scope.
func aggregatingRole(name, scope string, rules ...rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				labelRoleKind:  "role",
				labelRoleScope: scope,
			},
		},
		AggregationRule: &rbacv1.AggregationRule{
			ClusterRoleSelectors: []metav1.LabelSelector{
				{MatchLabels: map[string]string{"rbac.deckhouse.io/aggregate-to-" + scope + "-as": "admin"}},
			},
		},
		Rules: rules,
	}
}

func policyRule(group, resource string, verbs ...string) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{APIGroups: []string{group}, Resources: []string{resource}, Verbs: verbs}
}

func rowOf(t *testing.T, role v1alpha1.RoleAccess, group, resource string) v1alpha1.ResourceAccess {
	t.Helper()

	for _, row := range role.Resources {
		if row.Group == group && row.Resource == resource {
			return row
		}
	}

	t.Fatalf("role %q has no row for %s/%s", role.Name, group, resource)

	return v1alpha1.ResourceAccess{}
}

func roleOf(t *testing.T, status v1alpha1.RoleAccessReportStatus, name string) v1alpha1.RoleAccess {
	t.Helper()

	for _, role := range status.Roles {
		if role.Name == name {
			return role
		}
	}

	t.Fatalf("report has no role %q", name)

	return v1alpha1.RoleAccess{}
}

// A role holds no rules of its own: the rows have to come from the capabilities
// it aggregates, or the catalogue would report every role as granting nothing.
func TestRoleReport_RowsComeFromTheAggregatedCapabilities(t *testing.T) {
	t.Parallel()

	objs := []runtime.Object{
		aggregatingRole("d8:namespace:admin", "namespace"),
		capability("d8:namespace-capability:kubernetes:manage_workloads", "namespace",
			policyRule("apps", "deployments", "get", "list", "create")),
		capability("d8:namespace-capability:kubernetes:view_logs", "namespace",
			policyRule("", "pods/log", "get")),
	}

	status, err := setupRoleAccessResolver(t, objs).Report(context.Background(), RoleAccessRequest{ExpandWildcards: true})
	require.NoError(t, err)
	require.Len(t, status.Roles, 1)

	role := status.Roles[0]
	assert.Equal(t, "d8:namespace:admin", role.Name)
	assert.True(t, role.Namespaced, "a namespace-scoped role applies inside a namespace")

	deployments := rowOf(t, role, "apps", "deployments")
	assert.Equal(t, []string{"get", "list", "create"}, deployments.Verbs)

	logs := rowOf(t, role, "", "pods/log")
	assert.Equal(t, []string{"get"}, logs.Verbs)
}

// The simple mode is the plain matrix; the detailed one has to say which
// capability granted a row, which is the whole reason to keep the composition.
func TestRoleReport_CompositionNamesTheCapabilityBehindEachRow(t *testing.T) {
	t.Parallel()

	objs := []runtime.Object{
		aggregatingRole("d8:namespace:admin", "namespace"),
		capability("d8:namespace-capability:kubernetes:manage_workloads", "namespace",
			policyRule("apps", "deployments", "get")),
	}

	resolver := setupRoleAccessResolver(t, objs)

	plain, err := resolver.Report(context.Background(), RoleAccessRequest{ExpandWildcards: true})
	require.NoError(t, err)
	assert.Empty(t, plain.Roles[0].Composition, "the plain matrix carries no composition")
	assert.Empty(t, rowOf(t, plain.Roles[0], "apps", "deployments").Sources, "nor per-row sources")

	detailed, err := resolver.Report(context.Background(), RoleAccessRequest{ExpandWildcards: true, IncludeComposition: true})
	require.NoError(t, err)

	role := detailed.Roles[0]
	require.Len(t, role.Composition, 1)
	assert.Equal(t, "d8:namespace-capability:kubernetes:manage_workloads", role.Composition[0].Name)
	assert.Equal(t, "d8:namespace-capability:kubernetes:manage_workloads (ru)", role.Composition[0].Role.Titles["ru"])

	sources := rowOf(t, role, "apps", "deployments").Sources
	require.Len(t, sources, 1)
	assert.Equal(t, "d8:namespace-capability:kubernetes:manage_workloads", sources[0].RoleName)
	// A role grants nothing until something binds it, so naming a binding here
	// would be an invention.
	assert.Empty(t, sources[0].BindingKind)
	assert.Empty(t, sources[0].BindingName)
	assert.Empty(t, sources[0].MatchedBy.Name)
}

// A namespace-scoped role cannot be bound cluster-wide -- the binding webhook
// refuses it -- so a cluster-scoped resource in its rules can never be
// exercised. Listing it would overstate the access the document reports.
func TestRoleReport_NamespacedRoleDropsClusterScopedRows(t *testing.T) {
	t.Parallel()

	objs := []runtime.Object{
		aggregatingRole("d8:namespace:admin", "namespace"),
		capability("d8:namespace-capability:kubernetes:mixed", "namespace",
			policyRule("", "pods", "get"),
			policyRule("", "nodes", "get")),
		aggregatingRole("d8:system:manager", "system"),
		capability("d8:system-capability:kubernetes:nodes", "system",
			policyRule("", "nodes", "get")),
	}

	status, err := setupRoleAccessResolver(t, objs).Report(context.Background(), RoleAccessRequest{ExpandWildcards: true})
	require.NoError(t, err)

	namespaced := roleOf(t, status, "d8:namespace:admin")
	for _, row := range namespaced.Resources {
		assert.NotEqual(t, "nodes", row.Resource, "a namespace role must not report cluster-scoped access")
	}
	rowOf(t, namespaced, "", "pods")

	system := roleOf(t, status, "d8:system:manager")
	assert.False(t, system.Namespaced)
	rowOf(t, system, "", "nodes")
}

// The presence of a wildcard has to survive into the document: a reader must be
// able to tell an enumerated grant from one that was expanded for them.
func TestRoleReport_WildcardIsExpandedAndMarked(t *testing.T) {
	t.Parallel()

	objs := []runtime.Object{
		aggregatingRole("d8:system:superadmin", "system"),
		capability("d8:system-capability:kubernetes:everything", "system",
			rbacv1.PolicyRule{APIGroups: []string{"apps"}, Resources: []string{"*"}, Verbs: []string{"*"}}),
	}

	resolver := setupRoleAccessResolver(t, objs)

	expanded, err := resolver.Report(context.Background(), RoleAccessRequest{ExpandWildcards: true})
	require.NoError(t, err)

	deployments := rowOf(t, expanded.Roles[0], "apps", "deployments")
	assert.True(t, deployments.ViaWildcard, "an expanded row says it came from a wildcard")
	assert.True(t, deployments.ViaVerbWildcard, "and that its verbs are a sample")
	assert.Positive(t, expanded.Snapshot.DiscoveryResources, "the document says what it was expanded against")

	asWritten, err := resolver.Report(context.Background(), RoleAccessRequest{ExpandWildcards: false})
	require.NoError(t, err)
	rowOf(t, asWritten.Roles[0], "apps", "*")
}

// The legacy level is not a role: it is the base ClusterRole plus every module
// role annotated for this level or a lower one, which is what the fan-out hook
// binds. Getting the direction of that inclusion wrong would understate Admin.
func TestRoleReport_LegacyLevelIsCumulative(t *testing.T) {
	t.Parallel()

	annotated := func(name, level string, rules ...rbacv1.PolicyRule) *rbacv1.ClusterRole {
		return &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Annotations: map[string]string{annotationAccessLevel: level},
			},
			Rules: rules,
		}
	}

	objs := []runtime.Object{
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "user-authz:user"},
			Rules:      []rbacv1.PolicyRule{policyRule("", "pods", "get")},
		},
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "user-authz:admin"},
			Rules:      []rbacv1.PolicyRule{policyRule("", "secrets", "get")},
		},
		annotated("d8:user-authz:cert-manager:user", "User", policyRule("cert-manager.io", "certificates", "get")),
		annotated("d8:user-authz:cert-manager:admin", "Admin", policyRule("cert-manager.io", "issuers", "create")),
	}

	status, err := setupRoleAccessResolver(t, objs).Report(context.Background(), RoleAccessRequest{
		Model:              RoleModelLegacy,
		AccessLevels:       []string{"User", "Admin"},
		ExpandWildcards:    true,
		IncludeComposition: true,
	})
	require.NoError(t, err)
	require.Len(t, status.Roles, 2)
	assert.Equal(t, RoleModelLegacy, status.Snapshot.Model)

	user := roleOf(t, status, "User")
	rowOf(t, user, "cert-manager.io", "certificates")
	for _, row := range user.Resources {
		assert.NotEqual(t, "issuers", row.Resource, "User must not carry what only Admin is annotated for")
		assert.NotEqual(t, "secrets", row.Resource, "nor the base role of a higher level")
	}

	admin := roleOf(t, status, "Admin")
	rowOf(t, admin, "cert-manager.io", "issuers")
	rowOf(t, admin, "cert-manager.io", "certificates")
	rowOf(t, admin, "", "secrets")

	names := make([]string, 0, len(admin.Composition))
	for _, component := range admin.Composition {
		names = append(names, component.Name)
	}
	assert.Equal(t, []string{
		"d8:user-authz:cert-manager:admin",
		"d8:user-authz:cert-manager:user",
		"user-authz:admin",
	}, names)
}

// The digest is what makes two exports comparable. It must ignore the moment of
// capture and notice a changed rule; otherwise "nothing changed since last
// quarter" is not something a reader can check.
func TestRoleReport_DigestIsStableAndSensitive(t *testing.T) {
	t.Parallel()

	objs := []runtime.Object{
		aggregatingRole("d8:namespace:admin", "namespace"),
		capability("d8:namespace-capability:kubernetes:manage_workloads", "namespace",
			policyRule("apps", "deployments", "get")),
	}

	resolver := setupRoleAccessResolver(t, objs)

	first, err := resolver.Report(context.Background(), RoleAccessRequest{ExpandWildcards: true})
	require.NoError(t, err)

	resolver.now = func() time.Time { return time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC) }
	second, err := resolver.Report(context.Background(), RoleAccessRequest{ExpandWildcards: true})
	require.NoError(t, err)

	assert.NotEmpty(t, first.Snapshot.Digest)
	assert.Equal(t, first.Snapshot.Digest, second.Snapshot.Digest, "the timestamp must not enter the digest")

	widened := []runtime.Object{
		aggregatingRole("d8:namespace:admin", "namespace"),
		capability("d8:namespace-capability:kubernetes:manage_workloads", "namespace",
			policyRule("apps", "deployments", "get", "delete")),
	}
	changed, err := setupRoleAccessResolver(t, widened).Report(context.Background(), RoleAccessRequest{ExpandWildcards: true})
	require.NoError(t, err)

	assert.NotEqual(t, first.Snapshot.Digest, changed.Snapshot.Digest, "one more verb is a changed document")
}

// A document issued before the rename names the old role. Carrying the alias
// lets a reader line the two up instead of guessing.
func TestRoleReport_CarriesTheNameOfTheReplacedRole(t *testing.T) {
	t.Parallel()

	alias := func(name string) *rbacv1.ClusterRole {
		return &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Labels:      map[string]string{labelDeprecated: "true"},
				Annotations: map[string]string{annotationReplacedBy: "d8:namespace:admin"},
			},
		}
	}

	objs := []runtime.Object{
		aggregatingRole("d8:namespace:admin", "namespace"),
		capability("d8:namespace-capability:kubernetes:manage_workloads", "namespace", policyRule("apps", "deployments", "get")),
		// The rename folded the kubernetes-suffixed variant into the same role,
		// so one role carries two old names and an export must show both.
		alias("d8:use:role:admin"),
		alias("d8:use:role:admin:kubernetes"),
	}

	status, err := setupRoleAccessResolver(t, objs).Report(context.Background(), RoleAccessRequest{ExpandWildcards: true})
	require.NoError(t, err)

	assert.Equal(t, []string{"d8:use:role:admin", "d8:use:role:admin:kubernetes"}, roleOf(t, status, "d8:namespace:admin").LegacyNames)
}

// Selection is what keeps the console from downloading the whole catalogue to
// show one role.
func TestRoleReport_SelectionNarrowsTheReport(t *testing.T) {
	t.Parallel()

	objs := []runtime.Object{
		aggregatingRole("d8:namespace:admin", "namespace"),
		capability("d8:namespace-capability:kubernetes:workloads", "namespace", policyRule("apps", "deployments", "get")),
		aggregatingRole("d8:system:manager", "system"),
		capability("d8:system-capability:kubernetes:nodes", "system", policyRule("", "nodes", "get")),
	}

	resolver := setupRoleAccessResolver(t, objs)

	byName, err := resolver.Report(context.Background(), RoleAccessRequest{Names: []string{"d8:system:manager"}})
	require.NoError(t, err)
	require.Len(t, byName.Roles, 1)
	assert.Equal(t, "d8:system:manager", byName.Roles[0].Name)

	byScope, err := resolver.Report(context.Background(), RoleAccessRequest{Scopes: []string{"namespace"}})
	require.NoError(t, err)
	require.Len(t, byScope.Roles, 1)
	assert.Equal(t, "d8:namespace:admin", byScope.Roles[0].Name)
}

// The levels include one another by aggregating the role below. The export has
// to look through that: "which capability covers this resource" is the question
// it exists to answer, and "the manager role" is not an answer. Following the
// aggregation must also not count the same rules twice -- the controller has
// already copied them upwards into the included role.
func TestRoleReport_CompositionLooksThroughAnIncludedRole(t *testing.T) {
	t.Parallel()

	manager := aggregatingRole("d8:namespace:manager", "namespace",
		// What the aggregation controller copied up from the capability below.
		policyRule("apps", "deployments", "get"))
	manager.Labels["rbac.deckhouse.io/aggregate-to-namespace-as"] = "admin"

	admin := aggregatingRole("d8:namespace:admin", "namespace")

	objs := []runtime.Object{
		admin,
		manager,
		capability("d8:namespace-capability:kubernetes:workloads", "namespace", policyRule("apps", "deployments", "get")),
	}

	status, err := setupRoleAccessResolver(t, objs).Report(context.Background(), RoleAccessRequest{
		ExpandWildcards:    true,
		IncludeComposition: true,
	})
	require.NoError(t, err)

	role := roleOf(t, status, "d8:namespace:admin")

	names := make([]string, 0, len(role.Composition))
	for _, component := range role.Composition {
		names = append(names, component.Name)
	}
	assert.Equal(t, []string{"d8:namespace-capability:kubernetes:workloads"}, names,
		"the included role is followed to the capability under it")

	deployments := rowOf(t, role, "apps", "deployments")
	require.Len(t, deployments.Sources, 1, "the rules of an included role must not be counted twice")
	assert.Equal(t, "d8:namespace-capability:kubernetes:workloads", deployments.Sources[0].RoleName)
}

// A capability is a building block, not a role: it must not appear in the
// catalogue as something that can be granted.
func TestRoleReport_CapabilitiesAreNotRoles(t *testing.T) {
	t.Parallel()

	objs := []runtime.Object{
		capability("d8:namespace-capability:kubernetes:workloads", "namespace", policyRule("apps", "deployments", "get")),
	}

	status, err := setupRoleAccessResolver(t, objs).Report(context.Background(), RoleAccessRequest{})
	require.NoError(t, err)
	assert.Empty(t, status.Roles)
}

// customRole builds a ClusterRole created in the cluster, not shipped by the model.
func customRole(name, scope string, rules ...rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{labelRoleKind: "custom-role", labelRoleScope: scope},
		},
		Rules: rules,
	}
}

// A custom role belongs to one cluster; listing it beside the model reads as if
// the platform shipped it. The caller decides, and the catalogue reports both
// the roles it kept and how many it left out.
func TestRoleReport_CustomRolesFollowTheSelection(t *testing.T) {
	t.Parallel()

	objs := []runtime.Object{
		aggregatingRole("d8:namespace:admin", "namespace"),
		capability("d8:namespace-capability:kubernetes:workloads", "namespace", policyRule("apps", "deployments", "get")),
		customRole("d8:custom:namespace:release-manager", "namespace", policyRule("", "secrets", "get")),
	}

	resolver := setupRoleAccessResolver(t, objs)

	included, err := resolver.Report(context.Background(), RoleAccessRequest{})
	require.NoError(t, err)
	assert.Len(t, included.Roles, 2)
	assert.Empty(t, included.Notes)

	excluded, err := resolver.Report(context.Background(), RoleAccessRequest{ExcludeCustom: true})
	require.NoError(t, err)
	require.Len(t, excluded.Roles, 1)
	assert.Equal(t, "d8:namespace:admin", excluded.Roles[0].Name)
	require.Len(t, excluded.Notes, 1)
	assert.Contains(t, excluded.Notes[0], "1 custom roles")
}

// The catalogue is read from the narrowest access to the widest. By name it
// would open with the administrator, and telling the levels apart would take
// knowing the model by heart.
func TestRoleReport_RolesRunFromTheNarrowestAccessToTheWidest(t *testing.T) {
	t.Parallel()

	objs := []runtime.Object{
		aggregatingRole("d8:namespace:admin", "namespace"),
		aggregatingRole("d8:namespace:viewer", "namespace"),
		aggregatingRole("d8:namespace:superadmin", "namespace"),
		aggregatingRole("d8:namespace:user", "namespace"),
		aggregatingRole("d8:namespace:manager", "namespace"),
		aggregatingRole("d8:project:admin", "project"),
		aggregatingRole("d8:project:viewer", "project"),
		aggregatingRole("d8:system:admin", "system"),
		aggregatingRole("d8:subsystem:networking:manager", "subsystem"),
		aggregatingRole("d8:subsystem:networking:viewer", "subsystem"),
		aggregatingRole("d8:subsystem:cloud:viewer", "subsystem"),
	}

	status, err := setupRoleAccessResolver(t, objs).Report(context.Background(), RoleAccessRequest{})
	require.NoError(t, err)

	names := make([]string, 0, len(status.Roles))
	for _, role := range status.Roles {
		names = append(names, role.Name)
	}

	assert.Equal(t, []string{
		"d8:namespace:viewer",
		"d8:namespace:user",
		"d8:namespace:manager",
		"d8:namespace:admin",
		"d8:namespace:superadmin",
		"d8:project:viewer",
		"d8:project:admin",
		// Subsystems are peers, ordered by name; the levels inside each one
		// still run from the narrowest access to the widest.
		"d8:subsystem:cloud:viewer",
		"d8:subsystem:networking:viewer",
		"d8:subsystem:networking:manager",
		"d8:system:admin",
	}, names)
}

// Excluding custom roles must not make one unreachable: an audit of a named
// role still has to be possible.
func TestRoleReport_NamedCustomRoleSurvivesTheExclusion(t *testing.T) {
	t.Parallel()

	objs := []runtime.Object{
		aggregatingRole("d8:namespace:admin", "namespace"),
		customRole("d8:custom:namespace:release-manager", "namespace", policyRule("", "secrets", "get")),
	}

	status, err := setupRoleAccessResolver(t, objs).Report(context.Background(), RoleAccessRequest{
		ExcludeCustom: true,
		Names:         []string{"d8:custom:namespace:release-manager"},
	})
	require.NoError(t, err)

	require.Len(t, status.Roles, 1)
	rowOf(t, status.Roles[0], "", "secrets")
	assert.Empty(t, status.Notes)
}

// The rows are read from the capabilities, so a role the aggregation controller
// has not filled still reports its access -- and says that it did so.
func TestRoleReport_NotesAnUnfilledAggregation(t *testing.T) {
	t.Parallel()

	objs := []runtime.Object{
		aggregatingRole("d8:namespace:admin", "namespace"),
		capability("d8:namespace-capability:kubernetes:workloads", "namespace", policyRule("apps", "deployments", "get")),
	}

	status, err := setupRoleAccessResolver(t, objs).Report(context.Background(), RoleAccessRequest{ExpandWildcards: true})
	require.NoError(t, err)

	role := status.Roles[0]
	rowOf(t, role, "apps", "deployments")
	require.Len(t, role.Notes, 1)
	assert.Contains(t, role.Notes[0], "aggregation controller")
}
