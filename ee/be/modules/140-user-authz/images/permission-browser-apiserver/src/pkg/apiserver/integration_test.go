/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package apiserver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
	"permission-browser-apiserver/pkg/authorizer/composite"
	"permission-browser-apiserver/pkg/authorizer/rbacadapter"
	"permission-browser-apiserver/pkg/registry"
	"permission-browser-apiserver/pkg/resolver"
)

// TestIntegration_BulkSARWithFakeClient tests the full flow with fake Kubernetes client
func TestIntegration_BulkSARWithFakeClient(t *testing.T) {
	// Create fake Kubernetes objects
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "test-reader"},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "services"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "test-reader-binding"},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "test-reader",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "test-user"},
		},
	}

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespace-admin",
			Namespace: "test-ns",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"", "apps", "extensions"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespace-admin-binding",
			Namespace: "test-ns",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "namespace-admin",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "ns-admin-user"},
		},
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ns"},
	}

	// Create fake clientset with objects
	fakeClient := fake.NewSimpleClientset(clusterRole, clusterRoleBinding, role, roleBinding, namespace)

	// Create informer factory
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)

	// Create RBAC authorizer
	rbacAuth := rbacadapter.NewRBACAuthorizer(informerFactory)

	// Start informers and wait for sync
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	// Create storage with RBAC authorizer (no multi-tenancy for simplicity)
	storage := registry.NewBulkSARStorage(rbacAuth)

	tests := []struct {
		name           string
		user           string
		requests       []v1alpha1.SubjectAccessReviewRequest
		expectedAllow  []bool
		expectedReason []string
	}{
		{
			name: "test-user can read pods cluster-wide",
			user: "test-user",
			requests: []v1alpha1.SubjectAccessReviewRequest{
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Verb:     "get",
						Resource: "pods",
					},
				},
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Verb:     "list",
						Resource: "services",
					},
				},
			},
			expectedAllow: []bool{true, true},
		},
		{
			name: "test-user cannot create pods",
			user: "test-user",
			requests: []v1alpha1.SubjectAccessReviewRequest{
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Verb:     "create",
						Resource: "pods",
					},
				},
			},
			expectedAllow: []bool{false},
		},
		{
			name: "ns-admin-user can do anything in test-ns",
			user: "ns-admin-user",
			requests: []v1alpha1.SubjectAccessReviewRequest{
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Namespace: "test-ns",
						Verb:      "delete",
						Resource:  "pods",
					},
				},
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Namespace: "test-ns",
						Verb:      "create",
						Resource:  "deployments",
						Group:     "apps",
					},
				},
			},
			expectedAllow: []bool{true, true},
		},
		{
			name: "ns-admin-user cannot access other namespaces",
			user: "ns-admin-user",
			requests: []v1alpha1.SubjectAccessReviewRequest{
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Namespace: "other-ns",
						Verb:      "get",
						Resource:  "pods",
					},
				},
			},
			expectedAllow: []bool{false},
		},
		{
			name: "unknown user has no permissions",
			user: "unknown-user",
			requests: []v1alpha1.SubjectAccessReviewRequest{
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Verb:     "get",
						Resource: "pods",
					},
				},
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Verb:     "list",
						Resource: "secrets",
					},
				},
			},
			expectedAllow: []bool{false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create BulkSubjectAccessReview request
			bsar := &v1alpha1.BulkSubjectAccessReview{
				Spec: v1alpha1.BulkSubjectAccessReviewSpec{
					Requests: tt.requests,
				},
			}

			// Create context with user info (simulating authenticated request)
			testCtx := createTestContext(tt.user, nil)

			// Execute
			result, err := storage.Create(testCtx, bsar, nil, &metav1.CreateOptions{})
			require.NoError(t, err)

			resultBSAR, ok := result.(*v1alpha1.BulkSubjectAccessReview)
			require.True(t, ok, "result should be BulkSubjectAccessReview")

			// Verify results
			require.Len(t, resultBSAR.Status.Results, len(tt.expectedAllow))
			for i, expected := range tt.expectedAllow {
				assert.Equal(t, expected, resultBSAR.Status.Results[i].Allowed,
					"request %d: expected allowed=%v, got allowed=%v, reason=%s",
					i, expected, resultBSAR.Status.Results[i].Allowed, resultBSAR.Status.Results[i].Reason)
			}
		})
	}
}

