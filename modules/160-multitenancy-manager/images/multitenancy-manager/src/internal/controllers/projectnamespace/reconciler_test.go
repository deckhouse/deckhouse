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

package projectnamespace

import (
	"context"
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/naming"
)

func newReconciler(t *testing.T, objs ...client.Object) (*Reconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, v1alpha3.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&v1alpha3.ProjectNamespace{}).
		Build()
	return &Reconciler{Client: c, Recorder: record.NewFakeRecorder(20)}, c
}

// mainNamespace is the rendered main namespace of a project: it carries the template-labels
// annotation the renderer writes, plus the given labels.
func mainNamespace(project, templateLabels string, labels map[string]string) *corev1.Namespace {
	all := map[string]string{v1alpha3.ResourceLabelProject: project}
	for k, v := range labels {
		all[k] = v
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: project, Labels: all}}
	if templateLabels != "" {
		ns.Annotations = map[string]string{naming.TemplateLabelsAnnotation: templateLabels}
	}
	return ns
}

func projectNamespace(name, namespace, suffix string) *v1alpha3.ProjectNamespace {
	return &v1alpha3.ProjectNamespace{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       v1alpha3.ProjectNamespaceSpec{Name: suffix},
	}
}

func runReconcile(t *testing.T, r *Reconciler, namespace, name string) {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}})
	require.NoError(t, err)
}

func TestReconcile_CreatesNamespaceAndStatus(t *testing.T) {
	proj := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	pn := projectNamespace("backend", "team-a", "backend")
	r, c := newReconciler(t, proj, pn, mainNamespace("team-a", "{}", nil))

	// first pass adds the finalizer, second creates the namespace and writes the status.
	runReconcile(t, r, "team-a", "backend")
	runReconcile(t, r, "team-a", "backend")

	ns := &corev1.Namespace{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "team-a-backend"}, ns))
	assert.Equal(t, "team-a", ns.Labels[v1alpha3.ResourceLabelProject])
	assert.Equal(t, v1alpha3.ResourceHeritageMultitenancy, ns.Labels[v1alpha3.ResourceLabelHeritage])
	assert.Equal(t, "backend", ns.Labels[v1alpha3.ResourceLabelProjectNamespace])

	got := &v1alpha3.ProjectNamespace{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "backend"}, got))
	assert.Equal(t, "team-a-backend", got.Status.Namespace)
	assert.Contains(t, got.Finalizers, v1alpha3.ProjectNamespaceFinalizer)
	require.Len(t, got.Status.Conditions, 1)
	assert.Equal(t, corev1.ConditionTrue, got.Status.Conditions[0].Status)
}

func TestReconcile_DeletionRemovesNamespaceAndFinalizer(t *testing.T) {
	proj := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	pn := projectNamespace("backend", "team-a", "backend")
	r, c := newReconciler(t, proj, pn, mainNamespace("team-a", "{}", nil))

	runReconcile(t, r, "team-a", "backend")
	runReconcile(t, r, "team-a", "backend")

	// delete the object: the finalizer keeps it around (with DeletionTimestamp set) for cleanup.
	got := &v1alpha3.ProjectNamespace{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "backend"}, got))
	require.NoError(t, c.Delete(context.Background(), got))

	runReconcile(t, r, "team-a", "backend")

	ns := &corev1.Namespace{}
	err := c.Get(context.Background(), client.ObjectKey{Name: "team-a-backend"}, ns)
	assert.True(t, k8serrors.IsNotFound(err), "the additional namespace must be deleted")

	err = c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "backend"}, got)
	assert.True(t, k8serrors.IsNotFound(err), "the object must be gone once the finalizer is removed")
}

