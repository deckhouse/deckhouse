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

package projectrolebinding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/rolebinding"
)

func newReconciler(t *testing.T, objs ...client.Object) (*Reconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme, rbacv1.AddToScheme, v1alpha3.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&v1alpha3.ProjectRoleBinding{}).
		WithIndex(&v1alpha3.ProjectRoleBinding{}, subjectNamespaceIndex, subjectNamespaces).
		Build()
	return &Reconciler{Client: c, Recorder: record.NewFakeRecorder(20)}, c
}

// recordedEvents drains the events the reconciler has sent so far.
func recordedEvents(t *testing.T, r *Reconciler) []string {
	t.Helper()
	recorder, ok := r.Recorder.(*record.FakeRecorder)
	require.True(t, ok)
	var got []string
	for {
		select {
		case e := <-recorder.Events:
			got = append(got, e)
		default:
			return got
		}
	}
}

func namespace(name, project string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{v1alpha3.ResourceLabelProject: project}}}
}

func serviceAccount(namespace, name string) rbacv1.Subject {
	return rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: name, Namespace: namespace}
}

func condition(t *testing.T, prb *v1alpha3.ProjectRoleBinding, condType string) v1alpha3.Condition {
	t.Helper()
	for _, cond := range prb.Status.Conditions {
		if cond.Type == condType {
			return cond
		}
	}
	require.Failf(t, "no condition", "the %s condition is not set", condType)
	return v1alpha3.Condition{}
}

func project(name string, namespaces ...string) *v1alpha3.Project {
	p := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name}}
	for _, ns := range namespaces {
		kind := v1alpha3.NamespaceKindAdditional
		if ns == name {
			kind = v1alpha3.NamespaceKindMain
		}
		p.Status.Namespaces = append(p.Status.Namespaces, v1alpha3.NamespaceStatus{Name: ns, Kind: kind})
	}
	return p
}

func prb(name, namespace, role string) *v1alpha3.ProjectRoleBinding {
	return &v1alpha3.ProjectRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha3.ProjectRoleBindingSpec{
			Subjects: []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: "User", Name: "alice"}},
			RoleRef:  v1alpha3.RoleRef{Kind: "ClusterRole", Name: role},
		},
	}
}

func TestReconcile_FansOutToAllNamespaces(t *testing.T) {
	binding := prb("viewers", "proj", "d8:project:viewer")
	r, c := newReconciler(t, binding, project("proj", "proj", "proj-extra"))

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "proj", Name: "viewers"}})
	assert.NoError(t, err)

	for _, ns := range []string{"proj", "proj-extra"} {
		rb := &rbacv1.RoleBinding{}
		err := c.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: rolebinding.PRBServiceName("viewers")}, rb)
		assert.NoErrorf(t, err, "RoleBinding must exist in namespace %s", ns)
		assert.Equal(t, "d8:project:viewer", rb.RoleRef.Name)
		assert.Equal(t, "viewers", rb.Labels[v1alpha3.ResourceLabelOwnedByPRB])
		assert.Len(t, rb.Subjects, 1)
		assert.Equal(t, "alice", rb.Subjects[0].Name)
	}

	// the finalizer must have been added
	got := &v1alpha3.ProjectRoleBinding{}
	assert.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: "viewers"}, got))
	assert.Contains(t, got.Finalizers, v1alpha3.ProjectRoleBindingFinalizer)
}

// TestReconcile_FansOutToProjectNamespaceAdditional verifies the ProjectNamespace integration: when
// an additional namespace (created by a ProjectNamespace and reflected in Project.status.namespaces)
// appears, the existing PRB extends its service RoleBinding into it; when it is removed, the stale
// RoleBinding is pruned.
func TestReconcile_FansOutToProjectNamespaceAdditional(t *testing.T) {
	binding := prb("viewers", "proj", "d8:project:viewer")
	// the ProjectNamespace controller added "proj-backend" to the project status.
	r, c := newReconciler(t, binding, project("proj", "proj", "proj-backend"))

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "proj", Name: "viewers"}})
	assert.NoError(t, err)

	for _, ns := range []string{"proj", "proj-backend"} {
		assert.NoErrorf(t, c.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: rolebinding.PRBServiceName("viewers")}, &rbacv1.RoleBinding{}),
			"RoleBinding must be fanned out into %s", ns)
	}

	// the ProjectNamespace was deleted: the additional namespace leaves the project status.
	updated := &v1alpha3.ProjectRoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: "viewers"}, updated))
	r2, c2 := newReconciler(t, updated, project("proj", "proj"),
		&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{
			Name:      rolebinding.PRBServiceName("viewers"),
			Namespace: "proj-backend",
			Labels:    map[string]string{v1alpha3.ResourceLabelOwnedByPRB: "viewers", v1alpha3.ResourceLabelProject: "proj"},
		}})
	_, err = r2.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "proj", Name: "viewers"}})
	assert.NoError(t, err)
	err = c2.Get(context.Background(), client.ObjectKey{Namespace: "proj-backend", Name: rolebinding.PRBServiceName("viewers")}, &rbacv1.RoleBinding{})
	assert.Error(t, err, "the RoleBinding must be pruned from the removed additional namespace")
}

