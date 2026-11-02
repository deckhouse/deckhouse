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

package projectgrants

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	grantsv1alpha1 "controller/api/v1alpha1"
	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/controllers/templategrants"
)

func newReconciler(t *testing.T, objects ...client.Object) (*Reconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{v1alpha3.AddToScheme, grantsv1alpha1.AddToScheme, grantsv1alpha2.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	return &Reconciler{Client: c, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}, c
}

func project(name string, grants *v1alpha3.ProjectGrants) *v1alpha3.Project {
	return &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID(name + "-uid")}, Spec: v1alpha3.ProjectSpec{Grants: grants}}
}

func libraryPolicy(name string, resources ...grantsv1alpha1.GrantResource) *grantsv1alpha2.ClusterResourceGrantPolicy {
	return &grantsv1alpha2.ClusterResourceGrantPolicy{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: grantsv1alpha2.ClusterResourceGrantPolicySpec{Resources: resources}}
}

func reconcileProject(t *testing.T, r *Reconciler, name string) ctrl.Result {
	t.Helper()
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	require.NoError(t, err)
	return result
}

func policy(t *testing.T, c client.Client, name string) *grantsv1alpha2.ClusterResourceGrantPolicy {
	t.Helper()
	out := new(grantsv1alpha2.ClusterResourceGrantPolicy)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: name}, out))
	return out
}

func TestReconcileMaterializesTheGrantsOfAProject(t *testing.T) {
	storage := grantsv1alpha1.GrantResource{ResourceName: "storageclasses", Allowed: []string{"fast"}}
	ingress := grantsv1alpha1.GrantResource{ResourceName: "ingressclasses", Allowed: []string{"nginx"}}
	r, c := newReconciler(t,
		project("team", &v1alpha3.ProjectGrants{Resources: []grantsv1alpha1.GrantResource{storage}, GrantPolicies: []string{"web"}}),
		libraryPolicy("web", ingress))
	reconcileProject(t, r, "team")

	inline := policy(t, c, "project-team-inline")
	assert.Equal(t, map[string]string{v1alpha3.ResourceLabelProject: "team"}, inline.Spec.ProjectSelector.MatchLabels,
		"the policy applies to every namespace of the project")
	assert.Nil(t, inline.Spec.NamespaceSelector)
	assert.Equal(t, []grantsv1alpha1.GrantResource{storage}, inline.Spec.Resources)
	assert.Equal(t, "team", inline.Labels[LabelManagedByProject])
	assert.Equal(t, templategrants.GrantSourceInline, inline.Labels[templategrants.LabelGrantSource])
	require.Len(t, inline.OwnerReferences, 1, "deleting the project deletes its policies")
	assert.Equal(t, v1alpha3.ProjectKind, inline.OwnerReferences[0].Kind)

	copied := policy(t, c, "project-team-web")
	assert.Equal(t, []grantsv1alpha1.GrantResource{ingress}, copied.Spec.Resources)
}

func TestReconcilePrunesWhatTheProjectNoLongerGrants(t *testing.T) {
	r, c := newReconciler(t,
		project("team", &v1alpha3.ProjectGrants{Resources: []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses"}}, GrantPolicies: []string{"web"}}),
		libraryPolicy("web"), libraryPolicy("project-other-inline"))
	reconcileProject(t, r, "team")

	p := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "team"}, p))
	p.Spec.Grants = &v1alpha3.ProjectGrants{GrantPolicies: []string{"web"}}
	require.NoError(t, c.Update(context.Background(), p))
	reconcileProject(t, r, "team")
	err := c.Get(context.Background(), client.ObjectKey{Name: "project-team-inline"}, new(grantsv1alpha2.ClusterResourceGrantPolicy))
	assert.True(t, apierrors.IsNotFound(err))
	policy(t, c, "project-team-web")

	p.Spec.Grants = nil
	require.NoError(t, c.Update(context.Background(), p))
	reconcileProject(t, r, "team")
	err = c.Get(context.Background(), client.ObjectKey{Name: "project-team-web"}, new(grantsv1alpha2.ClusterResourceGrantPolicy))
	assert.True(t, apierrors.IsNotFound(err))
	policy(t, c, "project-other-inline")
}