func TestReconcile_ProjectGoneCleansUp(t *testing.T) {
	// no Project object: a ProjectNamespace whose project disappeared must clean up its namespace.
	pn := projectNamespace("backend", "team-a", "backend")
	pn.Finalizers = []string{v1alpha3.ProjectNamespaceFinalizer}
	existing := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "team-a-backend",
		Labels: map[string]string{v1alpha3.ResourceLabelProject: "team-a"},
	}}
	r, c := newReconciler(t, pn, existing)

	runReconcile(t, r, "team-a", "backend")

	ns := &corev1.Namespace{}
	err := c.Get(context.Background(), client.ObjectKey{Name: "team-a-backend"}, ns)
	assert.True(t, k8serrors.IsNotFound(err), "the namespace of an orphaned ProjectNamespace must be removed")
}

func TestReconcile_RefusesForeignNamespace(t *testing.T) {
	proj := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	pn := projectNamespace("backend", "team-a", "backend")
	foreign := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "team-a-backend",
		Labels: map[string]string{v1alpha3.ResourceLabelProject: "team-b"},
	}}
	r, c := newReconciler(t, proj, pn, foreign)

	// the reconcile must fail because the target namespace is owned by another project.
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "team-a", Name: "backend"}})
	require.Error(t, err)

	// the foreign namespace must keep its owner.
	ns := &corev1.Namespace{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "team-a-backend"}, ns))
	assert.Equal(t, "team-b", ns.Labels[v1alpha3.ResourceLabelProject])
}

// reconciledNamespace runs the two passes a new ProjectNamespace needs (finalizer, then namespace) and
// returns the additional namespace.
func reconciledNamespace(t *testing.T, r *Reconciler, c client.Client) *corev1.Namespace {
	t.Helper()
	runReconcile(t, r, "team-a", "backend")
	runReconcile(t, r, "team-a", "backend")
	ns := &corev1.Namespace{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "team-a-backend"}, ns))
	return ns
}

func setMainTemplateLabels(t *testing.T, c client.Client, value string) {
	t.Helper()
	main := &corev1.Namespace{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "team-a"}, main))
	main.Annotations[naming.TemplateLabelsAnnotation] = value
	require.NoError(t, c.Update(context.Background(), main))
}

// TestReconcile_InheritsTemplateLabels: the additional namespace gets exactly the labels listed in the
// template-labels annotation of the main namespace, records them in its own annotation, and does not
// take labels an administrator put on the main namespace by hand.
func TestReconcile_InheritsTemplateLabels(t *testing.T) {
	proj := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	pn := projectNamespace("backend", "team-a", "backend")
	pn.Labels = map[string]string{"team": "tenant-choice"}
	main := mainNamespace("team-a", `{"env":"prod","team":"backend"}`, map[string]string{
		"env": "prod", "team": "backend", "hand-made": "only-main", "security.deckhouse.io/pod-policy": "baseline",
	})
	r, c := newReconciler(t, proj, pn, main)

	ns := reconciledNamespace(t, r, c)
	assert.Equal(t, "prod", ns.Labels["env"])
	assert.Equal(t, "backend", ns.Labels["team"], "the ProjectNamespace's own labels are never read")
	assert.NotContains(t, ns.Labels, "hand-made", "a label set by hand on the main namespace is not inherited")
	assert.Equal(t, "baseline", ns.Labels["security.deckhouse.io/pod-policy"], "the fixed inherited labels stay")
	assert.JSONEq(t, `{"env":"prod","team":"backend"}`, ns.Annotations[naming.TemplateLabelsAnnotation])
}

// TestReconcile_FollowsTemplateLabelChanges: a changed value is updated, a key the template no longer
// sets is removed, and a label the administrator put on the additional namespace is left alone.
func TestReconcile_FollowsTemplateLabelChanges(t *testing.T) {
	proj := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	pn := projectNamespace("backend", "team-a", "backend")
	r, c := newReconciler(t, proj, pn, mainNamespace("team-a", `{"env":"prod","tier":"gold"}`, nil))

	ns := reconciledNamespace(t, r, c)
	ns.Labels["admin-note"] = "keep-me"
	require.NoError(t, c.Update(context.Background(), ns))

	setMainTemplateLabels(t, c, `{"env":"staging"}`)
	ns = reconciledNamespace(t, r, c)

	assert.Equal(t, "staging", ns.Labels["env"])
	assert.NotContains(t, ns.Labels, "tier", "a key the template dropped is removed")
	assert.Equal(t, "keep-me", ns.Labels["admin-note"], "a label that was never the template's is untouched")
	assert.JSONEq(t, `{"env":"staging"}`, ns.Annotations[naming.TemplateLabelsAnnotation])

	recorder, ok := r.Recorder.(*record.FakeRecorder)
	require.True(t, ok)
	assert.Empty(t, recorder.Events, "a change of the template is not a restored hand edit")
}