func TestReconcile_PrunesStaleRoleBindings(t *testing.T) {
	// a stale service RoleBinding exists in a namespace no longer part of the project; it carries
	// the project label, just like every RoleBinding the controller fans out.
	stale := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rolebinding.PRBServiceName("viewers"),
			Namespace: "proj-gone",
			Labels: map[string]string{
				v1alpha3.ResourceLabelOwnedByPRB: "viewers",
				v1alpha3.ResourceLabelProject:    "proj",
			},
		},
	}
	binding := prb("viewers", "proj", "d8:project:viewer")
	r, c := newReconciler(t, binding, project("proj", "proj"), stale)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "proj", Name: "viewers"}})
	assert.NoError(t, err)

	// the stale binding is removed
	err = c.Get(context.Background(), client.ObjectKey{Namespace: "proj-gone", Name: rolebinding.PRBServiceName("viewers")}, &rbacv1.RoleBinding{})
	assert.Error(t, err)

	// the live one exists
	assert.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: rolebinding.PRBServiceName("viewers")}, &rbacv1.RoleBinding{}))
}

// TestReconcile_PruneIsProjectScoped guards against cross-project deletion: two projects each have a
// PRB named "viewers"; reconciling one must not delete the other project's service RoleBinding.
func TestReconcile_PruneIsProjectScoped(t *testing.T) {
	// service RoleBinding belonging to project "other" (same PRB name, different project)
	otherRB := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rolebinding.PRBServiceName("viewers"),
			Namespace: "other",
			Labels: map[string]string{
				v1alpha3.ResourceLabelOwnedByPRB: "viewers",
				v1alpha3.ResourceLabelProject:    "other",
			},
		},
	}
	binding := prb("viewers", "proj", "d8:project:viewer")
	r, c := newReconciler(t, binding, project("proj", "proj"), otherRB)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "proj", Name: "viewers"}})
	assert.NoError(t, err)

	// the other project's RoleBinding must survive
	assert.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "other", Name: rolebinding.PRBServiceName("viewers")}, &rbacv1.RoleBinding{}),
		"reconciling project 'proj' must not prune project 'other' bindings")
}

// TestReconcile_RecreatesOnRoleRefChange verifies the immutable roleRef is handled by delete+recreate.
func TestReconcile_RecreatesOnRoleRefChange(t *testing.T) {
	// an existing service RoleBinding pointing at the old role
	old := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rolebinding.PRBServiceName("viewers"),
			Namespace: "proj",
			Labels: map[string]string{
				v1alpha3.ResourceLabelOwnedByPRB: "viewers",
				v1alpha3.ResourceLabelProject:    "proj",
			},
		},
		RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "d8:project:viewer"},
	}
	binding := prb("viewers", "proj", "d8:project:admin")
	r, c := newReconciler(t, binding, project("proj", "proj"), old)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "proj", Name: "viewers"}})
	assert.NoError(t, err)

	rb := &rbacv1.RoleBinding{}
	assert.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: rolebinding.PRBServiceName("viewers")}, rb))
	assert.Equal(t, "d8:project:admin", rb.RoleRef.Name, "roleRef must be updated to the new role")
}

// TestReconcile_ForbiddenRoleIsNotFannedOut verifies defense-in-depth: a disallowed role is cleaned
// up and never propagated, even if it slipped past the webhook.
func TestReconcile_ForbiddenRoleIsNotFannedOut(t *testing.T) {
	binding := prb("escalation", "proj", "cluster-admin")
	r, c := newReconciler(t, binding, project("proj", "proj"))

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "proj", Name: "escalation"}})
	assert.NoError(t, err)

	err = c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: rolebinding.PRBServiceName("escalation")}, &rbacv1.RoleBinding{})
	assert.Error(t, err, "a forbidden role must not be fanned out")
}