// A policy of the managed name that the project does not manage is not taken over.
func TestReconcileLeavesAPolicyItDoesNotManage(t *testing.T) {
	foreign := libraryPolicy("project-team-inline", grantsv1alpha1.GrantResource{ResourceName: "ingressclasses"})
	r, c := newReconciler(t, project("team", &v1alpha3.ProjectGrants{Resources: []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses"}}}), foreign)
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team"}})
	require.Error(t, err)
	assert.Equal(t, "ingressclasses", policy(t, c, "project-team-inline").Spec.Resources[0].ResourceName)
}

// A missing library policy is reported on the project and not polled for: creating it enqueues the
// projects that list it, and the copy of a policy that went away is pruned.
func TestReconcileReportsAMissingLibraryPolicy(t *testing.T) {
	r, c := newReconciler(t, project("team", &v1alpha3.ProjectGrants{GrantPolicies: []string{"later"}}))
	assert.Equal(t, ctrl.Result{}, reconcileProject(t, r, "team"), "no requeue")
	events := takeEvents(r)
	require.Len(t, events, 1)
	assert.Contains(t, events[0], "Warning "+reasonGrantMissing)
	assert.Contains(t, events[0], `"later"`)

	require.NoError(t, c.Create(context.Background(), libraryPolicy("later", grantsv1alpha1.GrantResource{ResourceName: "storageclasses"})))
	assert.Equal(t, ctrl.Result{}, reconcileProject(t, r, "team"))
	assert.Empty(t, takeEvents(r))
	policy(t, c, "project-team-later")

	require.NoError(t, c.Delete(context.Background(), libraryPolicy("later")))
	assert.Equal(t, ctrl.Result{}, reconcileProject(t, r, "team"))
	assert.Len(t, takeEvents(r), 1)
	err := c.Get(context.Background(), client.ObjectKey{Name: "project-team-later"}, new(grantsv1alpha2.ClusterResourceGrantPolicy))
	assert.True(t, apierrors.IsNotFound(err), "the copy of a deleted library policy is pruned")
}

// A source that cannot be applied does not keep a removed one from being pruned, and does not keep the
// others from being applied; the failure is reported on the project.
func TestReconcileKeepsPruningWhenASourceFails(t *testing.T) {
	foreign := libraryPolicy("project-team-inline", grantsv1alpha1.GrantResource{ResourceName: "ingressclasses"})
	removed := libraryPolicy("project-team-gone", grantsv1alpha1.GrantResource{ResourceName: "storageclasses"})
	removed.Labels = map[string]string{LabelManagedByProject: "team"}
	r, c := newReconciler(t,
		project("team", &v1alpha3.ProjectGrants{
			Resources:     []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses"}},
			GrantPolicies: []string{"web"},
		}),
		libraryPolicy("web", grantsv1alpha1.GrantResource{ResourceName: "ingressclasses"}), foreign, removed)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team"}})
	require.ErrorContains(t, err, "project-team-inline")

	gone := c.Get(context.Background(), client.ObjectKey{Name: "project-team-gone"}, new(grantsv1alpha2.ClusterResourceGrantPolicy))
	assert.True(t, apierrors.IsNotFound(gone), "a revoked grant is pruned although another source fails")
	policy(t, c, "project-team-web")
	assert.Equal(t, "ingressclasses", policy(t, c, "project-team-inline").Spec.Resources[0].ResourceName, "the foreign policy is left alone")

	events := takeEvents(r)
	require.Len(t, events, 1)
	assert.Contains(t, events[0], "Warning "+reasonGrantNotApplied)
	assert.Contains(t, events[0], "spec.grants.resources")
	assert.Contains(t, events[0], "project-team-inline")
}

// A library policy that cannot be read keeps its managed policy as it is, and the grant removed in the
// same change is pruned all the same.
func TestReconcileKeepsPruningWhenAPolicyCannotBeRead(t *testing.T) {
	kept := libraryPolicy("project-team-web", grantsv1alpha1.GrantResource{ResourceName: "ingressclasses"})
	kept.Labels = map[string]string{LabelManagedByProject: "team"}
	removed := libraryPolicy("project-team-gone", grantsv1alpha1.GrantResource{ResourceName: "storageclasses"})
	removed.Labels = map[string]string{LabelManagedByProject: "team"}
	r, c := newReconciler(t, project("team", &v1alpha3.ProjectGrants{GrantPolicies: []string{"web"}}), libraryPolicy("web"), kept, removed)
	r.Client = interceptor.NewClient(c.(client.WithWatch), interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if key.Name == "web" {
				return errors.New("unavailable")
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team"}})
	require.ErrorContains(t, err, "get grant policy web")

	gone := c.Get(context.Background(), client.ObjectKey{Name: "project-team-gone"}, new(grantsv1alpha2.ClusterResourceGrantPolicy))
	assert.True(t, apierrors.IsNotFound(gone), "the removed grant is pruned")
	assert.Equal(t, "ingressclasses", policy(t, c, "project-team-web").Spec.Resources[0].ResourceName, "the policy that could not be read is left as it is")
}

func TestReconcileJoinsTheErrorsOfEverySource(t *testing.T) {
	r, c := newReconciler(t,
		project("team", &v1alpha3.ProjectGrants{
			Resources:     []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses"}},
			GrantPolicies: []string{"web", "db"},
		}),
		libraryPolicy("web"), libraryPolicy("db"),
		libraryPolicy("project-team-inline"), libraryPolicy("project-team-web"))

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "team"}})
	require.ErrorContains(t, err, "project-team-inline")
	require.ErrorContains(t, err, "project-team-web")
	policy(t, c, "project-team-db")
	assert.Len(t, takeEvents(r), 2)
}

// takeEvents returns the events the reconciler recorded so far.
func takeEvents(r *Reconciler) []string {
	recorder, ok := r.Recorder.(*record.FakeRecorder)
	if !ok {
		return nil
	}
	var events []string
	for {
		select {
		case event := <-recorder.Events:
			events = append(events, event)
		default:
			return events
		}
	}
}

func TestProjectsReferencing(t *testing.T) {
	r, _ := newReconciler(t,
		project("a", &v1alpha3.ProjectGrants{GrantPolicies: []string{"web"}}),
		project("b", &v1alpha3.ProjectGrants{GrantPolicies: []string{"db"}}),
		project("c", nil))
	requests := r.projectsReferencing(context.Background(), libraryPolicy("web"))
	require.Len(t, requests, 1)
	assert.Equal(t, "a", requests[0].Name)

	managed := libraryPolicy("project-a-web")
	managed.Labels = map[string]string{LabelManagedByProject: "a"}
	assert.Empty(t, r.projectsReferencing(context.Background(), managed))
}

func TestManagedNames(t *testing.T) {
	assert.Empty(t, ManagedNames(project("team", nil)))
	assert.Equal(t, []string{"project-team-inline", "project-team-web"},
		ManagedNames(project("team", &v1alpha3.ProjectGrants{Resources: []grantsv1alpha1.GrantResource{{ResourceName: "x"}}, GrantPolicies: []string{"web"}})))
}
