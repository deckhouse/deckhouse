/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package rbacadapter

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
)

// newWhoCanTestAuthorizer builds an RBACAuthorizer backed by a fake clientset
// populated with the given RBAC objects, with synced informers.
func newWhoCanTestAuthorizer(t *testing.T, objs ...runtime.Object) *RBACAuthorizer {
	t.Helper()

	client := fake.NewSimpleClientset(objs...)
	informerFactory := informers.NewSharedInformerFactory(client, 0)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Construct the authorizer first so its listers register the RBAC informers
	// with the factory, then start and wait for the caches to sync.
	auth := NewRBACAuthorizer(informerFactory)

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	return auth
}

func clusterRole(name string, rules ...rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Rules:      rules,
	}
}

func role(name, namespace string, rules ...rbacv1.PolicyRule) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
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

func resourceRule(apiGroups, resources, verbs []string) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{APIGroups: apiGroups, Resources: resources, Verbs: verbs}
}

// whoCan runs a reverse-RBAC query and asserts it returned no (partial) error,
// returning the resolved subjects. Use the engine's WhoCan directly when the
// error itself is under test.
func whoCan(t *testing.T, r *RBACAuthorizer, attrs *mockAttrs) WhoCanResult {
	t.Helper()
	res, err := r.WhoCan(context.Background(), attrs)
	require.NoError(t, err)
	return res
}

func TestWhoCan_ViaClusterRoleBinding(t *testing.T) {
	r := newWhoCanTestAuthorizer(t,
		clusterRole("net-admin", resourceRule([]string{"networking.k8s.io"}, []string{"networkpolicies"}, []string{"create"})),
		clusterRoleBinding("net-admin-binding", "net-admin",
			rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"},
			rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "netops"},
			rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Namespace: "kube-system", Name: "controller"},
		),
	)

	res := whoCan(t, r, &mockAttrs{
		verb:       "create",
		resource:   "networkpolicies",
		apiGroup:   "networking.k8s.io",
		namespace:  "myproject",
		isResource: true,
	})

	assert.Equal(t, []string{"alice"}, res.Users)
	assert.Equal(t, []string{"netops"}, res.Groups)
	require.Len(t, res.ServiceAccounts, 1)
	assert.Equal(t, ServiceAccountRef{Namespace: "kube-system", Name: "controller"}, res.ServiceAccounts[0])
}

func TestWhoCan_ViaRoleBinding(t *testing.T) {
	r := newWhoCanTestAuthorizer(t,
		role("pod-reader", "myproject", resourceRule([]string{""}, []string{"pods"}, []string{"get", "list"})),
		roleBinding("pod-reader-binding", "myproject", "Role", "pod-reader",
			rbacv1.Subject{Kind: rbacv1.UserKind, Name: "bob"},
		),
		// A RoleBinding in another namespace must NOT leak into the result.
		role("pod-reader", "other", resourceRule([]string{""}, []string{"pods"}, []string{"get"})),
		roleBinding("pod-reader-binding", "other", "Role", "pod-reader",
			rbacv1.Subject{Kind: rbacv1.UserKind, Name: "charlie"},
		),
	)

	res := whoCan(t, r, &mockAttrs{
		verb:       "get",
		resource:   "pods",
		apiGroup:   "",
		namespace:  "myproject",
		isResource: true,
	})

	assert.Equal(t, []string{"bob"}, res.Users)
	assert.Empty(t, res.Groups)
	assert.Empty(t, res.ServiceAccounts)
}

func TestWhoCan_RoleBindingToClusterRole(t *testing.T) {
	r := newWhoCanTestAuthorizer(t,
		clusterRole("secret-reader", resourceRule([]string{""}, []string{"secrets"}, []string{"get"})),
		roleBinding("secret-reader-binding", "myproject", "ClusterRole", "secret-reader",
			rbacv1.Subject{Kind: rbacv1.UserKind, Name: "dave"},
		),
	)

	res := whoCan(t, r, &mockAttrs{
		verb:       "get",
		resource:   "secrets",
		namespace:  "myproject",
		isResource: true,
	})

	assert.Equal(t, []string{"dave"}, res.Users)
}

func TestWhoCan_WildcardVerbAndResource(t *testing.T) {
	r := newWhoCanTestAuthorizer(t,
		clusterRole("super-admin", resourceRule([]string{"*"}, []string{"*"}, []string{"*"})),
		clusterRoleBinding("super-admin-binding", "super-admin",
			rbacv1.Subject{Kind: rbacv1.UserKind, Name: "root"},
		),
	)

	res := whoCan(t, r, &mockAttrs{
		verb:       "delete",
		resource:   "anything",
		apiGroup:   "apps",
		namespace:  "myproject",
		isResource: true,
	})

	assert.Equal(t, []string{"root"}, res.Users)
}

