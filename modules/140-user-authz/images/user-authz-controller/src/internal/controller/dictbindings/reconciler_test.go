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

package dictbindings

import (
	"maps"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus/testutil"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authz-controller/internal/metrics"
)

func roleBinding(ns, name, role string, labels map[string]string, subjects ...rbacv1.Subject) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labels},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: role},
		Subjects:   subjects,
	}
}

func user(name string) rbacv1.Subject  { return rbacv1.Subject{Kind: "User", Name: name} }
func group(name string) rbacv1.Subject { return rbacv1.Subject{Kind: "Group", Name: name} }
func sa(ns, name string) rbacv1.Subject {
	return rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: name, Namespace: ns}
}

// legacyDict mimics a dict binding created by the hook with a generated name.
func legacyDict(name string, subject rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: maps.Clone(DictLabels)},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: DictRoleName},
		Subjects:   []rbacv1.Subject{subject},
	}
}

// newClient prepares a fake client the way the manager does: with the dict source index.
func newClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&rbacv1.RoleBinding{}, SourceIndexField, SourceIndexValue).
		WithIndex(&rbacv1.ClusterRoleBinding{}, OwnedIndexField, OwnedIndexValue).
		WithObjects(objs...).
		Build()
}

func reconcileOnce(t *testing.T, c client.Client) {
	t.Helper()
	r := New(c, metrics.New(), logr.Discard())
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKey{Name: RequestName}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func reconcileWith(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	c := newClient(t, objs...)
	reconcileOnce(t, c)
	return c
}

func dictSubjects(t *testing.T, c client.Client) map[string]rbacv1.Subject {
	t.Helper()
	list := &rbacv1.ClusterRoleBindingList{}
	if err := c.List(t.Context(), list, client.MatchingLabels(DictLabels)); err != nil {
		t.Fatal(err)
	}
	out := map[string]rbacv1.Subject{}
	for _, crb := range list.Items {
		if crb.RoleRef.Name != DictRoleName {
			t.Errorf("%s: roleRef = %v", crb.Name, crb.RoleRef)
		}
		if len(crb.Subjects) != 1 {
			t.Fatalf("%s: subjects = %v", crb.Name, crb.Subjects)
		}
		out[SubjectKey(crb.Subjects[0])] = crb.Subjects[0]
	}
	return out
}

func TestReconcile_GrantsDictToSubjectsOfUseBindings(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		// user-created binding of the experimental model
		roleBinding("team", "devs", "d8:use:role:user", nil, user("jane"), group("devs")),
		// module-created binding of a namespaced rule (current model)
		roleBinding("team", "user-authz:rule:editor", "user-authz:editor", map[string]string{"heritage": "deckhouse", "module": "user-authz"}, sa("", "deployer")),
		// deckhouse-created binding to a use role: excluded (only user bindings count there)
		roleBinding("team", "d8:use:admin:binding:x", "d8:use:role:admin", map[string]string{"heritage": "deckhouse"}, user("ignored-1")),
		// unrelated binding
		roleBinding("team", "other", "view", nil, user("ignored-2")),
	)

	got := dictSubjects(t, c)
	if len(got) != 3 {
		t.Fatalf("dict subjects = %v, want jane, devs and the service account", got)
	}
	if _, ok := got["user:jane"]; !ok {
		t.Error("jane must hold d8:use:dict")
	}
	if _, ok := got["group:devs"]; !ok {
		t.Error("group devs must hold d8:use:dict")
	}
	saSubject, ok := got["sa:team:deployer"]
	if !ok {
		t.Fatalf("service account must be keyed with the namespace of its RoleBinding; got %v", got)
	}
	if saSubject.Namespace != "team" {
		t.Errorf("service account namespace = %q, want the RoleBinding namespace", saSubject.Namespace)
	}
}

