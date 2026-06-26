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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/apis/deckhouse.io/v1alpha3"
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
	return &Reconciler{Client: c}, c
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
	r, c := newReconciler(t, proj, pn)

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
	r, c := newReconciler(t, proj, pn)

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