// TestReconcile_RevertsAHandEditOfATemplateLabel: the template owns its labels on the additional
// namespace; a direct edit is put back and reported with a Warning event.
func TestReconcile_RevertsAHandEditOfATemplateLabel(t *testing.T) {
	proj := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	pn := projectNamespace("backend", "team-a", "backend")
	r, c := newReconciler(t, proj, pn, mainNamespace("team-a", `{"env":"prod"}`, nil))

	ns := reconciledNamespace(t, r, c)
	ns.Labels["env"] = "dev"
	require.NoError(t, c.Update(context.Background(), ns))

	ns = reconciledNamespace(t, r, c)
	assert.Equal(t, "prod", ns.Labels["env"])

	recorder, ok := r.Recorder.(*record.FakeRecorder)
	require.True(t, ok)
	select {
	case event := <-recorder.Events:
		assert.Contains(t, event, "Warning")
		assert.Contains(t, event, "env")
	default:
		t.Fatal("reverting a hand edit must emit an event")
	}
}

// TestReconcile_TemplateLabelsCannotOverrideModuleKeys: even if the annotation lists a module-owned or
// GitOps tracking key, the additional namespace keeps its own ownership labels and does not get the
// tracking label.
func TestReconcile_TemplateLabelsCannotOverrideModuleKeys(t *testing.T) {
	proj := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	pn := projectNamespace("backend", "team-a", "backend")
	annotation := `{"projects.deckhouse.io/project":"team-b","projects.deckhouse.io/project-namespace":"x","heritage":"deckhouse","app.kubernetes.io/instance":"argo","ok":"yes"}`
	r, c := newReconciler(t, proj, pn, mainNamespace("team-a", annotation, nil))

	ns := reconciledNamespace(t, r, c)
	assert.Equal(t, "team-a", ns.Labels[v1alpha3.ResourceLabelProject])
	assert.Equal(t, "backend", ns.Labels[v1alpha3.ResourceLabelProjectNamespace])
	assert.Equal(t, v1alpha3.ResourceHeritageMultitenancy, ns.Labels[v1alpha3.ResourceLabelHeritage])
	assert.NotContains(t, ns.Labels, "app.kubernetes.io/instance")
	assert.Equal(t, "yes", ns.Labels["ok"])
	assert.JSONEq(t, `{"ok":"yes"}`, ns.Annotations[naming.TemplateLabelsAnnotation])
}

