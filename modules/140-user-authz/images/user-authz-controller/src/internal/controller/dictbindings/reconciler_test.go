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
	"testing"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: copyLabels(DictLabels)},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: DictRoleName},
		Subjects:   []rbacv1.Subject{subject},
	}
}

func reconcileWith(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	c := fake.NewClientBuilder().WithScheme(clientgoscheme.Scheme).WithObjects(objs...).Build()
	r := New(c, logr.Discard())
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKey{Name: RequestName}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
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
	r := New(c, logr.Discard())
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
		"user:averyveryveryveryveryveryveryveryveryveryverylongn": user("averyveryveryveryveryveryveryveryveryveryverylongnameXXXXXXXXXX"),
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
	if a.Annotations[SubjectAnnotat] != "user:jane" || a.Labels[labelDict] != "true" {
		t.Errorf("metadata = %+v", a.ObjectMeta)
	}
}