// TestWhoCan_AggregatedClusterRole verifies aggregated ClusterRoles work. The
// aggregation controller populates ClusterRole.Rules from matching roles; our
// engine reads those already-aggregated rules straight from the lister.
func TestWhoCan_AggregatedClusterRole(t *testing.T) {
	aggregated := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "view-aggregated"},
		AggregationRule: &rbacv1.AggregationRule{
			ClusterRoleSelectors: []metav1.LabelSelector{
				{MatchLabels: map[string]string{"rbac.example.com/aggregate-to-view": "true"}},
			},
		},
		// Rules as populated by the aggregation controller.
		Rules: []rbacv1.PolicyRule{
			resourceRule([]string{""}, []string{"configmaps"}, []string{"get", "list"}),
		},
	}

	r := newWhoCanTestAuthorizer(t,
		aggregated,
		clusterRoleBinding("view-binding", "view-aggregated",
			rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "viewers"},
		),
	)

	res := whoCan(t, r, &mockAttrs{
		verb:       "list",
		resource:   "configmaps",
		namespace:  "myproject",
		isResource: true,
	})

	assert.Equal(t, []string{"viewers"}, res.Groups)
}

func TestWhoCan_ServiceAccountNamespaceDefaulting(t *testing.T) {
	r := newWhoCanTestAuthorizer(t,
		role("cm-editor", "myproject", resourceRule([]string{""}, []string{"configmaps"}, []string{"update"})),
		// ServiceAccount subject with empty namespace defaults to the binding's namespace.
		roleBinding("cm-editor-binding", "myproject", "Role", "cm-editor",
			rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "builder"},
		),
	)

	res := whoCan(t, r, &mockAttrs{
		verb:       "update",
		resource:   "configmaps",
		namespace:  "myproject",
		isResource: true,
	})

	require.Len(t, res.ServiceAccounts, 1)
	assert.Equal(t, ServiceAccountRef{Namespace: "myproject", Name: "builder"}, res.ServiceAccounts[0])
}

func TestWhoCan_ResourceNames(t *testing.T) {
	r := newWhoCanTestAuthorizer(t,
		role("named-secret", "myproject", rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			Verbs:         []string{"get"},
			ResourceNames: []string{"my-secret"},
		}),
		roleBinding("named-secret-binding", "myproject", "Role", "named-secret",
			rbacv1.Subject{Kind: rbacv1.UserKind, Name: "erin"},
		),
	)

	// Query for the named resource: matches.
	res := whoCan(t, r, &mockAttrs{
		verb:       "get",
		resource:   "secrets",
		name:       "my-secret",
		namespace:  "myproject",
		isResource: true,
	})
	assert.Equal(t, []string{"erin"}, res.Users)

	// Query for a different name: does not match.
	res = whoCan(t, r, &mockAttrs{
		verb:       "get",
		resource:   "secrets",
		name:       "other-secret",
		namespace:  "myproject",
		isResource: true,
	})
	assert.Empty(t, res.Users)

	// Query with NO name (e.g. a list, or an unscoped get): a resourceNames-
	// scoped rule must NOT match, otherwise we over-report subjects that only
	// have access to one named object as if they had it broadly.
	res = whoCan(t, r, &mockAttrs{
		verb:       "list",
		resource:   "secrets",
		namespace:  "myproject",
		isResource: true,
	})
	assert.Empty(t, res.Users)

	// "*" in resourceNames is a literal name, not a wildcard: a request for a
	// differently named object must not match it.
	r = newWhoCanTestAuthorizer(t,
		role("wildcard-name", "myproject", rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			Verbs:         []string{"get"},
			ResourceNames: []string{"*"},
		}),
		roleBinding("wildcard-name-binding", "myproject", "Role", "wildcard-name",
			rbacv1.Subject{Kind: rbacv1.UserKind, Name: "erin"},
		),
	)
	res = whoCan(t, r, &mockAttrs{
		verb:       "get",
		resource:   "secrets",
		name:       "literally-star",
		namespace:  "myproject",
		isResource: true,
	})
	assert.Empty(t, res.Users)
}

func TestWhoCan_NegativeNoMatch(t *testing.T) {
	r := newWhoCanTestAuthorizer(t,
		clusterRole("pod-reader", resourceRule([]string{""}, []string{"pods"}, []string{"get"})),
		clusterRoleBinding("pod-reader-binding", "pod-reader",
			rbacv1.Subject{Kind: rbacv1.UserKind, Name: "frank"},
		),
	)

	res := whoCan(t, r, &mockAttrs{
		verb:       "delete",
		resource:   "pods",
		namespace:  "myproject",
		isResource: true,
	})

	assert.Empty(t, res.Users)
	assert.Empty(t, res.Groups)
	assert.Empty(t, res.ServiceAccounts)
}

