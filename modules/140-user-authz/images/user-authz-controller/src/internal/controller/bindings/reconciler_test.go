/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bindings

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus/testutil"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "user-authz-controller/api/v1"
	"user-authz-controller/api/v1alpha1"
	"user-authz-controller/internal/desired"
	"user-authz-controller/internal/metrics"
)

var testSubjects = []rbacv1.Subject{{Kind: "User", Name: "jane"}}

// writes records every write the reconciler issues, in order, as "<verb> <name>".
type writes struct {
	mu  sync.Mutex
	ops []string
}

func (w *writes) add(verb, name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ops = append(w.ops, verb+" "+name)
}

func (w *writes) count(verb string) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for _, op := range w.ops {
		if strings.HasPrefix(op, verb+" ") {
			n++
		}
	}
	return n
}

func (w *writes) total() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.ops)
}

func (w *writes) snapshot() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.ops...)
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, v1.AddToScheme, v1alpha1.AddToScheme} {
		if err := add(s); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// newBuilder prepares a fake client the way the manager does: status subresources on the rules and
// the rule index on both binding kinds. Every write is recorded in w; a test may pass its own
// interceptor funcs, which run after the recording (WithInterceptorFuncs replaces, so the recorder
// is composed here rather than chained by the caller).
func newBuilder(t *testing.T, w *writes, extra interceptor.Funcs, objs ...client.Object) *fake.ClientBuilder {
	t.Helper()
	return fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithStatusSubresource(&v1.ClusterAuthorizationRule{}, &v1alpha1.AuthorizationRule{}).
		WithIndex(&rbacv1.ClusterRoleBinding{}, RuleIndexField, RuleIndexValue).
		WithIndex(&rbacv1.RoleBinding{}, RuleIndexField, RuleIndexValue).
		WithObjects(objs...).
		WithInterceptorFuncs(recording(w, extra))
}

// recording wraps extra so that every write is recorded before extra (or the fake client) handles it.
func recording(w *writes, extra interceptor.Funcs) interceptor.Funcs {
	funcs := extra
	funcs.Create = func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
		w.add("create", obj.GetName())
		if extra.Create != nil {
			return extra.Create(ctx, c, obj, opts...)
		}
		return c.Create(ctx, obj, opts...)
	}
	funcs.Update = func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
		w.add("update", obj.GetName())
		if extra.Update != nil {
			return extra.Update(ctx, c, obj, opts...)
		}
		return c.Update(ctx, obj, opts...)
	}
	funcs.Delete = func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
		w.add("delete", obj.GetName())
		if extra.Delete != nil {
			return extra.Delete(ctx, c, obj, opts...)
		}
		return c.Delete(ctx, obj, opts...)
	}
	funcs.SubResourcePatch = func(ctx context.Context, c client.Client, sub string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
		w.add("status", obj.GetName())
		if extra.SubResourcePatch != nil {
			return extra.SubResourcePatch(ctx, c, sub, obj, patch, opts...)
		}
		return c.SubResource(sub).Patch(ctx, obj, patch, opts...)
	}
	return funcs
}

func newClient(t *testing.T, w *writes, objs ...client.Object) client.Client {
	t.Helper()
	return newBuilder(t, w, interceptor.Funcs{}, objs...).Build()
}

func car(name, level string, opts ...func(*v1.ClusterAuthorizationRule)) *v1.ClusterAuthorizationRule {
	c := &v1.ClusterAuthorizationRule{
		ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID("uid-" + name), Generation: 3},
		Spec:       v1.ClusterAuthorizationRuleSpec{AccessLevel: level, Subjects: testSubjects},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

func ar(ns, name, level string) *v1alpha1.AuthorizationRule {
	return &v1alpha1.AuthorizationRule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, UID: types.UID("uid-" + ns + "-" + name), Generation: 1},
		Spec:       v1alpha1.AuthorizationRuleSpec{AccessLevel: level, Subjects: testSubjects},
	}
}

func moduleLabels() map[string]string {
	return map[string]string{desired.LabelHeritage: desired.HeritageValue, desired.LabelModule: desired.ModuleName}
}

// helmBinding mimics a ClusterRoleBinding rendered by the module chart before the controller.
func helmBinding(name, role string) *rbacv1.ClusterRoleBinding {
	labels := moduleLabels()
	labels["app.kubernetes.io/managed-by"] = "Helm"
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
			Annotations: map[string]string{
				"meta.helm.sh/release-name":      "user-authz",
				"meta.helm.sh/release-namespace": "d8-system",
				"helm.sh/resource-policy":        "keep",
			},
		},
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: role},
		Subjects: testSubjects,
	}
}

func clusterReconciler(c client.Client) *Reconciler {
	return NewCluster(c, c, events.NewFakeRecorder(10), metrics.New(), logr.Discard())
}

func namespacedReconciler(c client.Client) *Reconciler {
	return NewNamespaced(c, c, events.NewFakeRecorder(10), metrics.New(), logr.Discard())
}

