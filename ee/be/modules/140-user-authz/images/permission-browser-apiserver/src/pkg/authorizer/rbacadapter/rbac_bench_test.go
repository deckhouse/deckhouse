package rbacadapter

import (
	"context"
	"fmt"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	benchChecks      = 258
	benchNoiseCRBs   = 940
	benchClusterCRBs = 53
)

var benchResources = []struct {
	group    string
	resource string
	verb     string
}{
	{"deckhouse.io", "modules", "list"},
	{"deckhouse.io", "moduleconfigs", "watch"},
	{"", "namespaces", "list"},
	{"", "nodes", "watch"},
	{"rbac.authorization.k8s.io", "clusterroles", "list"},
	{"cert-manager.io", "clusterissuers", "watch"},
	{"network.deckhouse.io", "egressgateways", "list"},
	{"observability.deckhouse.io", "clusterobservabilitydashboards", "watch"},
}

func benchAuthorizer(b *testing.B, objs ...runtime.Object) *RBACAuthorizer {
	b.Helper()
	client := fake.NewSimpleClientset(objs...)
	factory := informers.NewSharedInformerFactory(client, 0)
	auth := NewRBACAuthorizer(factory)
	stop := make(chan struct{})
	b.Cleanup(func() { close(stop) })
	factory.Start(stop)
	factory.WaitForCacheSync(stop)
	return auth
}

func carLabels() map[string]string {
	return map[string]string{"heritage": "deckhouse", "module": "user-authz"}
}

func wildcardClusterRole(name string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
			{NonResourceURLs: []string{"*"}, Verbs: []string{"*"}},
		},
	}
}

func granularClusterRole(name string, n int) *rbacv1.ClusterRole {
	rules := make([]rbacv1.PolicyRule, 0, n)
	for i := 0; i < n; i++ {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{fmt.Sprintf("mod%d.deckhouse.io", i%7)},
			Resources: []string{fmt.Sprintf("things%d", i), fmt.Sprintf("widgets%d", i)},
			Verbs:     []string{"get", "list", "watch", "update"},
		})
	}
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name}, Rules: rules}
}

func bind(name, role, userName string, car bool) *rbacv1.ClusterRoleBinding {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: userName}},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: role},
	}
	if car {
		crb.Labels = carLabels()
	}
	return crb
}

func noiseBindings(n int) []runtime.Object {
	objs := make([]runtime.Object, 0, n*2)
	for i := 0; i < n; i++ {
		roleName := fmt.Sprintf("noise-role-%d", i)
		objs = append(objs, granularClusterRole(roleName, 8))
		objs = append(objs, bind(fmt.Sprintf("user-authz:noise-%d:cluster-admin", i), roleName, fmt.Sprintf("noise-%d@example.io", i), true))
	}
	return objs
}

func superAdminWorld() []runtime.Object {
	objs := []runtime.Object{
		wildcardClusterRole("user-authz:super-admin"),
		bind("user-authz:super-admin:super-admin", "user-authz:super-admin", "super-admin@example.io", true),
	}
	return append(objs, noiseBindings(benchNoiseCRBs)...)
}

func clusterAdminWorld() []runtime.Object {
	objs := []runtime.Object{
		granularClusterRole("user-authz:cluster-admin", 45),
		bind("user-authz:cluster-admin:cluster-admin", "user-authz:cluster-admin", "cluster-admin@example.io", true),
	}
	for i := 1; i < benchClusterCRBs; i++ {
		roleName := fmt.Sprintf("d8:user-authz:module%d:cluster-admin", i)
		objs = append(objs, granularClusterRole(roleName, 12))
		objs = append(objs, bind(
			fmt.Sprintf("user-authz:cluster-admin:custom-%d", i),
			roleName,
			"cluster-admin@example.io",
			true,
		))
	}
	return append(objs, noiseBindings(benchNoiseCRBs)...)
}

func runBulk(b *testing.B, auth *RBACAuthorizer, userName string) {
	runBulkCtx(b, auth, userName, false)
}

func runBulkBound(b *testing.B, auth *RBACAuthorizer, userName string) {
	runBulkCtx(b, auth, userName, true)
}

func runBulkCtx(b *testing.B, auth *RBACAuthorizer, userName string, bind bool) {
	b.Helper()
	u := &user.DefaultInfo{Name: userName}
	ctx := context.Background()
	if bind {
		ctx = auth.BindSubject(ctx, u)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for i := 0; i < benchChecks; i++ {
			item := benchResources[i%len(benchResources)]
			attrs := &mockAttrs{
				user:       u,
				verb:       item.verb,
				resource:   item.resource,
				apiGroup:   item.group,
				isResource: true,
			}
			_, _, err := auth.Authorize(ctx, attrs)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkAuthorize_SuperAdmin_258(b *testing.B) {
	auth := benchAuthorizer(b, superAdminWorld()...)
	runBulk(b, auth, "super-admin@example.io")
}

func BenchmarkAuthorize_ClusterAdmin_258(b *testing.B) {
	auth := benchAuthorizer(b, clusterAdminWorld()...)
	runBulk(b, auth, "cluster-admin@example.io")
}

func BenchmarkAuthorize_Nobody_258(b *testing.B) {
	auth := benchAuthorizer(b, clusterAdminWorld()...)
	runBulk(b, auth, "nobody@example.io")
}

func BenchmarkAuthorize_SuperAdmin_258_Bound(b *testing.B) {
	auth := benchAuthorizer(b, superAdminWorld()...)
	runBulkBound(b, auth, "super-admin@example.io")
}

func BenchmarkAuthorize_ClusterAdmin_258_Bound(b *testing.B) {
	auth := benchAuthorizer(b, clusterAdminWorld()...)
	runBulkBound(b, auth, "cluster-admin@example.io")
}

func editorWorld() []runtime.Object {
	objs := []runtime.Object{
		granularClusterRole("user-authz:editor", 45),
		bind("user-authz:editor:editor", "user-authz:editor", "editor@example.io", true),
	}
	return append(objs, noiseBindings(benchNoiseCRBs)...)
}

func BenchmarkAuthorize_Editor_258(b *testing.B) {
	auth := benchAuthorizer(b, editorWorld()...)
	runBulk(b, auth, "editor@example.io")
}

func BenchmarkAuthorize_Editor_258_Bound(b *testing.B) {
	auth := benchAuthorizer(b, editorWorld()...)
	runBulkBound(b, auth, "editor@example.io")
}
