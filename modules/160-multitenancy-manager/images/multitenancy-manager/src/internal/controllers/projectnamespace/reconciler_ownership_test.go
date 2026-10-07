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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha3"
)

// admittedProjectNamespace is the "backend" ProjectNamespace of the "team-a" project after its first
// reconcile, which added the finalizer while the "team-a-backend" name was still free.
func admittedProjectNamespace() *v1alpha3.ProjectNamespace {
	pn := projectNamespace("backend", "team-a", "backend")
	pn.Finalizers = []string{v1alpha3.ProjectNamespaceFinalizer}
	return pn
}

func readyCondition(t *testing.T, c client.Client) v1alpha3.Condition {
	t.Helper()
	pn := &v1alpha3.ProjectNamespace{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "backend"}, pn))
	for _, condition := range pn.Status.Conditions {
		if condition.Type == v1alpha3.ProjectNamespaceConditionReady {
			return condition
		}
	}
	require.FailNow(t, "the ProjectNamespace has no Ready condition")
	return v1alpha3.Condition{}
}

// A namespace that already has the resulting name is the project's only when it carries the project
// label. The controller puts the label on every namespace it creates and nobody else may, so any
// other namespace under that name was made by someone else, and the ProjectNamespace does not take it:
// it reports the namespace and leaves it as it is.
func TestReconcile_TakesOnlyANamespaceOfTheProject(t *testing.T) {
	cases := []struct {
		name   string
		labels map[string]string
		// refusal is the Ready=False message; empty means the namespace is not refused
		refusal string
	}{
		{
			name:    "a namespace without the project label is refused",
			labels:  map[string]string{"app": "cache"},
			refusal: `namespace "team-a-backend" already exists and is not owned by project "team-a"`,
		},
		{
			name:    "a namespace of another project is refused",
			labels:  map[string]string{v1alpha3.ResourceLabelProject: "team-b"},
			refusal: `namespace "team-a-backend" already exists and is owned by project "team-b"`,
		},
		{
			name: "a namespace of the project is taken",
			labels: map[string]string{
				v1alpha3.ResourceLabelHeritage:         v1alpha3.ResourceHeritageMultitenancy,
				v1alpha3.ResourceLabelProject:          "team-a",
				v1alpha3.ResourceLabelProjectNamespace: "backend",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			existing := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a-backend", Labels: tc.labels}}
			r, c := newReconciler(t,
				&v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
				admittedProjectNamespace(),
				existing,
			)

			_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "team-a", Name: "backend"}})

			ns := &corev1.Namespace{}
			require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "team-a-backend"}, ns))
			condition := readyCondition(t, c)
			if tc.refusal == "" {
				// The namespace is not refused as someone else's and stays the project's.
				assert.NotContains(t, condition.Message, "already exists")
				assert.Equal(t, "team-a", ns.Labels[v1alpha3.ResourceLabelProject])
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.refusal)
			assert.Equal(t, corev1.ConditionFalse, condition.Status)
			assert.Equal(t, tc.refusal, condition.Message)
			assert.Equal(t, tc.labels, ns.Labels, "the namespace keeps its labels")
		})
	}
}

// Deleting a ProjectNamespace that was refused a namespace of someone else removes its finalizer and
// leaves that namespace in place.
func TestReconcile_DeletionLeavesARefusedNamespace(t *testing.T) {
	existing := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "team-a-backend",
		Labels: map[string]string{"app": "cache"},
	}}
	r, c := newReconciler(t,
		&v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
		admittedProjectNamespace(),
		existing,
	)
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "team-a", Name: "backend"}})
	require.Error(t, err)

	pn := &v1alpha3.ProjectNamespace{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "backend"}, pn))
	require.NoError(t, c.Delete(context.Background(), pn))
	runReconcile(t, r, "team-a", "backend")

	ns := &corev1.Namespace{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "team-a-backend"}, ns))
	assert.Equal(t, map[string]string{"app": "cache"}, ns.Labels)
	err = c.Get(context.Background(), client.ObjectKey{Namespace: "team-a", Name: "backend"}, pn)
	assert.True(t, k8serrors.IsNotFound(err), "the ProjectNamespace goes once its finalizer is removed")
}
