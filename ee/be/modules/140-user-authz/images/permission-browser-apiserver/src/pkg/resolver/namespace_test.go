/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"permission-browser-apiserver/pkg/authorizer/multitenancy"
)

// TestServiceAccountSubjectMatching tests the SA namespace defaulting edge case
func TestServiceAccountSubjectMatching(t *testing.T) {
	r := &NamespaceResolver{}

	tests := []struct {
		name        string
		subjects    []rbacv1.Subject
		userName    string
		userGroups  []string
		rbNamespace string
		expected    bool
	}{
		{
			name: "SA subject with explicit namespace",
			subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      "my-sa",
					Namespace: "explicit-ns",
				},
			},
			userName:    "system:serviceaccount:explicit-ns:my-sa",
			rbNamespace: "other-ns",
			expected:    true,
		},
		{
			name: "SA subject with empty namespace defaults to RB namespace",
			subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      "my-sa",
					Namespace: "", // Empty - should default to RB namespace
				},
			},
			userName:    "system:serviceaccount:rb-ns:my-sa",
			rbNamespace: "rb-ns",
			expected:    true,
		},
		{
			name: "SA subject with empty namespace doesn't match different namespace",
			subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      "my-sa",
					Namespace: "", // Empty - should default to RB namespace
				},
			},
			userName:    "system:serviceaccount:other-ns:my-sa",
			rbNamespace: "rb-ns",
			expected:    false,
		},
		{
			name: "User subject matches",
			subjects: []rbacv1.Subject{
				{Kind: rbacv1.UserKind, Name: "alice"},
			},
			userName:    "alice",
			rbNamespace: "any",
			expected:    true,
		},
		{
			name: "Group subject matches",
			subjects: []rbacv1.Subject{
				{Kind: rbacv1.GroupKind, Name: "developers"},
			},
			userName:    "bob",
			userGroups:  []string{"developers", "authenticated"},
			rbNamespace: "any",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.subjectMatchesWithNamespaceDefault(tt.subjects, tt.userName, tt.userGroups, tt.rbNamespace)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestHasNamespacedRules tests detection of namespaced rules
func TestHasNamespacedRules(t *testing.T) {
	r := &NamespaceResolver{scopeCache: newTestScopeCache()}

	tests := []struct {
		name     string
		rules    []rbacv1.PolicyRule
		expected bool
	}{
		{
			name: "wildcard resources",
			rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"*"},
					Verbs:     []string{"get"},
				},
			},
			expected: true,
		},
		{
			name: "wildcard apiGroups with resources",
			rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"*"},
					Resources: []string{"deployments"},
					Verbs:     []string{"get"},
				},
			},
			expected: true,
		},
		{
			name: "common namespaced resource - pods",
			rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"get"},
				},
			},
			expected: true,
		},
		{
			name: "common namespaced resource - services",
			rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"services"},
					Verbs:     []string{"get"},
				},
			},
			expected: true,
		},
		{
			name: "cluster-scoped resource only - namespaces",
			rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"namespaces"},
					Verbs:     []string{"get"},
				},
			},
			expected: false,
		},
		{
			name: "cluster-scoped resource only - clusterroles",
			rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"rbac.authorization.k8s.io"},
					Resources: []string{"clusterroles"},
					Verbs:     []string{"get"},
				},
			},
			expected: false,
		},
		{
			name: "nonResourceURLs only",
			rules: []rbacv1.PolicyRule{
				{
					NonResourceURLs: []string{"/healthz"},
					Verbs:           []string{"get"},
				},
			},
			expected: false,
		},
		{
			name: "empty verbs",
			rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{},
				},
			},
			expected: false,
		},
		{
			name: "mixed rules - one namespaced",
			rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"namespaces"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"list"},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.hasNamespacedRules(tt.rules)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestResolveAccessibleNamespaces_ClusterRoleBinding tests global access via CRB
func TestResolveAccessibleNamespaces_ClusterRoleBinding(t *testing.T) {
	// Setup: ClusterRoleBinding grants global namespaced access
	objs := []runtime.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "reader"},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"get", "list"},
				},
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "reader-binding"},
			Subjects: []rbacv1.Subject{
				{Kind: rbacv1.UserKind, Name: "global-reader"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "reader",
			},
		},
	}

	resolver := setupResolver(t, objs, nil)

	// User with ClusterRoleBinding should see all namespaces
	userInfo := &user.DefaultInfo{Name: "global-reader"}
	namespaces, err := resolver.ResolveAccessibleNamespaces(userInfo)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"default", "app-ns", "kube-system"}, namespaces)
}

