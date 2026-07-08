/*
Copyright 2026 Flant JSC
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
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func newTestRBACAuthorizer(t *testing.T, objs ...runtime.Object) *RBACAuthorizer {
	t.Helper()

	client := fake.NewSimpleClientset(objs...)
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	auth := NewRBACAuthorizer(informerFactory)

	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	return auth
}

func TestIsCARManagedClusterRoleBinding(t *testing.T) {
	deckhouseLabels := map[string]string{"heritage": "deckhouse", "module": "user-authz"}

	tests := []struct {
		name     string
		binding  *rbacv1.ClusterRoleBinding
		expected bool
	}{
		{
			name: "CAR-generated binding",
			binding: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "user-authz:car0:editor", Labels: deckhouseLabels},
			},
			expected: true,
		},
		{
			name: "user-created binding with similar name but without module labels",
			binding: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "user-authz:custom"},
			},
			expected: false,
		},
		{
			name: "deckhouse binding of another module",
			binding: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "d8:user-authz:webhook",
					Labels: deckhouseLabels,
				},
			},
			expected: false,
		},
		{
			name: "plain user binding",
			binding: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "viewers"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsCARManagedClusterRoleBinding(tt.binding))
		})
	}
}

// TestAllowsIndependently_ExcludesCARBindings is the core anti-escalation
// check: a CAR-generated cluster-wide binding must NOT count as an
// independent grant, while a plain RoleBinding or a user-created
// ClusterRoleBinding must.
func TestAllowsIndependently_ExcludesCARBindings(t *testing.T) {
	deckhouseLabels := map[string]string{"heritage": "deckhouse", "module": "user-authz"}

	objs := []runtime.Object{
		// CAR accessLevel: alice is Editor cluster-wide via a CAR-generated CRB.
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "user-authz:editor"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "user-authz:car0:editor", Labels: deckhouseLabels},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "alice"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "user-authz:editor"},
		},
		// Plain RoleBinding in ns-d: get/list pods only.
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-reader", Namespace: "ns-d"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list"}},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "alice-pod-reader", Namespace: "ns-d"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "alice"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "pod-reader"},
		},
		// User-created CRB for bob: view secrets cluster-wide.
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "secret-viewer"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get", "list"}},
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "bob-secret-viewer"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "bob"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "secret-viewer"},
		},
	}

	auth := newTestRBACAuthorizer(t, objs...)

	tests := []struct {
		name     string
		attrs    *mockAttrs
		expected bool
	}{
		{
			name: "RoleBinding grant in its namespace counts",
			attrs: &mockAttrs{
				user:       &user.DefaultInfo{Name: "alice"},
				verb:       "get",
				resource:   "pods",
				namespace:  "ns-d",
				isResource: true,
			},
			expected: true,
		},
		{
			name: "verb beyond the RoleBinding must not be granted even though the CAR CRB allows it",
			attrs: &mockAttrs{
				user:       &user.DefaultInfo{Name: "alice"},
				verb:       "delete",
				resource:   "pods",
				namespace:  "ns-d",
				isResource: true,
			},
			expected: false,
		},
		{
			name: "CAR CRB alone grants nothing independently",
			attrs: &mockAttrs{
				user:       &user.DefaultInfo{Name: "alice"},
				verb:       "get",
				resource:   "secrets",
				namespace:  "ns-x",
				isResource: true,
			},
			expected: false,
		},
		{
			name: "user-created ClusterRoleBinding counts",
			attrs: &mockAttrs{
				user:       &user.DefaultInfo{Name: "bob"},
				verb:       "list",
				resource:   "secrets",
				namespace:  "ns-x",
				isResource: true,
			},
			expected: true,
		},
		{
			name: "user-created ClusterRoleBinding counts for cluster-scoped requests too",
			attrs: &mockAttrs{
				user:       &user.DefaultInfo{Name: "bob"},
				verb:       "list",
				resource:   "secrets",
				namespace:  "",
				isResource: true,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, auth.AllowsIndependently(context.Background(), tt.attrs))
		})
	}
}

// TestMatchesResource_UpstreamSemantics pins the matcher to upstream RBAC
// behavior: bare resource rules do not grant subresources, and "resource/*"
// is not a wildcard.
func TestMatchesResource_UpstreamSemantics(t *testing.T) {
	r := &RBACAuthorizer{}

	assert.True(t, r.matchesResource([]string{"pods"}, "pods", ""))
	assert.True(t, r.matchesResource([]string{"pods/log"}, "pods", "log"))
	assert.True(t, r.matchesResource([]string{"*/log"}, "pods", "log"))
	assert.True(t, r.matchesResource([]string{"*"}, "pods", "log"))

	assert.False(t, r.matchesResource([]string{"pods"}, "pods", "log"),
		"a rule for the bare resource must not grant its subresources")
	assert.False(t, r.matchesResource([]string{"pods/*"}, "pods", "log"),
		"upstream RBAC does not support resource/* wildcards")
	assert.False(t, r.matchesResource([]string{"pods/log"}, "pods", ""),
		"a subresource rule must not grant the bare resource")
}