func TestWhoCan_NonResourceURL(t *testing.T) {
	r := newWhoCanTestAuthorizer(t,
		clusterRole("metrics-reader", rbacv1.PolicyRule{
			NonResourceURLs: []string{"/metrics"},
			Verbs:           []string{"get"},
		}),
		clusterRoleBinding("metrics-reader-binding", "metrics-reader",
			rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "monitoring"},
		),
	)

	res := whoCan(t, r, &mockAttrs{
		verb:       "get",
		path:       "/metrics",
		isResource: false,
	})

	assert.Equal(t, []string{"monitoring"}, res.Groups)
}

func TestWhoCan_DeduplicatesSubjects(t *testing.T) {
	// Two bindings granting the same action to the same user must dedup.
	r := newWhoCanTestAuthorizer(t,
		clusterRole("reader-a", resourceRule([]string{""}, []string{"pods"}, []string{"get"})),
		clusterRole("reader-b", resourceRule([]string{""}, []string{"*"}, []string{"get"})),
		clusterRoleBinding("binding-a", "reader-a", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "grace"}),
		clusterRoleBinding("binding-b", "reader-b", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "grace"}),
	)

	res := whoCan(t, r, &mockAttrs{
		verb:       "get",
		resource:   "pods",
		namespace:  "myproject",
		isResource: true,
	})

	assert.Equal(t, []string{"grace"}, res.Users)
}

// TestWhoCan_ClusterRoleBindingServiceAccountEmptyNamespace verifies that a
// ServiceAccount subject of a ClusterRoleBinding keeps an empty namespace
// (there is no binding namespace to default to, unlike a RoleBinding).
func TestWhoCan_ClusterRoleBindingServiceAccountEmptyNamespace(t *testing.T) {
	r := newWhoCanTestAuthorizer(t,
		clusterRole("node-reader", resourceRule([]string{""}, []string{"nodes"}, []string{"get"})),
		clusterRoleBinding("node-reader-binding", "node-reader",
			rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "kubelet"},
		),
	)

	res := whoCan(t, r, &mockAttrs{
		verb:       "get",
		resource:   "nodes",
		isResource: true,
	})

	require.Len(t, res.ServiceAccounts, 1)
	assert.Equal(t, ServiceAccountRef{Namespace: "", Name: "kubelet"}, res.ServiceAccounts[0])
}

// TestWhoCan_DeduplicatesAcrossClusterRoleBindingAndRoleBinding verifies that a
// ServiceAccount granted the same action via both a ClusterRoleBinding and a
// namespaced RoleBinding appears exactly once.
func TestWhoCan_DeduplicatesAcrossClusterRoleBindingAndRoleBinding(t *testing.T) {
	r := newWhoCanTestAuthorizer(t,
		clusterRole("cm-reader", resourceRule([]string{""}, []string{"configmaps"}, []string{"get"})),
		clusterRoleBinding("cm-reader-crb", "cm-reader",
			rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Namespace: "myproject", Name: "agent"},
		),
		role("cm-reader-role", "myproject", resourceRule([]string{""}, []string{"configmaps"}, []string{"get"})),
		roleBinding("cm-reader-rb", "myproject", "Role", "cm-reader-role",
			rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Namespace: "myproject", Name: "agent"},
		),
	)

	res := whoCan(t, r, &mockAttrs{
		verb:       "get",
		resource:   "configmaps",
		namespace:  "myproject",
		isResource: true,
	})

	require.Len(t, res.ServiceAccounts, 1)
	assert.Equal(t, ServiceAccountRef{Namespace: "myproject", Name: "agent"}, res.ServiceAccounts[0])
}

// TestWhoCan_ContextCancelled verifies the engine honors context cancellation
// and reports it as a (non-fatal) error instead of doing the work.
func TestWhoCan_ContextCancelled(t *testing.T) {
	r := newWhoCanTestAuthorizer(t,
		clusterRole("pod-reader", resourceRule([]string{""}, []string{"pods"}, []string{"get"})),
		clusterRoleBinding("pod-reader-binding", "pod-reader",
			rbacv1.Subject{Kind: rbacv1.UserKind, Name: "frank"},
		),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := r.WhoCan(ctx, &mockAttrs{
		verb:       "get",
		resource:   "pods",
		namespace:  "myproject",
		isResource: true,
	})

	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, res.Users)
}