// TestReconcile_WaitsForARenderedMainNamespace: without the main namespace, or before the renderer has
// written its template labels, a new additional namespace is not created (it would miss the labels a
// restriction may rely on) and an existing one is not touched.
func TestReconcile_WaitsForARenderedMainNamespace(t *testing.T) {
	for name, main := range map[string]*corev1.Namespace{
		"no main namespace":        nil,
		"no annotation yet":        mainNamespace("team-a", "", nil),
		"an unreadable annotation": mainNamespace("team-a", "not json", nil),
	} {
		t.Run(name, func(t *testing.T) {
			proj := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
			pn := projectNamespace("backend", "team-a", "backend")
			pn.Finalizers = []string{v1alpha3.ProjectNamespaceFinalizer}
			objs := []client.Object{proj, pn}
			if main != nil {
				objs = append(objs, main)
			}
			r, c := newReconciler(t, objs...)

			_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "team-a", Name: "backend"}})
			require.Error(t, err)

			ns := &corev1.Namespace{}
			assert.True(t, k8serrors.IsNotFound(c.Get(context.Background(), client.ObjectKey{Name: "team-a-backend"}, ns)))

			got := &v1alpha3.ProjectNamespace{}
			require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "backend"}, got))
			require.Len(t, got.Status.Conditions, 1)
			assert.Equal(t, corev1.ConditionFalse, got.Status.Conditions[0].Status)
			assert.Contains(t, got.Status.Conditions[0].Message, "team-a")
		})
	}

	t.Run("an existing namespace keeps its labels", func(t *testing.T) {
		proj := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
		pn := projectNamespace("backend", "team-a", "backend")
		pn.Finalizers = []string{v1alpha3.ProjectNamespaceFinalizer}
		existing := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:        "team-a-backend",
			Labels:      map[string]string{v1alpha3.ResourceLabelProject: "team-a", "env": "prod"},
			Annotations: map[string]string{naming.TemplateLabelsAnnotation: `{"env":"prod"}`},
		}}
		r, c := newReconciler(t, proj, pn, existing, mainNamespace("team-a", "", nil))

		_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "team-a", Name: "backend"}})
		require.Error(t, err)

		ns := &corev1.Namespace{}
		require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "team-a-backend"}, ns))
		assert.Equal(t, "prod", ns.Labels["env"])
	})
}

// TestReconcile_SecondPassWritesNothing: an unchanged project must not rewrite the namespace, or the
// namespace watch would loop.
func TestReconcile_SecondPassWritesNothing(t *testing.T) {
	proj := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	pn := projectNamespace("backend", "team-a", "backend")
	r, c := newReconciler(t, proj, pn, mainNamespace("team-a", `{"b":"2","a":"1"}`, nil))

	before := reconciledNamespace(t, r, c)
	after := reconciledNamespace(t, r, c)
	assert.Equal(t, before.ResourceVersion, after.ResourceVersion)
}

// TestNewNamespaceLabelsIsWhatTheReconcilerSets: the grant policy status predicts the labels of a new
// additional namespace with NewNamespaceLabels; it must be exactly what ensureNamespace gives a fresh
// namespace, apart from the label derived from the ProjectNamespace name.
func TestNewNamespaceLabelsIsWhatTheReconcilerSets(t *testing.T) {
	proj := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	pn := projectNamespace("backend", "team-a", "backend")
	main := mainNamespace("team-a", `{"env":"prod","app.kubernetes.io/instance":"argo","heritage":"forged"}`, map[string]string{
		"env": "prod", "hand-made": "only-main", "security.deckhouse.io/pod-policy": "baseline",
		"extended-monitoring.deckhouse.io/enabled": "", v1alpha3.ResourceLabelTemplate: "default",
	})
	r, c := newReconciler(t, proj, pn, main)

	ns := reconciledNamespace(t, r, c)
	predicted, err := NewNamespaceLabels(main, "team-a")
	require.NoError(t, err)

	got := maps.Clone(ns.Labels)
	delete(got, v1alpha3.ResourceLabelProjectNamespace)
	assert.Equal(t, predicted, got)
}

// TestReconcile_RevertsAHandRemovalOfATemplateLabel: a template label removed by hand from an additional
// namespace comes back, and the removal is reported like any other hand edit.
func TestReconcile_RevertsAHandRemovalOfATemplateLabel(t *testing.T) {
	proj := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	pn := projectNamespace("backend", "team-a", "backend")
	r, c := newReconciler(t, proj, pn, mainNamespace("team-a", `{"env":"prod"}`, nil))

	ns := reconciledNamespace(t, r, c)
	delete(ns.Labels, "env")
	require.NoError(t, c.Update(context.Background(), ns))

	ns = reconciledNamespace(t, r, c)
	assert.Equal(t, "prod", ns.Labels["env"])

	recorder, ok := r.Recorder.(*record.FakeRecorder)
	require.True(t, ok)
	select {
	case event := <-recorder.Events:
		assert.Contains(t, event, "Warning")
		assert.Contains(t, event, `env="prod" (was removed)`)
	default:
		t.Fatal("restoring a removed template label must emit an event")
	}
}