// TestReconcile_StatusUnchangedNoWrite is the unit-level guard for the self-triggered reconcile
// hot-loop fix: a second reconcile that changes nothing must not rewrite status (which the fake
// client would surface as a bumped resourceVersion). The full loop elimination still needs envtest.
// A left-out subject is part of it, as the same set gives the same condition and no second event.
func TestReconcile_StatusUnchangedNoWrite(t *testing.T) {
	binding := prb("viewers", "proj", "d8:project:viewer")
	binding.Spec.Subjects = append(binding.Spec.Subjects, serviceAccount("proj-other", "deployer"))
	r, c := newReconciler(t, binding, project("proj", "proj", "proj-extra"), namespace("proj-other", "proj-other"))

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "proj", Name: "viewers"}})
	require.NoError(t, err)
	first := &v1alpha3.ProjectRoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: "viewers"}, first))
	assert.Len(t, recordedEvents(t, r), 1)

	_, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "proj", Name: "viewers"}})
	require.NoError(t, err)
	second := &v1alpha3.ProjectRoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: "viewers"}, second))

	assert.Equal(t, first.ResourceVersion, second.ResourceVersion,
		"an unchanged reconcile must not rewrite the status and re-enqueue the object")
	assert.Empty(t, recordedEvents(t, r), "an unchanged set of left-out subjects must not be reported again")
}

// reconcileBinding runs one reconcile of the proj/ci binding and returns it as stored afterwards.
func reconcileBinding(t *testing.T, r *Reconciler, c client.Client) *v1alpha3.ProjectRoleBinding {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "proj", Name: "ci"}})
	require.NoError(t, err)
	got := &v1alpha3.ProjectRoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: "ci"}, got))
	return got
}

// fannedOutSubjects returns the subjects of the service RoleBinding of proj/ci in the namespace.
func fannedOutSubjects(t *testing.T, c client.Client, ns string) []rbacv1.Subject {
	t.Helper()
	rb := &rbacv1.RoleBinding{}
	require.NoErrorf(t, c.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: rolebinding.PRBServiceName("ci")}, rb),
		"the RoleBinding must exist in namespace %s", ns)
	return rb.Subjects
}

// The ServiceAccounts of the main namespace and of a namespace labelled with the project get the rights.
func TestReconcile_BindsServiceAccountsOfTheProject(t *testing.T) {
	binding := prb("ci", "proj", "d8:project:admin")
	binding.Spec.Subjects = append(binding.Spec.Subjects, serviceAccount("proj", "runner"), serviceAccount("proj-queue", "worker"))
	r, c := newReconciler(t, binding, project("proj", "proj", "proj-queue"), namespace("proj", "proj"), namespace("proj-queue", "proj"))

	got := reconcileBinding(t, r, c)

	for _, ns := range []string{"proj", "proj-queue"} {
		assert.Equal(t, binding.Spec.Subjects, fannedOutSubjects(t, c, ns))
	}
	bound := condition(t, got, v1alpha3.ProjectRoleBindingConditionSubjectsBound)
	assert.Equal(t, corev1.ConditionTrue, bound.Status)
	assert.Empty(t, bound.Message)
	assert.Empty(t, recordedEvents(t, r))
}

// The regression of a stored subject after its namespace left the project. proj-extra is now a
// separate project, and whoever creates the deployer ServiceAccount there must not get the rights of
// proj. Every form that names the ServiceAccounts of proj-extra is left out, the other subjects keep
// the rights, and the spec stays as it is.
func TestReconcile_LeavesOutServiceAccountsOfAnotherProject(t *testing.T) {
	alice := rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: "alice"}
	binding := prb("ci", "proj", "d8:project:admin")
	binding.Spec.Subjects = []rbacv1.Subject{
		alice,
		serviceAccount("proj-extra", "deployer"),
		{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: "system:serviceaccount:proj-extra:ci"},
		{APIGroup: rbacv1.GroupName, Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:proj-extra"},
		serviceAccount("proj-queue", "worker"),
	}
	r, c := newReconciler(t, binding,
		project("proj", "proj", "proj-queue"),
		namespace("proj", "proj"),
		namespace("proj-queue", "proj"),
		namespace("proj-extra", "proj-extra"),
	)

	got := reconcileBinding(t, r, c)

	for _, ns := range []string{"proj", "proj-queue"} {
		assert.Equal(t, []rbacv1.Subject{alice, serviceAccount("proj-queue", "worker")}, fannedOutSubjects(t, c, ns))
	}
	assert.Equal(t, binding.Spec.Subjects, got.Spec.Subjects, "the spec must not change")

	assert.Equal(t, corev1.ConditionTrue, condition(t, got, v1alpha3.ProjectRoleBindingConditionReady).Status,
		"the binding still works for the other subjects")
	bound := condition(t, got, v1alpha3.ProjectRoleBindingConditionSubjectsBound)
	assert.Equal(t, corev1.ConditionFalse, bound.Status)
	const expected = "The subjects are left out of the RoleBindings because their ServiceAccounts are not in a namespace of the project: " +
		"Group system:serviceaccounts:proj-extra, ServiceAccount proj-extra/deployer, User system:serviceaccount:proj-extra:ci"
	assert.Equal(t, expected, bound.Message)
	assert.Equal(t, []string{"Warning SubjectsLeftOut " + expected}, recordedEvents(t, r))
}

