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
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/apis/deckhouse.io/v1alpha3"
	namespacemanager "controller/internal/manager/namespace"
)

func newReconciler(t *testing.T, allowOrphanNamespaces bool, objs ...client.Object) *reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, v1alpha3.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &reconciler{
		init:                  new(sync.WaitGroup), // counter 0: Wait() returns immediately
		logger:                logr.Discard(),
		client:                c,
		manager:               namespacemanager.New(c, logr.Discard()),
		allowOrphanNamespaces: allowOrphanNamespaces,
	}
}

func TestIsAutoWrapCandidate(t *testing.T) {
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
			// created by a ProjectNamespace) must never be auto-wrapped into a separate project.
			name: "project-owned namespace is skipped",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "owned", Labels: map[string]string{v1alpha3.ResourceLabelProject: "owned", v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy}}},
			want: false,
		},
		{
			name: "additional project namespace is skipped",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a-backend", Labels: map[string]string{v1alpha3.ResourceLabelProject: "team-a", v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy}}},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isAutoWrapCandidate(tc.ns))
		})
	}
}

func TestReconcile_SyncsLabelsForAlreadyWrappedNamespace(t *testing.T) {
	// An already auto-wrapped namespace carries the project-ownership label (stamped by the project
	// reconciler) and the managed-project finalizer. A later user-label change must still propagate
	// into the managed project's spec.parameters.namespace.labels - this is the regression that made
	// the sync create-only.
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "team-a",
		Labels: map[string]string{
			"env":                          "prod", // user label added after the wrap
			v1alpha3.ResourceLabelProject:  "team-a",
			v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy,
		},
		Finalizers: []string{v1alpha3.NamespaceFinalizerManagedProject},
	}}
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{
		Name:   "team-a",
		Labels: map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace},
	}}
	require.False(t, isAutoWrapCandidate(ns), "an owned namespace is no longer an auto-wrap candidate")

	r := newReconciler(t, true, ns, project)
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team-a"}})
	require.NoError(t, err)

	updated := new(v1alpha3.Project)
	require.NoError(t, r.client.Get(context.Background(), client.ObjectKey{Name: "team-a"}, updated))
	require.NotNil(t, updated.Spec.Parameters["namespace"], "namespace parameters must be synced on update")
	nsParams := updated.Spec.Parameters["namespace"].(map[string]any)
	assert.Equal(t, map[string]any{"env": "prod"}, nsParams["labels"])
}

func TestPredicateShouldHandle(t *testing.T) {
	adopt := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "adopt-me", Annotations: map[string]string{v1alpha3.NamespaceAnnotationAdopt: ""}}}
	managed := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-managed", Finalizers: []string{v1alpha3.NamespaceFinalizerManagedProject}}}
	orphan := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	system := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}

	t.Run("orphan allowed when flag enabled", func(t *testing.T) {
		p := customPredicate[*corev1.Namespace]{logger: logr.Discard(), allowOrphanNamespaces: true}
		assert.True(t, p.shouldHandle(orphan))
		assert.False(t, p.shouldHandle(system))
	})

	t.Run("orphan ignored when flag disabled, adopt still handled", func(t *testing.T) {
		p := customPredicate[*corev1.Namespace]{logger: logr.Discard(), allowOrphanNamespaces: false}
		assert.False(t, p.shouldHandle(orphan))
		assert.True(t, p.shouldHandle(adopt))
	})

	t.Run("finalizer-marked namespace always handled", func(t *testing.T) {
		p := customPredicate[*corev1.Namespace]{logger: logr.Discard(), allowOrphanNamespaces: false}
		assert.True(t, p.shouldHandle(managed))
	})
}