func reconcileCluster(t *testing.T, c client.Client, name string) reconcile.Result {
	t.Helper()
	res, err := clusterReconciler(c).Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	return res
}

func reconcileNamespaced(t *testing.T, c client.Client, ns, name string) {
	t.Helper()
	if _, err := namespacedReconciler(c).Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: ns}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func crbNames(t *testing.T, c client.Client) map[string]rbacv1.ClusterRoleBinding {
	t.Helper()
	list := &rbacv1.ClusterRoleBindingList{}
	if err := c.List(t.Context(), list); err != nil {
		t.Fatal(err)
	}
	out := map[string]rbacv1.ClusterRoleBinding{}
	for _, item := range list.Items {
		out[item.Name] = item
	}
	return out
}

func readyCondition(t *testing.T, c client.Client, name string) (*metav1.Condition, v1.RuleStatus) {
	t.Helper()
	got := &v1.ClusterAuthorizationRule{}
	if err := c.Get(t.Context(), types.NamespacedName{Name: name}, got); err != nil {
		t.Fatal(err)
	}
	return apimeta.FindStatusCondition(got.Status.Conditions, ConditionReady), got.Status
}

func TestReconcile_CreatesBindingsWithOwnerAndStatus(t *testing.T) {
	t.Parallel()
	w := &writes{}
	c := newClient(t, w, car("dev", desired.AccessLevelEditor, func(r *v1.ClusterAuthorizationRule) {
		r.Spec.PortForwarding = true
	}))

	reconcileCluster(t, c, "dev")

	crbs := crbNames(t, c)
	for _, name := range []string{"user-authz:dev:editor", "user-authz:dev:editor:custom", "user-authz:dev:port-forward"} {
		b, ok := crbs[name]
		if !ok {
			t.Fatalf("binding %s not created; have %v", name, keys(crbs))
		}
		if b.Labels[desired.LabelManagedBy] != desired.ManagedByValue || b.Labels[desired.LabelHeritage] != desired.HeritageValue {
			t.Errorf("%s labels = %v", name, b.Labels)
		}
		if len(b.OwnerReferences) != 1 || b.OwnerReferences[0].UID != "uid-dev" || b.OwnerReferences[0].Kind != "ClusterAuthorizationRule" {
			t.Errorf("%s owner = %v", name, b.OwnerReferences)
		}
	}
	if crbs["user-authz:dev:editor:custom"].Labels[desired.LabelBindingKind] != desired.BindingKindAggregate {
		t.Error("aggregated binding must carry the binding-kind label")
	}
	if len(crbs) != 3 {
		t.Errorf("expected exactly 3 bindings, got %v", keys(crbs))
	}

	cond, status := readyCondition(t, c, "dev")
	if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != ReasonBindingsApplied {
		t.Fatalf("Ready condition = %+v", cond)
	}
	if status.Bindings != 3 || status.ObservedGeneration != 3 {
		t.Errorf("status = %+v", status)
	}
	if w.count("create") != 3 || w.count("update") != 0 || w.count("delete") != 0 || w.count("status") != 1 {
		t.Errorf("writes = %v", w.snapshot())
	}
}

func TestReconcile_IsIdempotentIncludingStatus(t *testing.T) {
	t.Parallel()
	w := &writes{}
	c := newClient(t, w, car("dev", desired.AccessLevelUser))

	reconcileCluster(t, c, "dev")
	before := w.total()

	reconcileCluster(t, c, "dev")
	if after := w.total(); after != before {
		t.Fatalf("second reconcile performed writes: %v", w.snapshot()[before:])
	}
}

func TestReconcile_AdoptsHelmBindingsInPlace(t *testing.T) {
	t.Parallel()
	w := &writes{}
	c := newClient(t, w,
		car("dev", desired.AccessLevelEditor),
		helmBinding("user-authz:dev:editor", "user-authz:editor"),
		helmBinding("user-authz:dev:editor:custom", "user-authz:editor:custom"),
	)

	reconcileCluster(t, c, "dev")

	crbs := crbNames(t, c)
	adopted := crbs["user-authz:dev:editor"]
	if _, helm := adopted.Labels["app.kubernetes.io/managed-by"]; helm {
		t.Error("Helm label must be removed on adoption")
	}
	if len(adopted.Annotations) != 0 {
		t.Errorf("Helm annotations must be removed on adoption, got %v", adopted.Annotations)
	}
	if adopted.Labels[desired.LabelManagedBy] != desired.ManagedByValue || len(adopted.OwnerReferences) != 1 {
		t.Errorf("adopted binding = %+v", adopted.ObjectMeta)
	}
	if w.count("create") != 0 || w.count("update") != 2 || w.count("delete") != 0 {
		t.Errorf("writes = %v, want two in-place updates", w.snapshot())
	}
}

