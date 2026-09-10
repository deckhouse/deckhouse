/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"fmt"
	"io"
	"log"
	"reflect"
	"sort"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	kcache "k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/go_lib/user-authz/binding"
	"github.com/deckhouse/deckhouse/go_lib/user-authz/rules"
)

func newTestRBACEvaluator(t *testing.T, objs ...runtime.Object) *RBACEvaluator {
	t.Helper()

	client := fake.NewSimpleClientset(objs...)
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	evaluator, err := NewRBACEvaluator(log.New(io.Discard, "", 0), informerFactory)
	if err != nil {
		t.Fatalf("build the RBAC evaluator: %v", err)
	}

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

	limited := rulesFor(
		rules.Rule{Name: "alice", Subjects: []rules.Subject{{Kind: "User", Name: "alice"}}, LimitNamespaces: []string{"limited-.*"}},
		rules.Rule{Name: "bob", Subjects: []rules.Subject{{Kind: "User", Name: "bob"}}, LimitNamespaces: []string{"limited-.*"}},
	)
	newHandler := func() *Handler {
		return &Handler{
			logger: log.New(io.Discard, "", 0),
			cache: &dummyCache{
				data: map[string]map[string]bool{
					"v1": {"pods": true},
				},
			},
			rules:           limited,
			bindings:        binding.NewIndex(),
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
				Reason: noNamespaceAccessReason,
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
				Reason: noNamespaceAccessReason,
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
				Reason: noNamespaceAccessReason,
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
				Reason: noNamespaceAccessReason,
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
				Reason: noNamespaceAccessReason,
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

// plainCRB is a ClusterRoleBinding that no ClusterAuthorizationRule generated.
func plainCRB(name string, subjects ...rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Subjects:   subjects,
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "some-role"},
	}
}

// indexedFor returns the names of the bindings the index answers for a request, sorted.
func indexedFor(idx *independentCRBIndex, user string, groups ...string) []string {
	var names []string
	for _, crb := range idx.forRequest(user, groups) {
		names = append(names, crb.Name)
	}
	sort.Strings(names)
	return names
}

// The index has to answer exactly what a full scan with subjectsMatch would: the bindings naming
// the user, its groups, or its ServiceAccount identity, and nothing else.
func TestIndependentCRBIndex_LookupBySubjectKind(t *testing.T) {
	idx := newIndependentCRBIndex()
	idx.upsert(plainCRB("by-user", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}))
	idx.upsert(plainCRB("by-group", rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "devs"}))
	idx.upsert(plainCRB("by-sa", rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "runner", Namespace: "ci"}))
	idx.upsert(plainCRB("someone-else", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "bob"}))

	if got := indexedFor(idx, "alice"); !reflect.DeepEqual(got, []string{"by-user"}) {
		t.Errorf("user lookup: got %v", got)
	}
	if got := indexedFor(idx, "alice", "devs"); !reflect.DeepEqual(got, []string{"by-group", "by-user"}) {
		t.Errorf("user and group lookup: got %v", got)
	}
	if got := indexedFor(idx, "system:serviceaccount:ci:runner"); !reflect.DeepEqual(got, []string{"by-sa"}) {
		t.Errorf("service account lookup: got %v", got)
	}
	if got := indexedFor(idx, "nobody", "no-group"); got != nil {
		t.Errorf("an unbound subject must match nothing, got %v", got)
	}
}

// A binding naming both the user and one of its groups is evaluated once, not twice.
func TestIndependentCRBIndex_DeduplicatesAcrossSubjects(t *testing.T) {
	idx := newIndependentCRBIndex()
	idx.upsert(plainCRB("both",
		rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"},
		rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "devs"},
	))

	if got := indexedFor(idx, "alice", "devs"); !reflect.DeepEqual(got, []string{"both"}) {
		t.Errorf("expected the binding once, got %v", got)
	}
}

// An update that moves a binding to another subject must stop answering for the old one: the
// previous contribution is withdrawn, not left behind.
func TestIndependentCRBIndex_UpdateWithdrawsOldSubjects(t *testing.T) {
	idx := newIndependentCRBIndex()
	idx.upsert(plainCRB("moving", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}))
	idx.upsert(plainCRB("moving", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "bob"}))

	if got := indexedFor(idx, "alice"); got != nil {
		t.Errorf("alice must no longer be bound, got %v", got)
	}
	if got := indexedFor(idx, "bob"); !reflect.DeepEqual(got, []string{"moving"}) {
		t.Errorf("bob must be bound, got %v", got)
	}
	if idx.len() != 1 {
		t.Errorf("an update must not duplicate the binding, index holds %d", idx.len())
	}
}