// TestResolveAccessibleNamespaces_RoleBinding tests namespace-specific access via RB
func TestResolveAccessibleNamespaces_RoleBinding(t *testing.T) {
	objs := []runtime.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other-ns"}},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "app-reader", Namespace: "app-ns"},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods", "services"},
					Verbs:     []string{"get", "list"},
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "app-reader-binding", Namespace: "app-ns"},
			Subjects: []rbacv1.Subject{
				{Kind: rbacv1.UserKind, Name: "app-user"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "app-reader",
			},
		},
	}

	resolver := setupResolver(t, objs, nil)

	// User with RoleBinding should only see that namespace
	userInfo := &user.DefaultInfo{Name: "app-user"}
	namespaces, err := resolver.ResolveAccessibleNamespaces(userInfo)
	require.NoError(t, err)
	assert.Equal(t, []string{"app-ns"}, namespaces)
}

// TestResolveAccessibleNamespaces_MultiTenancyFilter tests MT filtering
// Note: This is a simplified test that verifies the resolver without MT engine.
// MT filtering is tested more thoroughly in the multitenancy package tests.
func TestResolveAccessibleNamespaces_MultiTenancyFilter(t *testing.T) {
	objs := []runtime.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "admin"},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"*"},
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				},
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "admin-binding"},
			Subjects: []rbacv1.Subject{
				{Kind: rbacv1.UserKind, Name: "admin"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "admin",
			},
		},
	}

	// Without MT engine, all namespaces with RBAC access are returned
	resolver := setupResolver(t, objs, nil)

	userInfo := &user.DefaultInfo{Name: "admin"}
	namespaces, err := resolver.ResolveAccessibleNamespaces(userInfo)
	require.NoError(t, err)
	assert.Equal(t, []string{"app-ns"}, namespaces)
}

// TestResolveAccessibleNamespaces_ServiceAccountWithEmptyNamespace tests SA namespace defaulting
func TestResolveAccessibleNamespaces_ServiceAccountWithEmptyNamespace(t *testing.T) {
	objs := []runtime.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other-ns"}},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-reader", Namespace: "app-ns"},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"get"},
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "sa-binding", Namespace: "app-ns"},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      "my-sa",
					Namespace: "", // Empty - should default to RoleBinding namespace
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "pod-reader",
			},
		},
	}

	resolver := setupResolver(t, objs, nil)

	// SA in app-ns should match the binding with empty namespace
	userInfo := &user.DefaultInfo{Name: "system:serviceaccount:app-ns:my-sa"}
	namespaces, err := resolver.ResolveAccessibleNamespaces(userInfo)
	require.NoError(t, err)
	assert.Equal(t, []string{"app-ns"}, namespaces)

	// SA in other-ns should NOT match
	userInfo2 := &user.DefaultInfo{Name: "system:serviceaccount:other-ns:my-sa"}
	namespaces2, err := resolver.ResolveAccessibleNamespaces(userInfo2)
	require.NoError(t, err)
	assert.Empty(t, namespaces2)
}

// TestIsNamespaceAccessible tests the single namespace check
func TestIsNamespaceAccessible(t *testing.T) {
	objs := []runtime.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "allowed-ns"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "denied-ns"}},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "reader", Namespace: "allowed-ns"},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"get"},
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "reader-binding", Namespace: "allowed-ns"},
			Subjects: []rbacv1.Subject{
				{Kind: rbacv1.UserKind, Name: "test-user"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "reader",
			},
		},
	}

	resolver := setupResolver(t, objs, nil)
	userInfo := &user.DefaultInfo{Name: "test-user"}

	// Allowed namespace
	accessible, err := resolver.IsNamespaceAccessible(userInfo, "allowed-ns")
	require.NoError(t, err)
	assert.True(t, accessible)

	// Denied namespace (no binding)
	accessible, err = resolver.IsNamespaceAccessible(userInfo, "denied-ns")
	require.NoError(t, err)
	assert.False(t, accessible)

	// Non-existent namespace
	accessible, err = resolver.IsNamespaceAccessible(userInfo, "nonexistent")
	require.NoError(t, err)
	assert.False(t, accessible)
}

// TestResolveAccessibleNamespaces_NoUser tests nil user handling
func TestResolveAccessibleNamespaces_NoUser(t *testing.T) {
	resolver := setupResolver(t, []runtime.Object{}, nil)

	namespaces, err := resolver.ResolveAccessibleNamespaces(nil)
	require.NoError(t, err)
	assert.Nil(t, namespaces)
}

// TestIsResourceNamespaced_WithScopeCache tests resource namespace detection via scope cache
func TestIsResourceNamespaced_WithScopeCache(t *testing.T) {
	scopeCache := NewResourceScopeCache(nil)
	// Manually populate the cache for testing
	scopeCache.mu.Lock()
	scopeCache.scopeMap = map[string]bool{
		"/pods":                                  true,
		"/services":                              true,
		"apps/deployments":                       true,
		"/namespaces":                            false,
		"/nodes":                                 false,
		"rbac.authorization.k8s.io/clusterroles": false,
	}
	scopeCache.mu.Unlock()

	r := &NamespaceResolver{scopeCache: scopeCache}

	assert.True(t, r.isResourceNamespaced("", "pods"), "pods should be namespaced")
	assert.True(t, r.isResourceNamespaced("", "services"), "services should be namespaced")
	assert.True(t, r.isResourceNamespaced("apps", "deployments"), "deployments should be namespaced")
	assert.False(t, r.isResourceNamespaced("", "namespaces"), "namespaces should be cluster-scoped")
	assert.False(t, r.isResourceNamespaced("", "nodes"), "nodes should be cluster-scoped")
	assert.False(t, r.isResourceNamespaced("rbac.authorization.k8s.io", "clusterroles"), "clusterroles should be cluster-scoped")
}