// TestIntegration_BulkSARWithGroups tests group-based authorization
func TestIntegration_BulkSARWithGroups(t *testing.T) {
	// Create ClusterRole for developers group
	developerRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "developer"},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "replicasets"},
				Verbs:     []string{"get", "list", "watch", "create", "update"},
			},
		},
	}

	developerBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "developers-binding"},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "developer",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "Group", Name: "developers"},
		},
	}

	fakeClient := fake.NewSimpleClientset(developerRole, developerBinding)
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	rbacAuth := rbacadapter.NewRBACAuthorizer(informerFactory)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	storage := registry.NewBulkSARStorage(rbacAuth)

	// User in developers group
	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			Requests: []v1alpha1.SubjectAccessReviewRequest{
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Verb:     "create",
						Resource: "deployments",
						Group:    "apps",
					},
				},
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Verb:     "delete",
						Resource: "deployments",
						Group:    "apps",
					},
				},
			},
		},
	}

	testCtx := createTestContext("dev-user", []string{"developers", "authenticated"})
	result, err := storage.Create(testCtx, bsar, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	resultBSAR := result.(*v1alpha1.BulkSubjectAccessReview)

	// Can create (in verbs)
	assert.True(t, resultBSAR.Status.Results[0].Allowed, "should be able to create deployments")
	// Cannot delete (not in verbs)
	assert.False(t, resultBSAR.Status.Results[1].Allowed, "should not be able to delete deployments")
}

// TestIntegration_BulkSARWithServiceAccount tests ServiceAccount authorization
func TestIntegration_BulkSARWithServiceAccount(t *testing.T) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-system"},
	}

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configmap-reader",
			Namespace: "kube-system",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list"},
			},
		},
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configmap-reader-binding",
			Namespace: "kube-system",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "configmap-reader",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "my-sa",
				Namespace: "kube-system",
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(namespace, role, roleBinding)
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	rbacAuth := rbacadapter.NewRBACAuthorizer(informerFactory)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	storage := registry.NewBulkSARStorage(rbacAuth)

	// ServiceAccount user
	saUser := "system:serviceaccount:kube-system:my-sa"
	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			Requests: []v1alpha1.SubjectAccessReviewRequest{
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Namespace: "kube-system",
						Verb:      "get",
						Resource:  "configmaps",
					},
				},
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Namespace: "default",
						Verb:      "get",
						Resource:  "configmaps",
					},
				},
			},
		},
	}

	testCtx := createTestContext(saUser, []string{"system:serviceaccounts", "system:serviceaccounts:kube-system"})
	result, err := storage.Create(testCtx, bsar, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	resultBSAR := result.(*v1alpha1.BulkSubjectAccessReview)

	// Can read configmaps in kube-system
	assert.True(t, resultBSAR.Status.Results[0].Allowed, "SA should be able to get configmaps in kube-system")
	// Cannot read configmaps in default
	assert.False(t, resultBSAR.Status.Results[1].Allowed, "SA should not be able to get configmaps in default")
}

// TestIntegration_CompositeAuthorizerWithFakeClient tests composite authorizer
func TestIntegration_CompositeAuthorizerWithFakeClient(t *testing.T) {
	// Setup RBAC that allows everything
	adminRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "admin"},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	}

	adminBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "admin-binding"},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "admin",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "admin-user"},
		},
	}

	fakeClient := fake.NewSimpleClientset(adminRole, adminBinding)
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	rbacAuth := rbacadapter.NewRBACAuthorizer(informerFactory)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	// Create composite authorizer with mock multi-tenancy (that denies system namespaces)
	mockMT := &mockMultitenancyAuthorizer{
		denyNamespaces: []string{"kube-system", "kube-public"},
	}

	compositeAuth := composite.NewCompositeAuthorizer(mockMT, rbacAuth)
	storage := registry.NewBulkSARStorage(compositeAuth)

	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			Requests: []v1alpha1.SubjectAccessReviewRequest{
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Namespace: "default",
						Verb:      "delete",
						Resource:  "pods",
					},
				},
				{
					ResourceAttributes: &v1alpha1.ResourceAttributes{
						Namespace: "kube-system",
						Verb:      "delete",
						Resource:  "pods",
					},
				},
			},
		},
	}

	testCtx := createTestContext("admin-user", nil)
	result, err := storage.Create(testCtx, bsar, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	resultBSAR := result.(*v1alpha1.BulkSubjectAccessReview)

	// Admin can delete in default (RBAC allows, MT allows)
	assert.True(t, resultBSAR.Status.Results[0].Allowed, "admin should be able to delete pods in default")
	// Admin cannot delete in kube-system (RBAC allows, MT denies)
	assert.False(t, resultBSAR.Status.Results[1].Allowed, "admin should be denied by multi-tenancy in kube-system")
	assert.True(t, resultBSAR.Status.Results[1].Denied, "should be explicitly denied")
}