func TestIndependentCRBIndex_Delete(t *testing.T) {
	idx := newIndependentCRBIndex()
	crb := plainCRB("gone", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"})
	idx.upsert(crb)
	idx.deleteByName(crb.Name)

	if got := indexedFor(idx, "alice"); got != nil {
		t.Errorf("a deleted binding must not be returned, got %v", got)
	}
	if idx.len() != 0 {
		t.Errorf("the index must be empty, holds %d", idx.len())
	}
}

// A delete the informer did not observe directly still has to withdraw the binding.
//
// The tombstone usually carries the last known object, and the handler used to give up when it did
// not. Giving up leaves the entry in the index for the life of the process, and a stale entry here
// claims a CAR-independent grant that no longer exists - a claim that OVERRIDES the multi-tenancy
// denial, so the subject keeps namespace-wide access the deleted binding gave them.
func TestIndependentCRBIndex_TombstoneWithoutTheObject(t *testing.T) {
	for _, tc := range []struct {
		name      string
		tombstone kcache.DeletedFinalStateUnknown
	}{
		{
			name:      "carrying the object",
			tombstone: kcache.DeletedFinalStateUnknown{Key: "gone", Obj: plainCRB("gone", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"})},
		},
		{
			// A watch replaced under the informer can leave a key with something else behind it.
			name:      "carrying something else",
			tombstone: kcache.DeletedFinalStateUnknown{Key: "gone", Obj: &rbacv1.RoleBinding{}},
		},
		{
			name:      "carrying nothing",
			tombstone: kcache.DeletedFinalStateUnknown{Key: "gone"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idx := newIndependentCRBIndex()
			idx.upsert(plainCRB("gone", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}))

			idx.eventHandler().OnDelete(tc.tombstone)

			if got := indexedFor(idx, "alice"); got != nil {
				t.Errorf("the binding is still indexed after its delete: %v", got)
			}
			if idx.len() != 0 {
				t.Errorf("the index must be empty, holds %d", idx.len())
			}
		})
	}
}

// The bindings a ClusterAuthorizationRule generated are the ones whose scope this webhook
// enforces; they must never count as an independent grant, and a binding that gains the module's
// labels is withdrawn from the index.
func TestIndependentCRBIndex_ExcludesCARManaged(t *testing.T) {
	idx := newIndependentCRBIndex()
	idx.upsert(ruleBinding("user-authz:rule:admin", "alice"))
	if got := indexedFor(idx, "alice"); got != nil {
		t.Errorf("a CAR-generated binding must not be indexed, got %v", got)
	}

	// The same name without the module labels is an ordinary binding.
	idx.upsert(plainCRB("user-authz:rule:admin", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}))
	if got := indexedFor(idx, "alice"); !reflect.DeepEqual(got, []string{"user-authz:rule:admin"}) {
		t.Errorf("expected the unlabelled binding to be indexed, got %v", got)
	}

	// ...and once it gains them, it stops answering.
	idx.upsert(ruleBinding("user-authz:rule:admin", "alice"))
	if got := indexedFor(idx, "alice"); got != nil {
		t.Errorf("a binding that became CAR-generated must be withdrawn, got %v", got)
	}
}

// The informer reports a delete it could not observe directly as a tombstone.
func TestIndependentCRBIndex_EventHandlerTombstone(t *testing.T) {
	idx := newIndependentCRBIndex()
	crb := plainCRB("tombstoned", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"})

	handler := idx.eventHandler()
	handler.OnAdd(crb, false)
	if got := indexedFor(idx, "alice"); !reflect.DeepEqual(got, []string{"tombstoned"}) {
		t.Fatalf("expected the binding to be indexed, got %v", got)
	}

	handler.OnDelete(kcache.DeletedFinalStateUnknown{Key: "tombstoned", Obj: crb})
	if got := indexedFor(idx, "alice"); got != nil {
		t.Errorf("a tombstoned delete must withdraw the binding, got %v", got)
	}
}

// benchCRBs builds n ordinary ClusterRoleBindings, plus one that names the user under test, in the
// proportion a cluster with a few thousand ClusterAuthorizationRules has: most bindings belong to
// somebody else, and half of them are CAR-generated.
func benchCRBs(n int) []*rbacv1.ClusterRoleBinding {
	out := make([]*rbacv1.ClusterRoleBinding, 0, n+1)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("binding-%d", i)
		subject := rbacv1.Subject{Kind: rbacv1.UserKind, Name: fmt.Sprintf("user-%d", i)}
		if i%2 == 0 {
			out = append(out, ruleBinding(fmt.Sprintf("user-authz:rule-%d:editor", i), subject.Name))
			continue
		}
		out = append(out, plainCRB(name, subject))
	}
	out = append(out, plainCRB("the-one", rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "target-group"}))
	return out
}

// BenchmarkIndependentCRB_IndexLookup measures what a request costs now: one map lookup per
// subject of the request.
func BenchmarkIndependentCRB_IndexLookup(b *testing.B) {
	idx := newIndependentCRBIndex()
	for _, crb := range benchCRBs(20000) {
		idx.upsert(crb)
	}

	b.ReportAllocs()
	for b.Loop() {
		if got := idx.forRequest("someone", []string{"target-group"}); len(got) != 1 {
			b.Fatalf("expected the one binding, got %d", len(got))
		}
	}
}

// BenchmarkIndependentCRB_FullScan measures what a request used to cost: a scan of every
// ClusterRoleBinding in the cluster with the subject match run on each one.
func BenchmarkIndependentCRB_FullScan(b *testing.B) {
	all := benchCRBs(20000)
	spec := &WebhookResourceSpec{User: "someone", Group: []string{"target-group"}}

	b.ReportAllocs()
	for b.Loop() {
		matched := 0
		for _, crb := range all {
			if isCARManagedClusterRoleBinding(crb) {
				continue
			}
			if !subjectsMatch(crb.Subjects, spec, "") {
				continue
			}
			matched++
		}
		if matched != 1 {
			b.Fatalf("expected the one binding, got %d", matched)
		}
	}
}
