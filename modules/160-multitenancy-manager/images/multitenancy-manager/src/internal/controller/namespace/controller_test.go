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

package namespace

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"controller/apis/deckhouse.io/v1alpha3"
	namespacemanager "controller/internal/manager/namespace"
)

func newReconciler(t *testing.T, objs ...client.Object) *reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, v1alpha3.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &reconciler{
		init:    new(sync.WaitGroup), // counter 0: Wait() returns immediately
		logger:  logr.Discard(),
		client:  c,
		manager: namespacemanager.New(c, logr.Discard()),
	}
}

func TestIsAdoptionCandidate(t *testing.T) {
	cases := []struct {
		name string
		ns   *corev1.Namespace
		want bool
	}{
		{name: "plain user namespace", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}, want: true},
		{name: "default namespace", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, want: false},
		{name: "reserved d8 prefix", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-system"}}, want: false},
		{name: "reserved kube prefix", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}, want: false},
		{
			name: "deckhouse heritage",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "module-ns", Labels: map[string]string{v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageDeckhouse}}},
			want: false,
		},
		{
			// A namespace already owned by a project (its main namespace or an additional namespace
			// created by a ProjectNamespace) must never become a separate project.
			name: "project-owned namespace is skipped",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "owned", Labels: map[string]string{v1alpha3.ResourceLabelProject: "owned", v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy}}},
			want: false,
		},
		{
			name: "additional project namespace is skipped",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a-backend", Labels: map[string]string{v1alpha3.ResourceLabelProject: "team-a", v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy}}},
			want: false,
		},
		{
			name: "upmeter heritage",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "obsv", Labels: map[string]string{v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageUpmeter}}},
			want: false,
		},
		{
			name: "upmeter name prefix",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "upmeter-probe-namespace-foo"}},
			want: false,
		},
		{
			name: "name longer than the project limit",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: strings.Repeat("a", v1alpha3.ProjectNameMaxLength+1)}},
			want: false,
		},
		{
			name: "name at the project limit is still a candidate",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: strings.Repeat("a", v1alpha3.ProjectNameMaxLength)}},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isAdoptionCandidate(tc.ns))
		})
	}
}

func TestReconcile_AdoptsOrphanNamespace(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "team-a",
		Labels: map[string]string{"env": "prod"},
	}}

	r := newReconciler(t, ns)
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}})
	require.NoError(t, err)

	created := new(v1alpha3.Project)
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: "team-a"}, created))
	assert.Equal(t, namespacemanager.TemplateSimple, created.Spec.ProjectTemplateName)
	nsParams := created.Spec.Parameters["namespace"].(map[string]any)
	assert.Equal(t, map[string]any{"env": "prod"}, nsParams["labels"])
}

func TestReconcile_SkipsNamespaceOwnedByProject(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "team-a",
		Labels: map[string]string{
			v1alpha3.ResourceLabelProject:  "team-a",
			v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy,
		},
	}}

	r := newReconciler(t, ns)
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}})
	require.NoError(t, err)

	err = r.client.Get(context.Background(), client.ObjectKey{Name: "team-a"}, new(v1alpha3.Project))
	assert.True(t, apierrors.IsNotFound(err), "an owned namespace must not get a project of its own")
}

func TestReconcile_ClearsRetiredMarkersOnTerminatingNamespace(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "team-a",
		Labels: map[string]string{
			v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace,
		},
		Finalizers: []string{v1alpha3.NamespaceFinalizerManagedProject, "example.com/keep"},
	}}
	r := newReconciler(t, ns)
	require.NoError(t, r.client.Delete(context.Background(), ns))

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}})
	require.NoError(t, err)

	updated := new(corev1.Namespace)
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: "team-a"}, updated))
	assert.NotContains(t, updated.Labels, v1alpha3.ProjectLabelManagedByNamespace)
	assert.False(t, controllerutil.ContainsFinalizer(updated, v1alpha3.NamespaceFinalizerManagedProject))
	assert.True(t, controllerutil.ContainsFinalizer(updated, "example.com/keep"))

	err = r.client.Get(context.Background(), client.ObjectKey{Name: "team-a"}, new(v1alpha3.Project))
	assert.True(t, apierrors.IsNotFound(err), "a terminating namespace must not be adopted")
}

func TestReconcile_ClearsRetiredMarkersWhenAlreadyOwned(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "team-a",
		Labels: map[string]string{
			v1alpha3.ResourceLabelProject:           "team-a",
			v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace,
		},
		Finalizers: []string{v1alpha3.NamespaceFinalizerManagedProject},
	}}
	existing := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a"},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: namespacemanager.TemplateSimple},
	}
	r := newReconciler(t, ns, existing)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}})
	require.NoError(t, err)

	updated := new(corev1.Namespace)
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: "team-a"}, updated))
	assert.NotContains(t, updated.Labels, v1alpha3.ProjectLabelManagedByNamespace)
	assert.False(t, controllerutil.ContainsFinalizer(updated, v1alpha3.NamespaceFinalizerManagedProject))
}

func TestReconcile_SkipsTerminatingNamespace(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:       "team-a",
		Finalizers: []string{"example.com/keep"},
	}}
	r := newReconciler(t, ns)
	require.NoError(t, r.client.Delete(context.Background(), ns))

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}})
	require.NoError(t, err)

	err = r.client.Get(context.Background(), client.ObjectKey{Name: "team-a"}, new(v1alpha3.Project))
	assert.True(t, apierrors.IsNotFound(err), "a namespace on its way out must not be adopted")
}

func TestPredicate(t *testing.T) {
	orphan := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	system := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}
	owned := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "owned", Labels: map[string]string{v1alpha3.ResourceLabelProject: "owned"}}}

	p := customPredicate[*corev1.Namespace]{logger: logr.Discard()}

	t.Run("orphan namespace is handled", func(t *testing.T) {
		assert.True(t, p.Create(event.TypedCreateEvent[*corev1.Namespace]{Object: orphan}))
	})

	t.Run("system namespace is ignored", func(t *testing.T) {
		assert.False(t, p.Create(event.TypedCreateEvent[*corev1.Namespace]{Object: system}))
	})

	t.Run("project-owned namespace is ignored", func(t *testing.T) {
		assert.False(t, p.Update(event.TypedUpdateEvent[*corev1.Namespace]{ObjectOld: owned, ObjectNew: owned}))
	})

	t.Run("leftover markers still enqueue a project-owned namespace", func(t *testing.T) {
		leftover := owned.DeepCopy()
		leftover.Finalizers = []string{v1alpha3.NamespaceFinalizerManagedProject}
		assert.True(t, p.Update(event.TypedUpdateEvent[*corev1.Namespace]{ObjectOld: leftover, ObjectNew: leftover}))
	})

	t.Run("delete events are never handled", func(t *testing.T) {
		assert.False(t, p.Delete(event.TypedDeleteEvent[*corev1.Namespace]{Object: orphan}))
	})
}