func TestReconcile_PreservesForeignAnnotationsAndLabels(t *testing.T) {
	t.Parallel()
	w := &writes{}
	b := helmBinding("user-authz:dev:user", "user-authz:user")
	b.Annotations["example.com/added-by-admission"] = "yes"
	b.Labels["example.com/team"] = "blue"
	c := newClient(t, w, car("dev", desired.AccessLevelUser), b)

	reconcileCluster(t, c, "dev")
	got := crbNames(t, c)["user-authz:dev:user"]
	if got.Annotations["example.com/added-by-admission"] != "yes" {
		t.Errorf("foreign annotation must survive adoption, got %v", got.Annotations)
	}
	if got.Labels["example.com/team"] != "blue" {
		t.Errorf("foreign label must survive adoption, got %v", got.Labels)
	}
	if _, helm := got.Labels[helmManagedByLabel]; helm {
		t.Error("the Helm ownership label must be removed")
	}
	for _, key := range helmAnnotations {
		if _, present := got.Annotations[key]; present {
			t.Errorf("%s must be removed", key)
		}
	}

	// A binding that only carries a foreign annotation is in the desired state: no write loop.
	before := w.total()
	reconcileCluster(t, c, "dev")
	if w.total() != before {
		t.Errorf("foreign annotation caused writes: %v", w.snapshot()[before:])
	}
}

func TestReconcile_RemovesLegacyPerRoleBindingsAfterAggregatedExists(t *testing.T) {
	t.Parallel()
	w := &writes{}
	c := newClient(t, w,
		car("dev", desired.AccessLevelEditor),
		helmBinding("user-authz:dev:editor", "user-authz:editor"),
		helmBinding("user-authz:dev:editor:custom-cluster-role:d8:user-authz:istio:editor", "d8:user-authz:istio:editor"),
		helmBinding("user-authz:dev:editor:custom-cluster-role:d8:user-authz:istio:user", "d8:user-authz:istio:user"),
	)

	reconcileCluster(t, c, "dev")

	crbs := crbNames(t, c)
	if _, ok := crbs["user-authz:dev:editor:custom"]; !ok {
		t.Fatal("aggregated binding must be created")
	}
	for name := range crbs {
		if strings.HasPrefix(name, "user-authz:dev:editor:custom-cluster-role:") {
			t.Errorf("legacy per-role binding %s must be removed", name)
		}
	}
	if w.count("delete") != 2 {
		t.Errorf("deletes = %d, want 2", w.count("delete"))
	}

	// The replacement is written before anything is removed.
	ops := w.snapshot()
	firstDelete, lastWrite := len(ops), -1
	for i, op := range ops {
		switch {
		case strings.HasPrefix(op, "delete "):
			firstDelete = min(firstDelete, i)
		case strings.HasPrefix(op, "create ") || strings.HasPrefix(op, "update "):
			lastWrite = max(lastWrite, i)
		}
	}
	if lastWrite > firstDelete {
		t.Errorf("a delete happened before the last create/update: %v", ops)
	}
}

func TestReconcile_LeavesForeignBindingsAlone(t *testing.T) {
	t.Parallel()
	w := &writes{}
	foreign := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "user-authz:dev:something-else"}, // no module labels
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "view"},
		Subjects:   testSubjects,
	}
	other := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "user-authz:devops:user", Labels: moduleLabels()}, // another rule
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "user-authz:user"},
		Subjects:   testSubjects,
	}
	c := newClient(t, w, car("dev", desired.AccessLevelUser), foreign, other)

	reconcileCluster(t, c, "dev")

	crbs := crbNames(t, c)
	if _, ok := crbs["user-authz:dev:something-else"]; !ok {
		t.Error("binding without module labels must not be touched")
	}
	if got := crbs["user-authz:devops:user"]; got.Labels[desired.LabelManagedBy] != "" {
		t.Error("binding of another rule must not be touched by this reconcile")
	}
	if w.count("delete") != 0 {
		t.Errorf("deletes = %d, want 0", w.count("delete"))
	}
}

func TestReconcile_DeletedRuleCleansUpInheritedBindings(t *testing.T) {
	t.Parallel()
	w := &writes{}
	c := newClient(t, w,
		helmBinding("user-authz:gone:editor", "user-authz:editor"),
		helmBinding("user-authz:gone:editor:custom-cluster-role:d8:user-authz:istio:editor", "d8:user-authz:istio:editor"),
		helmBinding("user-authz:gone-but-longer:editor", "user-authz:editor"),
	)

	reconcileCluster(t, c, "gone")

	crbs := crbNames(t, c)
	if len(crbs) != 1 {
		t.Fatalf("bindings left: %v", keys(crbs))
	}
	if _, ok := crbs["user-authz:gone-but-longer:editor"]; !ok {
		t.Error("the rule index must stop at the rule name separator")
	}
}