func TestReconcile_RemovesDictOfSubjectsWithoutUseRole(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t,
		roleBinding("team", "devs", "d8:use:role:user", nil, user("jane")),
		legacyDict("d8:dict:abcde", user("jane")),
		legacyDict("d8:dict:fghij", user("gone")),
	)

	got := dictSubjects(t, c)
	if len(got) != 1 {
		t.Fatalf("dict subjects = %v, want only jane", got)
	}
	if _, ok := got["user:jane"]; !ok {
		t.Error("jane must keep d8:use:dict")
	}

	// the legacy object with a generated name must have been kept, not replaced
	kept := &rbacv1.ClusterRoleBinding{}
	if err := c.Get(t.Context(), client.ObjectKey{Name: "d8:dict:abcde"}, kept); err != nil {
		t.Errorf("legacy dict binding of a still-valid subject must survive: %v", err)
	}
}

func TestReconcile_IsIdempotent(t *testing.T) {
	t.Parallel()
	c := reconcileWith(t, roleBinding("team", "devs", "d8:use:role:user", nil, user("jane"), sa("", "bot")))

	first := dictSubjects(t, c)
	r := New(c, metrics.New(), logr.Discard())
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKey{Name: RequestName}}); err != nil {
		t.Fatal(err)
	}
	second := dictSubjects(t, c)
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("subjects before=%v after=%v", first, second)
	}
	list := &rbacv1.ClusterRoleBindingList{}
	if err := c.List(t.Context(), list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("second reconcile must not duplicate bindings, have %d", len(list.Items))
	}
}

func TestSubjectKey(t *testing.T) {
	t.Parallel()
	cases := map[string]rbacv1.Subject{
		"user:jane":   user("Jane"),
		"group:devs":  group("Devs"),
		"sa:team:bot": sa("team", "bot"),
		"user:averyveryveryveryveryveryveryveryveryveryverylongnamexxxxxxxxxx": user("averyveryveryveryveryveryveryveryveryveryverylongnameXXXXXXXXXX"),
	}
	for want, subject := range cases {
		if got := SubjectKey(subject); got != want {
			t.Errorf("SubjectKey(%+v) = %q, want %q", subject, got, want)
		}
	}
}

func TestBindingNameIsStableAndPrefixed(t *testing.T) {
	t.Parallel()
	a := Binding("user:jane", user("jane"))
	b := Binding("user:jane", user("jane"))
	if a.Name != b.Name || len(a.Name) <= len(NamePrefix) || a.Name[:len(NamePrefix)] != NamePrefix {
		t.Fatalf("names = %q / %q", a.Name, b.Name)
	}
	if a.Annotations[SubjectAnnotation] != "user:jane" || a.Labels[labelDict] != "true" {
		t.Errorf("metadata = %+v", a.ObjectMeta)
	}
}

// Two subjects that only differ after the 55th character are two subjects (the hook this
// reconciler replaces collapsed them).
func TestReconcile_LongSubjectsStayDistinct(t *testing.T) {
	t.Parallel()
	prefix := strings.Repeat("a", 60)
	c := reconcileWith(t,
		roleBinding("team", "devs", "d8:use:role:user", nil, user(prefix+"-one"), user(prefix+"-two")),
	)

	got := dictSubjects(t, c)
	if len(got) != 2 {
		t.Fatalf("dict subjects = %d, want 2 distinct bindings", len(got))
	}
}