// A ServiceAccount of a namespace that is gone is left out, and it gets the rights back as soon as
// the namespace returns to the project, when a ProjectNamespace of the project is created again.
func TestReconcile_RestoresAServiceAccountWhenItsNamespaceReturns(t *testing.T) {
	binding := prb("ci", "proj", "d8:project:admin")
	binding.Spec.Subjects = []rbacv1.Subject{serviceAccount("proj-extra", "deployer")}
	r, c := newReconciler(t, binding, project("proj", "proj"), namespace("proj", "proj"))

	got := reconcileBinding(t, r, c)
	assert.Empty(t, fannedOutSubjects(t, c, "proj"))
	assert.Equal(t, corev1.ConditionFalse, condition(t, got, v1alpha3.ProjectRoleBindingConditionSubjectsBound).Status)
	assert.Len(t, recordedEvents(t, r), 1)

	require.NoError(t, c.Create(context.Background(), namespace("proj-extra", "proj")))
	got = reconcileBinding(t, r, c)
	assert.Equal(t, binding.Spec.Subjects, fannedOutSubjects(t, c, "proj"))
	bound := condition(t, got, v1alpha3.ProjectRoleBindingConditionSubjectsBound)
	assert.Equal(t, corev1.ConditionTrue, bound.Status)
	assert.Empty(t, bound.Message)
	assert.Empty(t, recordedEvents(t, r), "nothing is left out any more")
}

// A namespace being deleted is on its way out of the project, and its name is free once it is gone.
func TestReconcile_LeavesOutServiceAccountsOfADeletingNamespace(t *testing.T) {
	deleting := namespace("proj-extra", "proj")
	deleting.DeletionTimestamp = &metav1.Time{Time: metav1.Now().Time}
	deleting.Finalizers = []string{"test"}
	binding := prb("ci", "proj", "d8:project:admin")
	binding.Spec.Subjects = []rbacv1.Subject{serviceAccount("proj-extra", "deployer")}
	r, c := newReconciler(t, binding, project("proj", "proj", "proj-extra"), namespace("proj", "proj"), deleting)

	got := reconcileBinding(t, r, c)

	assert.Empty(t, fannedOutSubjects(t, c, "proj"))
	bound := condition(t, got, v1alpha3.ProjectRoleBindingConditionSubjectsBound)
	assert.Equal(t, corev1.ConditionFalse, bound.Status)
	assert.Contains(t, bound.Message, "ServiceAccount proj-extra/deployer")
}

// With every subject left out the RoleBindings stay in every namespace of the project, without
// subjects. They grant nothing, and the subjects come back into them in place.
func TestReconcile_AllSubjectsLeftOut(t *testing.T) {
	binding := prb("ci", "proj", "d8:project:admin")
	binding.Spec.Subjects = []rbacv1.Subject{serviceAccount("proj-other", "deployer")}
	r, c := newReconciler(t, binding, project("proj", "proj", "proj-queue"), namespace("proj-other", "proj-other"))

	got := reconcileBinding(t, r, c)

	for _, ns := range []string{"proj", "proj-queue"} {
		assert.Empty(t, fannedOutSubjects(t, c, ns))
	}
	assert.Equal(t, corev1.ConditionTrue, condition(t, got, v1alpha3.ProjectRoleBindingConditionReady).Status)
	assert.Equal(t, corev1.ConditionFalse, condition(t, got, v1alpha3.ProjectRoleBindingConditionSubjectsBound).Status)
}