// TestIsResourceNamespaced_NilScopeCache tests fail-closed behavior without scope cache
func TestIsResourceNamespaced_NilScopeCache(t *testing.T) {
	r := &NamespaceResolver{scopeCache: nil}

	// Without scope cache, everything should be assumed cluster-scoped (fail-closed)
	assert.False(t, r.isResourceNamespaced("", "pods"),
		"should assume cluster-scoped when scope cache is nil")
	assert.False(t, r.isResourceNamespaced("", "namespaces"),
		"should assume cluster-scoped when scope cache is nil")
	assert.False(t, r.isResourceNamespaced("custom.example.com", "unknownresource"),
		"unknown resource should be assumed cluster-scoped when scope cache is nil")
}

// TestIsResourceNamespaced_UnknownResource tests fail-closed for unknown resources
func TestIsResourceNamespaced_UnknownResource(t *testing.T) {
	scopeCache := NewResourceScopeCache(nil)
	// Populate with some known resources
	scopeCache.mu.Lock()
	scopeCache.scopeMap = map[string]bool{
		"/pods": true,
	}
	scopeCache.mu.Unlock()

	r := &NamespaceResolver{scopeCache: scopeCache}

	// Unknown resource should return false (fail-closed)
	assert.False(t, r.isResourceNamespaced("unknown.example.com", "unknownresource"),
		"unknown resource should be assumed cluster-scoped")
}

// Helper functions

// newTestScopeCache creates a ResourceScopeCache pre-populated with well-known resources.
// This is used in tests because fake.NewSimpleClientset does not expose real API resources
// via discovery, so we populate the cache manually.
func newTestScopeCache() *ResourceScopeCache {
	c := NewResourceScopeCache(nil)
	c.mu.Lock()
	c.scopeMap = map[string]bool{
		// Core namespaced
		"/pods": true, "/services": true, "/configmaps": true, "/secrets": true,
		"/serviceaccounts": true, "/endpoints": true, "/events": true,
		"/persistentvolumeclaims": true, "/replicationcontrollers": true,
		// Core cluster-scoped
		"/namespaces": false, "/nodes": false, "/persistentvolumes": false,
		// Apps
		"apps/deployments": true, "apps/replicasets": true, "apps/statefulsets": true,
		"apps/daemonsets": true, "apps/controllerrevisions": true,
		// Batch
		"batch/jobs": true, "batch/cronjobs": true,
		// RBAC
		"rbac.authorization.k8s.io/roles": true, "rbac.authorization.k8s.io/rolebindings": true,
		"rbac.authorization.k8s.io/clusterroles": false, "rbac.authorization.k8s.io/clusterrolebindings": false,
		// Networking
		"networking.k8s.io/ingresses": true, "networking.k8s.io/networkpolicies": true,
	}
	c.mu.Unlock()
	return c
}

func setupResolver(t *testing.T, objs []runtime.Object, mtEngine *multitenancy.Engine) *NamespaceResolver {
	client := fake.NewSimpleClientset(objs...)
	informerFactory := informers.NewSharedInformerFactory(client, 0)

	// Explicitly access all informers we need - this registers them with the factory
	// The listers must be obtained BEFORE calling Start() for the informers to be registered
	nsLister := informerFactory.Core().V1().Namespaces().Lister()
	roleLister := informerFactory.Rbac().V1().Roles().Lister()
	roleBindingLister := informerFactory.Rbac().V1().RoleBindings().Lister()
	clusterRoleLister := informerFactory.Rbac().V1().ClusterRoles().Lister()
	clusterRoleBindingLister := informerFactory.Rbac().V1().ClusterRoleBindings().Lister()

	// Create a stop channel
	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })

	// Start informers
	informerFactory.Start(stopCh)

	// Wait for all informers to sync
	synced := informerFactory.WaitForCacheSync(stopCh)
	for informerType, ok := range synced {
		if !ok {
			t.Fatalf("informer %v failed to sync", informerType)
		}
	}

	return &NamespaceResolver{
		nsLister:                 nsLister,
		roleLister:               roleLister,
		roleBindingLister:        roleBindingLister,
		clusterRoleLister:        clusterRoleLister,
		clusterRoleBindingLister: clusterRoleBindingLister,
		scopeCache:               newTestScopeCache(),
		mtEngine:                 mtEngine,
	}
}