func TestReconcile_RepairsBrokenDictBindings(t *testing.T) {
	t.Parallel()
	wrongRole := legacyDict("d8:dict:wrong-role", user("jane"))
	wrongRole.RoleRef.Name = "cluster-admin"
	noSubject := legacyDict("d8:dict:no-subject", user("jane"))
	noSubject.Subjects = nil
	c := reconcileWith(t,
		roleBinding("team", "devs", "d8:use:role:user", nil, user("jane"), user("bob")),
		wrongRole,
		noSubject,
		legacyDict("d8:dict:bob", user("bob")),
		legacyDict("d8:dict:bob-duplicate", user("bob")),
	)

	got := dictSubjects(t, c)
	if len(got) != 2 {
		t.Fatalf("dict subjects = %v, want jane and bob", got)
	}
	for _, name := range []string{"d8:dict:wrong-role", "d8:dict:no-subject", "d8:dict:bob-duplicate"} {
		if err := c.Get(t.Context(), client.ObjectKey{Name: name}, &rbacv1.ClusterRoleBinding{}); err == nil {
			t.Errorf("%s must be removed", name)
		}
	}
	if err := c.Get(t.Context(), client.ObjectKey{Name: "d8:dict:bob"}, &rbacv1.ClusterRoleBinding{}); err != nil {
		t.Errorf("the first binding of a duplicated subject must be kept: %v", err)
	}
	list := &rbacv1.ClusterRoleBindingList{}
	if err := c.List(t.Context(), list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 {
		t.Errorf("bindings = %d, want exactly one per subject", len(list.Items))
	}
}

func TestSourceIndexValue(t *testing.T) {
	t.Parallel()
	if got := SourceIndexValue(roleBinding("team", "devs", "d8:use:role:user", nil, user("jane"))); len(got) != 1 {
		t.Errorf("user binding to a use role must be indexed, got %v", got)
	}
	if got := SourceIndexValue(roleBinding("team", "x", "view", nil, user("jane"))); got != nil {
		t.Errorf("unrelated binding must not be indexed, got %v", got)
	}
	if got := SourceIndexValue(&rbacv1.ClusterRoleBinding{}); got != nil {
		t.Errorf("a ClusterRoleBinding must not be indexed, got %v", got)
	}
}

func TestOwnedIndexValue(t *testing.T) {
	t.Parallel()
	if got := OwnedIndexValue(legacyDict("d8:dict:x", user("jane"))); len(got) != 1 {
		t.Errorf("a dict binding must be indexed, got %v", got)
	}
	plain := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "user-authz:dev:user", Labels: map[string]string{"heritage": "deckhouse"}}}
	if got := OwnedIndexValue(plain); got != nil {
		t.Errorf("a binding without the dict labels must not be indexed, got %v", got)
	}
}

func TestReconcile_ReportsMetrics(t *testing.T) {
	t.Parallel()
	m := metrics.New()
	c := newClient(t,
		roleBinding("team", "devs", "d8:use:role:user", nil, user("jane"), user("bob")),
		legacyDict("d8:dict:jane", user("jane")),
		legacyDict("d8:dict:gone", user("gone")),
	)
	if _, err := New(c, m, logr.Discard()).Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKey{Name: RequestName}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	expected := `
# HELP d8_user_authz_authorization_rules Reconciled objects known to the controller, by kind (1 for the singleton dict and manage reconcilers).
# TYPE d8_user_authz_authorization_rules gauge
d8_user_authz_authorization_rules{kind="dict"} 1
# HELP d8_user_authz_bindings_actual Bindings of the reconciled objects of a kind after their last reconcile; equals desired once the controller converged.
# TYPE d8_user_authz_bindings_actual gauge
d8_user_authz_bindings_actual{kind="dict"} 2
# HELP d8_user_authz_bindings_desired Bindings the reconciled objects of a kind must have.
# TYPE d8_user_authz_bindings_desired gauge
d8_user_authz_bindings_desired{kind="dict"} 2
# HELP d8_user_authz_bindings_drift Bindings still not in the desired state after the last reconcile, by reason (missing, extra, changed); above zero only while the controller fails to converge.
# TYPE d8_user_authz_bindings_drift gauge
d8_user_authz_bindings_drift{kind="dict",reason="changed"} 0
d8_user_authz_bindings_drift{kind="dict",reason="extra"} 0
d8_user_authz_bindings_drift{kind="dict",reason="missing"} 0
`
	if err := testutil.CollectAndCompare(m, strings.NewReader(expected),
		"d8_user_authz_authorization_rules", "d8_user_authz_bindings_desired", "d8_user_authz_bindings_actual", "d8_user_authz_bindings_drift"); err != nil {
		t.Fatal(err)
	}
	// one binding created (bob), one deleted (gone)
	if got := testutil.ToFloat64(m.ApplyTotal().WithLabelValues(metrics.KindDict, metrics.OpCreate, metrics.ResultSuccess)); got != 1 {
		t.Errorf("creates = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.ApplyTotal().WithLabelValues(metrics.KindDict, metrics.OpDelete, metrics.ResultSuccess)); got != 1 {
		t.Errorf("deletes = %v, want 1", got)
	}
}