// A binding event can reach the queue before the informer of the rules has seen the new rule.
// The reconciler must not treat the cache miss as a deleted rule.
func TestReconcile_CacheMissOnLiveRuleRequeuesInsteadOfDeleting(t *testing.T) {
	t.Parallel()
	w := &writes{}
	rule := car("dev", desired.AccessLevelUser)
	cached := newClient(t, w, helmBinding("user-authz:dev:user", "user-authz:user")) // no rule in "cache"
	live := newClient(t, &writes{}, rule)                                            // rule visible to the API reader

	r := NewCluster(cached, live, events.NewFakeRecorder(10), metrics.New(), logr.Discard())
	_, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "dev"}})
	if err == nil || errors.Is(err, reconcile.TerminalError(nil)) {
		t.Fatalf("err = %v: a live rule missing from the cache must be retried with backoff", err)
	}
	if w.count("delete") != 0 {
		t.Errorf("bindings of a live rule were deleted: %v", w.snapshot())
	}
}

// The API server defaults apiGroup on subjects; a binding that already matches the rule must not be
// rewritten on every reconcile because of it.
func TestReconcile_APIDefaultedSubjectsCauseNoWrites(t *testing.T) {
	t.Parallel()
	w := &writes{}
	rule := car("dev", desired.AccessLevelUser)
	converged := make([]client.Object, 0, 2)
	for _, b := range mustBindings(t, desired.FromClusterAuthorizationRule(rule)) {
		crb := desired.ClusterRoleBinding(b, desired.OwnerReference(desired.FromClusterAuthorizationRule(rule)))
		for i := range crb.Subjects { // what the API server stores for a User subject
			crb.Subjects[i].APIGroup = rbacv1.GroupName
		}
		converged = append(converged, crb)
	}
	c := newClient(t, w, append(converged, rule)...)

	reconcileCluster(t, c, "dev")

	if n := w.count("create") + w.count("update") + w.count("delete"); n != 0 {
		t.Fatalf("a converged rule caused writes: %v", w.snapshot())
	}
}

// A binding whose module labels were stripped (a mutating policy, a hand edit) still carries the
// controller's ownerReference: it is ours and is repaired, not refused.
func TestReconcile_BindingWithoutLabelsButOwnedByTheRuleIsRepaired(t *testing.T) {
	t.Parallel()
	w := &writes{}
	rule := car("dev", desired.AccessLevelUser)
	stripped := desired.ClusterRoleBinding(mustBindings(t, desired.FromClusterAuthorizationRule(rule))[0], desired.OwnerReference(desired.FromClusterAuthorizationRule(rule)))
	stripped.Labels = map[string]string{"example.com/team": "blue"}
	c := newClient(t, w, rule, stripped)

	reconcileCluster(t, c, "dev")

	got := crbNames(t, c)["user-authz:dev:user"]
	if got.Labels[desired.LabelModule] != desired.ModuleName || got.Labels[desired.LabelManagedBy] != desired.ManagedByValue || got.Labels["example.com/team"] != "blue" {
		t.Errorf("labels = %v, want the module labels restored and the foreign one kept", got.Labels)
	}
	if w.count("create") != 1 || w.count("update") != 1 { // :custom created, the stripped one repaired
		t.Errorf("writes = %v", w.snapshot())
	}
}

func TestReconcile_InvalidSpecReportsConditionAndKeepsBindings(t *testing.T) {
	t.Parallel()
	w := &writes{}
	rule := ar("team", "dev", desired.AccessLevelClusterAdmin) // not allowed for a namespaced rule
	existing := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "user-authz:dev:admin", Namespace: "team", Labels: moduleLabels()},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "user-authz:admin"},
		Subjects:   testSubjects,
	}
	c := newClient(t, w, rule, existing)

	reconcileNamespaced(t, c, "team", "dev")

	got := &v1alpha1.AuthorizationRule{}
	if err := c.Get(t.Context(), types.NamespacedName{Name: "dev", Namespace: "team"}, got); err != nil {
		t.Fatal(err)
	}
	cond := apimeta.FindStatusCondition(got.Status.Conditions, ConditionReady)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != ReasonInvalidSpec {
		t.Fatalf("Ready condition = %+v", cond)
	}
	if w.count("create")+w.count("update")+w.count("delete") != 0 {
		t.Error("an invalid rule must not change any binding")
	}
	rb := &rbacv1.RoleBinding{}
	if err := c.Get(t.Context(), types.NamespacedName{Name: "user-authz:dev:admin", Namespace: "team"}, rb); err != nil {
		t.Errorf("existing binding must survive an invalid spec: %v", err)
	}
}

func TestReconcile_NamespacedRuleRendersRoleBindings(t *testing.T) {
	t.Parallel()
	w := &writes{}
	c := newClient(t, w, ar("team", "dev", desired.AccessLevelEditor))

	reconcileNamespaced(t, c, "team", "dev")

	rb := &rbacv1.RoleBinding{}
	if err := c.Get(t.Context(), types.NamespacedName{Name: "user-authz:dev:editor:custom", Namespace: "team"}, rb); err != nil {
		t.Fatalf("aggregated rolebinding: %v", err)
	}
	if rb.RoleRef.Kind != "ClusterRole" || rb.RoleRef.Name != "user-authz:editor:custom" {
		t.Errorf("roleRef = %+v", rb.RoleRef)
	}
	if rb.OwnerReferences[0].Kind != "AuthorizationRule" {
		t.Errorf("owner = %+v", rb.OwnerReferences)
	}
	crb := &rbacv1.ClusterRoleBinding{}
	if err := c.Get(t.Context(), types.NamespacedName{Name: "user-authz:dev:editor"}, crb); !apierrors.IsNotFound(err) {
		t.Error("a namespaced rule must not produce ClusterRoleBindings")
	}
}

