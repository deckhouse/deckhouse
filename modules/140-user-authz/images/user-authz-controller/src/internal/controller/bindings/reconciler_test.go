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
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "user-authz-controller/api/v1"
	"user-authz-controller/api/v1alpha1"
	"user-authz-controller/internal/desired"
)

var testSubjects = []rbacv1.Subject{{Kind: "User", Name: "jane"}}

type writes struct {
	creates, updates, deletes atomic.Int32
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

func newClient(t *testing.T, w *writes, objs ...client.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithStatusSubresource(&v1.ClusterAuthorizationRule{}, &v1alpha1.AuthorizationRule{}).
		WithObjects(objs...).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				w.creates.Add(1)
				return c.Create(ctx, obj, opts...)
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				w.updates.Add(1)
				return c.Update(ctx, obj, opts...)
			},
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				w.deletes.Add(1)
				return c.Delete(ctx, obj, opts...)
			},
		}).
		Build()
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
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, UID: types.UID("uid-" + name), Generation: 1},
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

func reconcileCluster(t *testing.T, c client.Client, name string) reconcile.Result {
	t.Helper()
	r := NewCluster(c, events.NewFakeRecorder(10), logr.Discard())
	res, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	return res
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
	if w.creates.Load() != 3 || w.updates.Load() != 0 || w.deletes.Load() != 0 {
		t.Errorf("writes: creates=%d updates=%d deletes=%d", w.creates.Load(), w.updates.Load(), w.deletes.Load())
	}
}

func TestReconcile_IsIdempotent(t *testing.T) {
	t.Parallel()
	w := &writes{}
	c := newClient(t, w, car("dev", desired.AccessLevelUser))

	reconcileCluster(t, c, "dev")
	before := w.creates.Load() + w.updates.Load() + w.deletes.Load()

	reconcileCluster(t, c, "dev")
	after := w.creates.Load() + w.updates.Load() + w.deletes.Load()
	if after != before {
		t.Fatalf("second reconcile performed %d writes", after-before)
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
	if w.creates.Load() != 0 || w.updates.Load() != 2 || w.deletes.Load() != 0 {
		t.Errorf("writes: creates=%d updates=%d deletes=%d, want two in-place updates", w.creates.Load(), w.updates.Load(), w.deletes.Load())
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
		if len(name) > len("user-authz:dev:editor:custom-cluster-role:") && name[:len("user-authz:dev:editor:custom-cluster-role:")] == "user-authz:dev:editor:custom-cluster-role:" {
			t.Errorf("legacy per-role binding %s must be removed", name)
		}
	}
	if w.deletes.Load() != 2 {
		t.Errorf("deletes = %d, want 2", w.deletes.Load())
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
	if w.deletes.Load() != 0 {
		t.Errorf("deletes = %d, want 0", w.deletes.Load())
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
		t.Error("prefix match must stop at the rule name separator")
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

	r := NewNamespaced(c, events.NewFakeRecorder(10), logr.Discard())
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "dev", Namespace: "team"}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	got := &v1alpha1.AuthorizationRule{}
	if err := c.Get(t.Context(), types.NamespacedName{Name: "dev", Namespace: "team"}, got); err != nil {
		t.Fatal(err)
	}
	cond := apimeta.FindStatusCondition(got.Status.Conditions, ConditionReady)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != ReasonInvalidSpec {
		t.Fatalf("Ready condition = %+v", cond)
	}
	if w.creates.Load()+w.updates.Load()+w.deletes.Load() != 0 {
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

	r := NewNamespaced(c, events.NewFakeRecorder(10), logr.Discard())
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "dev", Namespace: "team"}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

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

func keys(m map[string]rbacv1.ClusterRoleBinding) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