// TestIntegration_NonResourceURLs tests non-resource URL authorization
func TestIntegration_NonResourceURLs(t *testing.T) {
	// Create role that allows /healthz and /version
	healthRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "health-checker"},
		Rules: []rbacv1.PolicyRule{
			{
				NonResourceURLs: []string{"/healthz", "/healthz/*", "/version"},
				Verbs:           []string{"get"},
			},
		},
	}

	healthBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "health-checker-binding"},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "health-checker",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "Group", Name: "system:authenticated"},
		},
	}

	fakeClient := fake.NewSimpleClientset(healthRole, healthBinding)
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	rbacAuth := rbacadapter.NewRBACAuthorizer(informerFactory)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	storage := registry.NewBulkSARStorage(rbacAuth)

	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			Requests: []v1alpha1.SubjectAccessReviewRequest{
				{
					NonResourceAttributes: &v1alpha1.NonResourceAttributes{
						Path: "/healthz",
						Verb: "get",
					},
				},
				{
					NonResourceAttributes: &v1alpha1.NonResourceAttributes{
						Path: "/version",
						Verb: "get",
					},
				},
				{
					NonResourceAttributes: &v1alpha1.NonResourceAttributes{
						Path: "/metrics",
						Verb: "get",
					},
				},
			},
		},
	}

	testCtx := createTestContext("any-user", []string{"system:authenticated"})
	result, err := storage.Create(testCtx, bsar, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	resultBSAR := result.(*v1alpha1.BulkSubjectAccessReview)

	assert.True(t, resultBSAR.Status.Results[0].Allowed, "should be able to get /healthz")
	assert.True(t, resultBSAR.Status.Results[1].Allowed, "should be able to get /version")
	assert.False(t, resultBSAR.Status.Results[2].Allowed, "should not be able to get /metrics")
}

// TestIntegration_LargeBulkRequest tests handling of large bulk requests
func TestIntegration_LargeBulkRequest(t *testing.T) {
	adminRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "viewer"},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	adminBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "viewer-binding"},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "viewer",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "viewer-user"},
		},
	}

	fakeClient := fake.NewSimpleClientset(adminRole, adminBinding)
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	rbacAuth := rbacadapter.NewRBACAuthorizer(informerFactory)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	storage := registry.NewBulkSARStorage(rbacAuth)

	// Create 100 requests (simulating real frontend usage)
	requests := make([]v1alpha1.SubjectAccessReviewRequest, 100)
	resources := []string{"pods", "services", "deployments", "configmaps", "secrets"}
	verbs := []string{"get", "list", "create", "delete", "update"}

	for i := 0; i < 100; i++ {
		requests[i] = v1alpha1.SubjectAccessReviewRequest{
			ResourceAttributes: &v1alpha1.ResourceAttributes{
				Namespace: "default",
				Verb:      verbs[i%len(verbs)],
				Resource:  resources[i%len(resources)],
			},
		}
	}

	bsar := &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			Requests: requests,
		},
	}

	testCtx := createTestContext("viewer-user", nil)

	start := time.Now()
	result, err := storage.Create(testCtx, bsar, nil, &metav1.CreateOptions{})
	duration := time.Since(start)

	require.NoError(t, err)
	resultBSAR := result.(*v1alpha1.BulkSubjectAccessReview)
	require.Len(t, resultBSAR.Status.Results, 100)

	// Count allowed (get, list, watch) vs denied (create, delete, update)
	allowed := 0
	denied := 0
	for _, r := range resultBSAR.Status.Results {
		if r.Allowed {
			allowed++
		} else {
			denied++
		}
	}

	// Viewer should be able to do get/list/watch but not create/delete/update
	// 100 requests, 5 verbs rotating: get(20), list(20), create(20), delete(20), update(20)
	assert.Equal(t, 40, allowed, "should have 40 allowed (get, list)")
	assert.Equal(t, 60, denied, "should have 60 denied (create, delete, update)")

	// Performance check - should complete in reasonable time
	assert.Less(t, duration, 5*time.Second, "100 checks should complete in under 5 seconds")
	t.Logf("100 authorization checks completed in %v", duration)
}