// Two AuthorizationRules with the same name in different namespaces must not see each other's
// bindings.
func TestReconcile_NamespacedRulesAreIsolatedByNamespace(t *testing.T) {
	t.Parallel()
	w := &writes{}
	c := newClient(t, w, ar("team-a", "dev", desired.AccessLevelUser), ar("team-b", "dev", desired.AccessLevelEditor))

	reconcileNamespaced(t, c, "team-a", "dev")
	reconcileNamespaced(t, c, "team-b", "dev")

	list := &rbacv1.RoleBindingList{}
	if err := c.List(t.Context(), list, client.InNamespace("team-a")); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("team-a bindings = %d, want 2", len(list.Items))
	}
	for _, rb := range list.Items {
		if rb.OwnerReferences[0].UID != "uid-team-a-dev" {
			t.Errorf("%s owned by %s", rb.Name, rb.OwnerReferences[0].UID)
		}
	}
	if w.count("delete") != 0 {
		t.Errorf("a rule deleted bindings of its namesake: %v", w.snapshot())
	}
}

func TestReconcile_SubjectDriftIsRepaired(t *testing.T) {
	t.Parallel()
	w := &writes{}
	drifted := helmBinding("user-authz:dev:user", "user-authz:user")
	drifted.Subjects = []rbacv1.Subject{{Kind: "User", Name: "someone-else"}}
	c := newClient(t, w, car("dev", desired.AccessLevelUser), drifted)

	reconcileCluster(t, c, "dev")

	got := crbNames(t, c)["user-authz:dev:user"]
	if len(got.Subjects) != 1 || got.Subjects[0].Name != "jane" {
		t.Errorf("subjects = %v, want the rule's subjects", got.Subjects)
	}
}

// A rule deleted and recreated under the same name gets a new UID; bindings still pointing at the
// old UID would be garbage-collected, so the owner reference is repaired.
func TestReconcile_OwnerUIDDriftIsRepaired(t *testing.T) {
	t.Parallel()
	w := &writes{}
	rule := car("dev", desired.AccessLevelUser)
	stale := desired.ClusterRoleBinding(mustBindings(t, desired.FromClusterAuthorizationRule(rule))[0], desired.OwnerReference(desired.FromClusterAuthorizationRule(rule)))
	stale.OwnerReferences[0].UID = "uid-of-the-previous-incarnation"
	c := newClient(t, w, rule, stale)

	reconcileCluster(t, c, "dev")

	got := crbNames(t, c)["user-authz:dev:user"]
	if len(got.OwnerReferences) != 1 || got.OwnerReferences[0].UID != "uid-dev" {
		t.Errorf("owner = %+v", got.OwnerReferences)
	}
	if w.count("update") != 1 {
		t.Errorf("writes = %v, want one update", w.snapshot())
	}
}

// The cache may not know about a binding that already exists on the API server (another worker
// or a previous incarnation created it a moment ago): AlreadyExists is resolved by adopting it.
func TestReconcile_AlreadyExistsIsAdopted(t *testing.T) {
	t.Parallel()
	w := &writes{}
	live := newClient(t, &writes{}, helmBinding("user-authz:dev:user", "user-authz:user"))
	cached := newBuilder(t, w, interceptor.Funcs{
		Create: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
			if obj.GetName() == "user-authz:dev:user" {
				return apierrors.NewAlreadyExists(schema.GroupResource{Group: rbacv1.GroupName, Resource: "clusterrolebindings"}, obj.GetName())
			}
			return nil
		},
		Update: func(ctx context.Context, _ client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			return live.Update(ctx, obj, opts...)
		},
	}, car("dev", desired.AccessLevelUser)).Build()

	// The API read after the conflict must go into an empty object: the real client decodes into
	// whatever it is given, and a copy of the desired object would keep labels the live one lacks.
	var readInto []client.Object
	reader := interceptor.NewClient(live.(client.WithWatch), interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			readInto = append(readInto, obj.DeepCopyObject().(client.Object))
			return c.Get(ctx, key, obj, opts...)
		},
	})

	r := NewCluster(cached, reader, events.NewFakeRecorder(10), metrics.New(), logr.Discard())
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "dev"}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(readInto) != 1 || len(readInto[0].GetLabels()) != 0 || len(readInto[0].GetOwnerReferences()) != 0 {
		t.Fatalf("the conflict read must use an empty object, got %+v", readInto)
	}

	adopted := crbNames(t, live)["user-authz:dev:user"]
	if adopted.Labels[desired.LabelManagedBy] != desired.ManagedByValue || len(adopted.Annotations) != 0 {
		t.Errorf("binding was not adopted: %+v", adopted.ObjectMeta)
	}
	if w.count("update") != 1 {
		t.Errorf("writes = %v, want one adopting update", w.snapshot())
	}
}