// The administrators binding the controller writes from spec.administrators carries the
// administrators of the project, whatever namespace their ServiceAccounts are in. The upgrade lifts
// such administrators of the previous release into spec.administrators, and an AuthorizationRule gave
// them the rights until then. A subject the project no longer names as an administrator is judged like
// the subjects of every other binding.
func TestReconcile_CarriesTheAdministratorsOfTheProject(t *testing.T) {
	runner := rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: "system:serviceaccount:ci:runner"}
	builders := rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:ci"}
	removed := rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: "system:serviceaccount:ci:deployer"}
	binding := prb(v1alpha3.ProjectAdministratorsBinding, "proj", v1alpha3.ProjectAdministratorsRoleName)
	binding.Labels = map[string]string{v1alpha3.ResourceLabelManagedBy: v1alpha3.ManagedByController}
	binding.Spec.Subjects = []rbacv1.Subject{runner, builders, removed}
	p := project("proj", "proj", "proj-queue")
	p.Spec.Administrators = []v1alpha3.Administrator{{Kind: runner.Kind, Name: runner.Name}, {Kind: builders.Kind, Name: builders.Name}}
	r, c := newReconciler(t, binding, p, namespace("proj", "proj"), namespace("proj-queue", "proj"), namespace("ci", "ci"))

	key := types.NamespacedName{Namespace: "proj", Name: v1alpha3.ProjectAdministratorsBinding}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
	require.NoError(t, err)

	for _, ns := range []string{"proj", "proj-queue"} {
		rb := &rbacv1.RoleBinding{}
		require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: rolebinding.PRBServiceName(key.Name)}, rb))
		assert.Equal(t, []rbacv1.Subject{runner, builders}, rb.Subjects, "the RoleBinding in %s", ns)
	}
	got := &v1alpha3.ProjectRoleBinding{}
	require.NoError(t, c.Get(context.Background(), key, got))
	const expected = "The subjects are left out of the RoleBindings because their ServiceAccounts are not in a namespace of the project: " +
		"User system:serviceaccount:ci:deployer"
	assert.Equal(t, expected, condition(t, got, v1alpha3.ProjectRoleBindingConditionSubjectsBound).Message)
	assert.Equal(t, []string{"Warning SubjectsLeftOut " + expected}, recordedEvents(t, r))
}

// Only the administrators binding the controller writes carries the administrators of the project
// whatever namespace they name. It has the managed-by label, which the webhook refuses to everyone but
// the controller and Deckhouse. Another binding that names an administrator holds it to the project
// like every other binding, and so does a binding named d8-administrators without the label, until
// the label is set.
func TestReconcile_CarriesTheAdministratorsOnlyInTheLabelledBinding(t *testing.T) {
	alice := rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: "alice"}
	runner := rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: "system:serviceaccount:ci:runner"}
	naming := func(name string) *v1alpha3.ProjectRoleBinding {
		b := prb(name, "proj", v1alpha3.ProjectAdministratorsRoleName)
		b.Spec.Subjects = []rbacv1.Subject{alice, runner}
		return b
	}
	p := project("proj", "proj", "proj-queue")
	p.Spec.Administrators = []v1alpha3.Administrator{{Kind: alice.Kind, Name: alice.Name}, {Kind: runner.Kind, Name: runner.Name}}
	r, c := newReconciler(t, naming("ci-admins"), naming(v1alpha3.ProjectAdministratorsBinding), p,
		namespace("proj", "proj"), namespace("proj-queue", "proj"), namespace("ci", "ci"))

	reconcileNamed := func(name string) *v1alpha3.ProjectRoleBinding {
		t.Helper()
		key := types.NamespacedName{Namespace: "proj", Name: name}
		_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
		require.NoError(t, err)
		got := &v1alpha3.ProjectRoleBinding{}
		require.NoError(t, c.Get(context.Background(), key, got))
		return got
	}
	fannedOut := func(name, ns string) []rbacv1.Subject {
		t.Helper()
		rb := &rbacv1.RoleBinding{}
		require.NoErrorf(t, c.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: rolebinding.PRBServiceName(name)}, rb),
			"the RoleBinding of %s must exist in namespace %s", name, ns)
		return rb.Subjects
	}

	const leftOut = "The subjects are left out of the RoleBindings because their ServiceAccounts are not in a namespace of the project: " +
		"User system:serviceaccount:ci:runner"
	for _, name := range []string{"ci-admins", v1alpha3.ProjectAdministratorsBinding} {
		got := reconcileNamed(name)
		for _, ns := range []string{"proj", "proj-queue"} {
			assert.Equal(t, []rbacv1.Subject{alice}, fannedOut(name, ns), "the RoleBinding of %s in %s", name, ns)
		}
		bound := condition(t, got, v1alpha3.ProjectRoleBindingConditionSubjectsBound)
		assert.Equal(t, corev1.ConditionFalse, bound.Status, name)
		assert.Equal(t, leftOut, bound.Message, name)
		assert.Equal(t, []string{"Warning SubjectsLeftOut " + leftOut}, recordedEvents(t, r), name)
	}

	labelled := &v1alpha3.ProjectRoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: v1alpha3.ProjectAdministratorsBinding}, labelled))
	labelled.Labels = map[string]string{v1alpha3.ResourceLabelManagedBy: v1alpha3.ManagedByController}
	require.NoError(t, c.Update(context.Background(), labelled))

	got := reconcileNamed(v1alpha3.ProjectAdministratorsBinding)
	for _, ns := range []string{"proj", "proj-queue"} {
		assert.Equal(t, []rbacv1.Subject{alice, runner}, fannedOut(v1alpha3.ProjectAdministratorsBinding, ns), "the RoleBinding in %s", ns)
	}
	bound := condition(t, got, v1alpha3.ProjectRoleBindingConditionSubjectsBound)
	assert.Equal(t, corev1.ConditionTrue, bound.Status)
	assert.Empty(t, bound.Message)
	assert.Empty(t, recordedEvents(t, r))
}

