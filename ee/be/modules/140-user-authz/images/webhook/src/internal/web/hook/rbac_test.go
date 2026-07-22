/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"io"
	"log"
	"regexp"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"webhook/internal/cache"
)

func newTestRBACEvaluator(t *testing.T, objs ...runtime.Object) *RBACEvaluator {
	t.Helper()

	client := fake.NewSimpleClientset(objs...)
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	evaluator := NewRBACEvaluator(log.New(io.Discard, "", 0), informerFactory)

	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	return evaluator
}

// TestAuthorizeRequestWithIndependentRBAC covers the interaction between
// multi-tenancy denies and CAR-independent RBAC grants. The fixture:
//
//   - user "alice" has a CAR with limitNamespaces ["limited-.*"] whose
//     accessLevel Editor is materialized as the cluster-wide CRB
//     "user-authz:car0:editor" (must NOT count as independent);
//   - a plain RoleBinding in ns-d grants alice get/list pods;
//   - an AR-rendered RoleBinding in ns-g grants alice get pods;
//   - user "bob" has a CAR too, plus a user-created CRB granting list pods
//     cluster-wide.
func TestAuthorizeRequestWithIndependentRBAC(t *testing.T) {
	deckhouseLabels := map[string]string{"heritage": "deckhouse", "module": "user-authz"}

	rbacObjs := []runtime.Object{
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "user-authz:editor"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "user-authz:car0:editor", Labels: deckhouseLabels},
			Subjects: []rbacv1.Subject{
				{Kind: rbacv1.UserKind, Name: "alice"},
				{Kind: rbacv1.UserKind, Name: "bob"},
			},
			RoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "user-authz:editor"},
		},

		// Plain RoleBinding in ns-d.
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

		// AR-rendered RoleBinding in ns-g.
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "user-authz:user"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "user-authz:ar0:user", Namespace: "ns-g", Labels: deckhouseLabels},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "alice"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "user-authz:user"},
		},

		// User-created cluster-wide grant for bob.
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-lister"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"list"}},
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "bob-pod-lister"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "bob"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "pod-lister"},
		},
	}

	evaluator := newTestRBACEvaluator(t, rbacObjs...)

	limitedRegex, _ := regexp.Compile("^limited-.*$")
	newHandler := func() *Handler {
		return &Handler{
			logger: log.New(io.Discard, "", 0),
			cache: &dummyCache{
				data: map[string]map[string]bool{
					"v1": {"pods": true},
				},
				coreResources: cache.CoreResourcesDict{"pods": struct{}{}},
			},
			directory: map[string]map[string]DirectoryEntry{
				"User": {
					"alice": {LimitNamespaces: []*regexp.Regexp{limitedRegex}},
					"bob":   {LimitNamespaces: []*regexp.Regexp{limitedRegex}},
				},
			},
			nsLister:        newFakeNamespaceLister(nil),
			nsSynced:        func() bool { return true },
			independentRBAC: evaluator,
		}
	}

	tc := []struct {
		Name         string
		User         string
		Attributes   WebhookResourceAttributes
		ResultStatus WebhookRequestStatus
	}{
		{
			Name: "CAR namespace stays allowed (RBAC will apply the CAR accessLevel)",
			User: "alice",
			Attributes: WebhookResourceAttributes{
				Version: "v1", Resource: "pods", Verb: "delete", Namespace: "limited-ns",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name: "RoleBinding grant outside CAR scope is not denied",
			User: "alice",
			Attributes: WebhookResourceAttributes{
				Version: "v1", Resource: "pods", Verb: "get", Namespace: "ns-d",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name: "verb beyond the RoleBinding is denied - CAR accessLevel must not leak into ns-d",
			User: "alice",
			Attributes: WebhookResourceAttributes{
				Version: "v1", Resource: "pods", Verb: "delete", Namespace: "ns-d",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "user has no access to the namespace",
			},
		},
		{
			Name: "AR-rendered RoleBinding grant is not denied",
			User: "alice",
			Attributes: WebhookResourceAttributes{
				Version: "v1", Resource: "pods", Verb: "get", Namespace: "ns-g",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name: "verb beyond the AR is denied - CAR accessLevel must not leak into ns-g",
			User: "alice",
			Attributes: WebhookResourceAttributes{
				Version: "v1", Resource: "pods", Verb: "delete", Namespace: "ns-g",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "user has no access to the namespace",
			},
		},
		{
			Name: "namespace without any grant is denied",
			User: "alice",
			Attributes: WebhookResourceAttributes{
				Version: "v1", Resource: "pods", Verb: "get", Namespace: "ns-f",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "user has no access to the namespace",
			},
		},
		{
			Name: "subresource is not granted by a bare resource rule",
			User: "alice",
			Attributes: WebhookResourceAttributes{
				Version: "v1", Resource: "pods", Subresource: "exec", Verb: "get", Namespace: "ns-d",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "user has no access to the namespace",
			},
		},
		{
			Name: "cluster-scoped list of a namespaced resource is denied without independent grants",
			User: "alice",
			Attributes: WebhookResourceAttributes{
				Version: "v1", Resource: "pods", Verb: "list",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "making cluster-scoped requests for namespaced resources is not allowed",
			},
		},
		{
			Name: "cluster-scoped list granted by a user-created ClusterRoleBinding is not denied",
			User: "bob",
			Attributes: WebhookResourceAttributes{
				Version: "v1", Resource: "pods", Verb: "list",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name: "user-created ClusterRoleBinding also unlocks namespaced requests outside CAR scope",
			User: "bob",
			Attributes: WebhookResourceAttributes{
				Version: "v1", Resource: "pods", Verb: "list", Namespace: "ns-f",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name: "CAR-generated ClusterRoleBinding does not unlock foreign namespaces",
			User: "bob",
			Attributes: WebhookResourceAttributes{
				Version: "v1", Resource: "pods", Verb: "delete", Namespace: "ns-f",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "user has no access to the namespace",
			},
		},
	}

	for _, testCase := range tc {
		t.Run(testCase.Name, func(t *testing.T) {
			req := &WebhookRequest{
				Spec: WebhookResourceSpec{
					User:               testCase.User,
					ResourceAttributes: testCase.Attributes,
				},
			}

			req = newHandler().authorizeRequest(req)

			if req.Status.Denied != testCase.ResultStatus.Denied {
				t.Errorf("denied: got %v | expected %v", req.Status.Denied, testCase.ResultStatus.Denied)
			}
			if req.Status.Reason != testCase.ResultStatus.Reason {
				t.Errorf("reason: got %q | expected %q", req.Status.Reason, testCase.ResultStatus.Reason)
			}
		})
	}
}

// TestRBACEvaluatorUnsyncedCaches ensures the evaluator fails closed while
// informer caches are not synced.
func TestRBACEvaluatorUnsyncedCaches(t *testing.T) {
	evaluator := newTestRBACEvaluator(t)
	evaluator.synced = append(evaluator.synced, func() bool { return false })

	spec := &WebhookResourceSpec{
		User: "alice",
		ResourceAttributes: WebhookResourceAttributes{
			Version: "v1", Resource: "pods", Verb: "get", Namespace: "ns-d",
		},
	}
	if evaluator.AllowsIndependently(spec) {
		t.Error("expected the evaluator to fail closed with unsynced caches")
	}
}