// TestIntegration_SubjectAccessReport walks the whole SubjectAccessReport path
// with fake clients: informers -> resolver -> REST storage. It is the guard
// against wiring regressions that unit tests of the parts cannot catch.
func TestIntegration_SubjectAccessReport(t *testing.T) {
	objects := []runtime.Object{
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "d8:system:manager",
				Labels:      map[string]string{"rbac.deckhouse.io/kind": "role", "rbac.deckhouse.io/scope": "system"},
				Annotations: map[string]string{"en.meta.deckhouse.io/title": "System Manager"},
			},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list"}},
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "managers"},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "d8:system:manager"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: "ops"}},
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "local-editor", Namespace: "team-a"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"update"}},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "local-editor", Namespace: "team-a"},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: "local-editor"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "alice"}},
		},
	}

	client := fake.NewSimpleClientset(objects...)
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	rbacInformers := informerFactory.Rbac().V1()

	subjectAccess := resolver.NewSubjectAccessResolver(
		rbacInformers.Roles().Lister(),
		rbacInformers.RoleBindings().Lister(),
		rbacInformers.ClusterRoles().Lister(),
		rbacInformers.ClusterRoleBindings().Lister(),
		nil,
		nil,
		nil,
	)

	stopCh := make(chan struct{})
	defer close(stopCh)
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	storages := registry.GetStorage(registry.Storages{
		Authorizer:    rbacadapter.NewRBACAuthorizer(informerFactory),
		SubjectAccess: subjectAccess,
	})

	storage, ok := storages["subjectaccessreports"].(*registry.SubjectAccessStorage)
	require.True(t, ok, "SubjectAccessReport storage must be registered")

	// The caller reports on somebody else, which the RBAC authorizer above
	// denies: only a self report is possible without an explicit grant.
	_, err := storage.Create(createTestContext("observer", nil), &v1alpha1.SubjectAccessReport{
		Spec: v1alpha1.SubjectAccessReportSpec{
			Subject: &v1alpha1.SubjectReference{Kind: v1alpha1.SubjectKindUser, Name: "alice"},
		},
	}, nil, nil)
	require.Error(t, err)
	assert.True(t, apierrors.IsForbidden(err))

	result, err := storage.Create(createTestContext("alice", []string{"ops"}), &v1alpha1.SubjectAccessReport{}, nil, nil)
	require.NoError(t, err)

	report, ok := result.(*v1alpha1.SubjectAccessReport)
	require.True(t, ok)

	assert.Equal(t, "alice", report.Status.Subject.Name)
	require.Len(t, report.Status.RoleAssignments, 2)

	var clusterScope, namespaceScope *v1alpha1.AccessScope
	for i := range report.Status.Scopes {
		if report.Status.Scopes[i].Cluster {
			clusterScope = &report.Status.Scopes[i]
			continue
		}
		namespaceScope = &report.Status.Scopes[i]
	}

	require.NotNil(t, clusterScope, "the group grant must produce a cluster scope")
	require.Len(t, clusterScope.Resources, 1)
	assert.Equal(t, "pods", clusterScope.Resources[0].Resource)
	require.Len(t, clusterScope.Resources[0].Sources, 1)
	assert.Equal(t, "Group", clusterScope.Resources[0].Sources[0].MatchedBy.Kind)
	assert.Equal(t, "System Manager", clusterScope.Resources[0].Sources[0].Role.Titles["en"])

	require.NotNil(t, namespaceScope, "the RoleBinding must produce a namespace scope")
	assert.Equal(t, []string{"team-a"}, namespaceScope.Namespaces)
	require.Len(t, namespaceScope.Resources, 1)
	assert.Equal(t, "deployments", namespaceScope.Resources[0].Resource)
	assert.Equal(t, "User", namespaceScope.Resources[0].Sources[0].MatchedBy.Kind)
}

// Helper functions and mocks

func createTestContext(userName string, groups []string) context.Context {
	userInfo := &user.DefaultInfo{
		Name:   userName,
		Groups: groups,
	}
	return request.WithUser(context.Background(), userInfo)
}

// mockMultitenancyAuthorizer denies access to specific namespaces
type mockMultitenancyAuthorizer struct {
	denyNamespaces []string
}

func (m *mockMultitenancyAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	ns := attrs.GetNamespace()
	for _, denied := range m.denyNamespaces {
		if ns == denied {
			return authorizer.DecisionDeny, "multi-tenancy: namespace access denied", nil
		}
	}
	return authorizer.DecisionNoOpinion, "", nil
}

// Ensure mock implements authorizer.Authorizer
var _ authorizer.Authorizer = &mockMultitenancyAuthorizer{}