// An object with our name but without the module labels is not ours; it is reported, not taken.
func TestReconcile_AlreadyExistsForeignObjectIsTerminal(t *testing.T) {
	t.Parallel()
	w := &writes{}
	foreign := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "user-authz:dev:user"},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "cluster-admin"},
		Subjects:   testSubjects,
	}
	live := newClient(t, &writes{}, foreign)
	cached := newBuilder(t, w, interceptor.Funcs{
		Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if obj.GetName() == "user-authz:dev:user" {
				return apierrors.NewAlreadyExists(schema.GroupResource{Group: rbacv1.GroupName, Resource: "clusterrolebindings"}, obj.GetName())
			}
			return cl.Create(ctx, obj, opts...)
		},
	}, car("dev", desired.AccessLevelUser)).Build()

	r := NewCluster(cached, live, events.NewFakeRecorder(10), metrics.New(), logr.Discard())
	_, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "dev"}})
	if !errors.Is(err, reconcile.TerminalError(nil)) {
		t.Fatalf("err = %v, want a terminal error", err)
	}
	if got := crbNames(t, live)["user-authz:dev:user"]; got.RoleRef.Name != "cluster-admin" || len(got.Labels) != 0 {
		t.Errorf("foreign object was modified: %+v", got)
	}
}

// A binding under a controller name whose roleRef differs was planted or edited by hand (the name
// fixes the roleRef). It is replaced, and the reconcile of the rule goes on: refusing to touch it
// would let a namespace admin block the removal of bindings the rule no longer grants.
func TestReconcile_ForeignRoleRefIsRecreatedAndDoesNotBlockDeletions(t *testing.T) {
	t.Parallel()
	w := &writes{}
	planted := helmBinding("user-authz:dev:editor", "user-authz:admin") // must be user-authz:editor
	planted.UID = "uid-planted"
	legacy := helmBinding("user-authz:dev:editor:custom-cluster-role:d8:user-authz:istio:editor", "d8:user-authz:istio:editor")
	c := newClient(t, w, car("dev", desired.AccessLevelEditor), planted, legacy)

	reconcileCluster(t, c, "dev")

	crbs := crbNames(t, c)
	got, ok := crbs["user-authz:dev:editor"]
	if !ok || got.RoleRef.Name != "user-authz:editor" {
		t.Fatalf("binding = %+v, want it recreated with the rule's roleRef", got)
	}
	if got.Labels[desired.LabelManagedBy] != desired.ManagedByValue || len(got.OwnerReferences) != 1 {
		t.Errorf("recreated binding metadata = %+v", got.ObjectMeta)
	}
	if _, ok := crbs[legacy.Name]; ok {
		t.Error("the legacy binding must still be removed")
	}
	if w.count("delete") != 2 || w.count("create") != 2 { // planted + legacy deleted; editor + :custom created
		t.Errorf("writes = %v", w.snapshot())
	}
	cond, _ := readyCondition(t, c, "dev")
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Errorf("Ready condition = %+v", cond)
	}
}

// A create the API server rejects as invalid is terminal: the status says so, nothing is deleted,
// and the other bindings of the rule are still applied.
func TestReconcile_InvalidCreateIsTerminalAndReportedWithoutDeleting(t *testing.T) {
	t.Parallel()
	w := &writes{}
	legacy := helmBinding("user-authz:dev:editor:custom-cluster-role:d8:user-authz:istio:editor", "d8:user-authz:istio:editor")
	c := newBuilder(t, w, interceptor.Funcs{
		Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if obj.GetName() == "user-authz:dev:editor:custom" {
				return apierrors.NewInvalid(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding").GroupKind(), obj.GetName(),
					field.ErrorList{field.Invalid(field.NewPath("metadata", "name"), obj.GetName(), "rejected by an admission webhook")})
			}
			return cl.Create(ctx, obj, opts...)
		},
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			return cl.Delete(ctx, obj, opts...)
		},
	}, car("dev", desired.AccessLevelEditor), legacy).Build()

	_, err := clusterReconciler(c).Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "dev"}})
	if !errors.Is(err, reconcile.TerminalError(nil)) {
		t.Fatalf("err = %v, want a terminal error", err)
	}

	cond, status := readyCondition(t, c, "dev")
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != ReasonApplyError || !strings.Contains(cond.Message, "admission webhook") {
		t.Fatalf("Ready condition = %+v", cond)
	}
	if status.ObservedGeneration != 3 {
		t.Errorf("observedGeneration = %d", status.ObservedGeneration)
	}
	crbs := crbNames(t, c)
	if _, ok := crbs["user-authz:dev:editor"]; !ok {
		t.Error("the other bindings must still be applied after one failed")
	}
	if _, ok := crbs[legacy.Name]; !ok {
		t.Error("nothing may be deleted while a wanted binding could not be written")
	}
	if w.count("delete") != 0 {
		t.Errorf("writes = %v", w.snapshot())
	}
}