// A namespace event reaches the bindings whose subjects name the ServiceAccounts of the namespace, in
// any form, and no other binding. The own namespace of a binding is not indexed, as its
// ServiceAccounts always get the rights.
func TestRequestsForNamespace(t *testing.T) {
	withSubjects := func(name, ns string, subjects ...rbacv1.Subject) *v1alpha3.ProjectRoleBinding {
		b := prb(name, ns, "d8:project:viewer")
		b.Spec.Subjects = subjects
		return b
	}
	r, _ := newReconciler(t,
		withSubjects("account", "proj", serviceAccount("proj-extra", "deployer")),
		withSubjects("group", "proj", rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:proj-extra"}),
		withSubjects("user", "other", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "system:serviceaccount:proj-extra:ci"}),
		withSubjects("people", "proj", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
		withSubjects("queue", "proj", serviceAccount("proj-queue", "worker")),
		withSubjects("own", "proj-extra", serviceAccount("proj-extra", "local")),
	)
	request := func(ns, name string) reconcile.Request {
		return reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}}
	}

	got := r.requestsForNamespace(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "proj-extra"}})
	assert.ElementsMatch(t, []reconcile.Request{request("proj", "account"), request("proj", "group"), request("other", "user")}, got)

	got = r.requestsForNamespace(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "unrelated"}})
	assert.Empty(t, got)
}

func TestMembershipMayChange(t *testing.T) {
	now := metav1.Now()
	labelled := func(project string) *corev1.Namespace {
		return namespace("proj-extra", project)
	}
	deleting := labelled("proj")
	deleting.DeletionTimestamp = &now
	annotated := labelled("proj")
	annotated.Annotations = map[string]string{"note": "x"}
	relabelled := labelled("proj")
	relabelled.Labels["team"] = "a"
	unlabelled := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "proj-extra"}}

	tests := []struct {
		name     string
		old      *corev1.Namespace
		updated  *corev1.Namespace
		expected bool
	}{
		{name: "the project label changes", old: labelled("proj"), updated: labelled("proj-extra"), expected: true},
		{name: "the project label is set", old: unlabelled, updated: labelled("proj"), expected: true},
		{name: "the project label is removed", old: labelled("proj"), updated: unlabelled, expected: true},
		{name: "the deletion starts", old: labelled("proj"), updated: deleting, expected: true},
		{name: "a deleting namespace changes", old: deleting, updated: deleting.DeepCopy()},
		{name: "another label changes", old: labelled("proj"), updated: relabelled},
		{name: "an annotation changes", old: labelled("proj"), updated: annotated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, membershipMayChange().Update(event.UpdateEvent{ObjectOld: tt.old, ObjectNew: tt.updated}))
		})
	}

	assert.True(t, membershipMayChange().Create(event.CreateEvent{Object: labelled("proj")}), "create")
	assert.True(t, membershipMayChange().Delete(event.DeleteEvent{Object: labelled("proj")}), "delete")
}