// A write conflict (stale resourceVersion) is retried, not terminal.
func TestReconcile_ConflictIsRetryable(t *testing.T) {
	t.Parallel()
	w := &writes{}
	c := newBuilder(t, w, interceptor.Funcs{
		Update: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.UpdateOption) error {
			return apierrors.NewConflict(schema.GroupResource{Group: rbacv1.GroupName, Resource: "clusterrolebindings"}, obj.GetName(), errors.New("the object has been modified"))
		},
	}, car("dev", desired.AccessLevelUser), helmBinding("user-authz:dev:user", "user-authz:user")).Build()

	_, err := clusterReconciler(c).Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "dev"}})
	if err == nil || errors.Is(err, reconcile.TerminalError(nil)) {
		t.Fatalf("err = %v, want a retryable error", err)
	}
	cond, _ := readyCondition(t, c, "dev")
	if cond == nil || cond.Reason != ReasonApplyError {
		t.Errorf("Ready condition = %+v", cond)
	}
}

// Deletes carry the UID of the object the reconciler saw, so a binding recreated under the same
// name by someone else in the meantime is not the one removed.
func TestReconcile_DeleteCarriesUIDPrecondition(t *testing.T) {
	t.Parallel()
	w := &writes{}
	legacy := helmBinding("user-authz:dev:user:custom-cluster-role:d8:user-authz:istio:user", "d8:user-authz:istio:user")
	legacy.UID = "uid-legacy"
	var seen []types.UID
	c := newBuilder(t, w, interceptor.Funcs{
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			o := &client.DeleteOptions{}
			o.ApplyOptions(opts)
			if o.Preconditions != nil && o.Preconditions.UID != nil {
				seen = append(seen, *o.Preconditions.UID)
			}
			return cl.Delete(ctx, obj, opts...)
		},
	}, car("dev", desired.AccessLevelUser), legacy).Build()

	reconcileCluster(t, c, "dev")

	if len(seen) != 1 || seen[0] != "uid-legacy" {
		t.Fatalf("delete preconditions = %v, want the UID of the legacy binding", seen)
	}
}

// A transient write error is retried (not terminal), even when another binding failed terminally.
func TestReconcile_TransientErrorWinsOverTerminal(t *testing.T) {
	t.Parallel()
	w := &writes{}
	c := newBuilder(t, w, interceptor.Funcs{
		Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			switch obj.GetName() {
			case "user-authz:dev:editor":
				return apierrors.NewInvalid(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding").GroupKind(), obj.GetName(), nil)
			case "user-authz:dev:editor:custom":
				return apierrors.NewServiceUnavailable("etcd leader changed")
			}
			return cl.Create(ctx, obj, opts...)
		},
	}, car("dev", desired.AccessLevelEditor)).Build()

	_, err := clusterReconciler(c).Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "dev"}})
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, reconcile.TerminalError(nil)) {
		t.Fatalf("err = %v: a retryable failure must keep the request in the backoff queue", err)
	}
	if !strings.Contains(err.Error(), "etcd leader changed") || !strings.Contains(err.Error(), "user-authz:dev:editor") {
		t.Errorf("both failures must be reported: %v", err)
	}
}

func TestRuleIndexValue(t *testing.T) {
	t.Parallel()
	if got := RuleIndexValue(helmBinding("user-authz:dev:editor", "x")); len(got) != 1 || got[0] != "dev" {
		t.Errorf("index of a module binding = %v", got)
	}
	rule := desired.FromClusterAuthorizationRule(car("dev", desired.AccessLevelUser))
	owned := desired.ClusterRoleBinding(mustBindings(t, rule)[0], desired.OwnerReference(rule))
	owned.Labels = nil
	if got := RuleIndexValue(owned); len(got) != 1 || got[0] != "dev" {
		t.Errorf("index of a binding owned by the rule but without labels = %v", got)
	}
	misnamed := owned.DeepCopy()
	misnamed.Name = "user-authz:other:user"
	if got := RuleIndexValue(misnamed); got != nil {
		t.Errorf("a binding owned by a rule but not named after it must not be indexed, got %v", got)
	}
	foreign := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "user-authz:dev:editor"}}
	if got := RuleIndexValue(foreign); got != nil {
		t.Errorf("index of a binding without module labels = %v", got)
	}
	other := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "d8:user-authz:kubeconfig", Labels: moduleLabels()}}
	if got := RuleIndexValue(other); got != nil {
		t.Errorf("index of a module object that is not a rule binding = %v", got)
	}
}

func mustBindings(t *testing.T, rule desired.Rule) []desired.Binding {
	t.Helper()
	bs, err := desired.Bindings(rule)
	if err != nil {
		t.Fatal(err)
	}
	return bs
}

func keys(m map[string]rbacv1.ClusterRoleBinding) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestReconcile_ReportsMetrics(t *testing.T) {
	t.Parallel()
	w := &writes{}
	m := metrics.New()
	c := newClient(t, w,
		car("dev", desired.AccessLevelEditor),
		helmBinding("user-authz:dev:editor", "user-authz:editor"),
		helmBinding("user-authz:dev:editor:custom-cluster-role:d8:user-authz:istio:editor", "d8:user-authz:istio:editor"),
	)
	r := NewCluster(c, c, events.NewFakeRecorder(10), m, logr.Discard())
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "dev"}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// converged: desired == actual, no drift, one rule known
	expected := `
# HELP d8_user_authz_authorization_rules Reconciled objects known to the controller, by kind (1 for the singleton dict and manage reconcilers).
# TYPE d8_user_authz_authorization_rules gauge
d8_user_authz_authorization_rules{kind="ClusterAuthorizationRule"} 1
# HELP d8_user_authz_bindings_actual Bindings of the reconciled objects of a kind after their last reconcile; equals desired once the controller converged.
# TYPE d8_user_authz_bindings_actual gauge
d8_user_authz_bindings_actual{kind="ClusterAuthorizationRule"} 2
# HELP d8_user_authz_bindings_desired Bindings the reconciled objects of a kind must have.
# TYPE d8_user_authz_bindings_desired gauge
d8_user_authz_bindings_desired{kind="ClusterAuthorizationRule"} 2
`
	if err := testutil.CollectAndCompare(m, strings.NewReader(expected),
		"d8_user_authz_authorization_rules", "d8_user_authz_bindings_desired", "d8_user_authz_bindings_actual"); err != nil {
		t.Fatal(err)
	}
	if n := testutil.CollectAndCount(m, "d8_user_authz_authorization_rule_invalid"); n != 0 {
		t.Errorf("invalid series = %d, want 0", n)
	}
	// one adoption update, one aggregated create, one legacy delete, one status patch
	if got := w.snapshot(); len(got) != 4 {
		t.Errorf("writes = %v", got)
	}
	if got := testutil.ToFloat64(m.ApplyTotal().WithLabelValues(metrics.KindClusterAuthorizationRule, metrics.OpDelete, metrics.ResultSuccess)); got != 1 {
		t.Errorf("deletes = %v, want 1", got)
	}

	// the rule disappears: its series go away
	if err := c.Delete(t.Context(), car("dev", desired.AccessLevelEditor)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "dev"}}); err != nil {
		t.Fatalf("Reconcile after delete: %v", err)
	}
	gone := `
# HELP d8_user_authz_authorization_rules Reconciled objects known to the controller, by kind (1 for the singleton dict and manage reconcilers).
# TYPE d8_user_authz_authorization_rules gauge
d8_user_authz_authorization_rules{kind="ClusterAuthorizationRule"} 0
`
	if err := testutil.CollectAndCompare(m, strings.NewReader(gone), "d8_user_authz_authorization_rules"); err != nil {
		t.Fatal(err)
	}
}

// Drift is what is left wrong after the writes: the binding that could not be created and the
// legacy binding kept in its place; the adoption update that went through is not drift.
func TestReconcile_ReportsDriftAndInvalidOnApplyError(t *testing.T) {
	t.Parallel()
	w := &writes{}
	m := metrics.New()
	adopted := helmBinding("user-authz:dev:editor", "user-authz:editor")
	legacy := helmBinding("user-authz:dev:editor:custom-cluster-role:d8:user-authz:istio:editor", "d8:user-authz:istio:editor")
	c := newBuilder(t, w, interceptor.Funcs{
		Create: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
			return apierrors.NewInvalid(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding").GroupKind(), obj.GetName(), nil)
		},
	}, car("dev", desired.AccessLevelEditor), adopted, legacy).Build()
	r := NewCluster(c, c, events.NewFakeRecorder(10), m, logr.Discard())
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "dev"}}); err == nil {
		t.Fatal("expected the invalid create to fail the reconcile")
	}

	expected := `
# HELP d8_user_authz_authorization_rule_invalid Rules whose bindings cannot be applied (1 per rule, at most MaxInvalidSeries rules), with the reason of the Ready=False condition.
# TYPE d8_user_authz_authorization_rule_invalid gauge
d8_user_authz_authorization_rule_invalid{kind="ClusterAuthorizationRule",name="dev",reason="ApplyError",rule_namespace=""} 1
# HELP d8_user_authz_bindings_drift Bindings still not in the desired state after the last reconcile, by reason (missing, extra, changed); above zero only while the controller fails to converge.
# TYPE d8_user_authz_bindings_drift gauge
d8_user_authz_bindings_drift{kind="ClusterAuthorizationRule",reason="changed"} 0
d8_user_authz_bindings_drift{kind="ClusterAuthorizationRule",reason="extra"} 1
d8_user_authz_bindings_drift{kind="ClusterAuthorizationRule",reason="missing"} 1
`
	if err := testutil.CollectAndCompare(m, strings.NewReader(expected), "d8_user_authz_authorization_rule_invalid", "d8_user_authz_bindings_drift"); err != nil {
		t.Fatal(err)
	}
	if w.count("update") != 1 || w.count("delete") != 0 {
		t.Errorf("writes = %v", w.snapshot())
	}
}
