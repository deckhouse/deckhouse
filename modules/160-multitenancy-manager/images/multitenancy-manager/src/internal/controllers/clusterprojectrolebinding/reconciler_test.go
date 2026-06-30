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

package clusterprojectrolebinding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/rolebinding"
)

func reconcileCPRB(t *testing.T, r *Reconciler, name string) {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	require.NoError(t, err)
}

func serviceRoleBinding(name, namespace, role string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rolebinding.CPRBServiceName(name),
			Namespace: namespace,
			Labels:    map[string]string{v1alpha3.ResourceLabelOwnedByCPRB: name},
		},
		RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: role},
	}
}

func newReconciler(t *testing.T, objs ...client.Object) (*Reconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		rbacv1.AddToScheme, v1alpha3.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&v1alpha3.ClusterProjectRoleBinding{}).
		Build()
	return &Reconciler{Client: c}, c
}

func project(name string, virtual bool, namespaces ...string) *v1alpha3.Project {
	p := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if virtual {
		p.Labels = map[string]string{v1alpha3.ProjectLabelVirtualProject: "true"}
	}
	for _, ns := range namespaces {
		kind := v1alpha3.NamespaceKindAdditional
		if ns == name {
			kind = v1alpha3.NamespaceKindMain
		}
		p.Status.Namespaces = append(p.Status.Namespaces, v1alpha3.NamespaceStatus{Name: ns, Kind: kind})
	}
	return p
}

func cprb(name, role string) *v1alpha3.ClusterProjectRoleBinding {
	return &v1alpha3.ClusterProjectRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha3.ClusterProjectRoleBindingSpec{
			Subjects: []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: "Group", Name: "platform"}},
			RoleRef:  v1alpha3.RoleRef{Kind: "ClusterRole", Name: role},
		},
	}
}

func TestReconcile_FansOutToAllNonVirtualProjects(t *testing.T) {
	r, c := newReconciler(t,
		cprb("global-viewer", "d8:project:viewer"),
		project("alpha", false, "alpha", "alpha-extra"),
		project("beta", false, "beta"),
		project("default", true, "default"),
	)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "global-viewer"}})
	assert.NoError(t, err)

	name := rolebinding.CPRBServiceName("global-viewer")
	for _, ns := range []string{"alpha", "alpha-extra", "beta"} {
		assert.NoErrorf(t, c.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: name}, &rbacv1.RoleBinding{}),
			"RoleBinding must exist in namespace %s", ns)
	}

	// the virtual project must NOT receive the binding
	err = c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &rbacv1.RoleBinding{})
	assert.Error(t, err)

	// status reflects the bound non-virtual projects
	got := &v1alpha3.ClusterProjectRoleBinding{}
	assert.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "global-viewer"}, got))
	assert.Equal(t, int32(2), got.Status.BoundProjects)
	assert.Contains(t, got.Finalizers, v1alpha3.ClusterProjectRoleBindingFinalizer)
}

// TestReconcile_DeletionRemovesFinalizerAndBindings mirrors the PRB deletion path: the finalizer is
// removed only after the fanned-out service RoleBindings are cleaned up.
func TestReconcile_DeletionRemovesFinalizerAndBindings(t *testing.T) {
	t.Parallel()
	r, c := newReconciler(t, cprb("global-viewer", "d8:project:viewer"), project("alpha", false, "alpha"))

	// first pass adds the finalizer and fans out the binding
	reconcileCPRB(t, r, "global-viewer")
	name := rolebinding.CPRBServiceName("global-viewer")
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "alpha", Name: name}, &rbacv1.RoleBinding{}))

	// delete the CPRB: the finalizer keeps it around (with DeletionTimestamp) for cleanup
	got := &v1alpha3.ClusterProjectRoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "global-viewer"}, got))
	require.NoError(t, c.Delete(context.Background(), got))

	reconcileCPRB(t, r, "global-viewer")

	err := c.Get(context.Background(), client.ObjectKey{Namespace: "alpha", Name: name}, &rbacv1.RoleBinding{})
	assert.True(t, k8serrors.IsNotFound(err), "the fanned-out RoleBinding must be cleaned up on deletion")
	err = c.Get(context.Background(), client.ObjectKey{Name: "global-viewer"}, &v1alpha3.ClusterProjectRoleBinding{})
	assert.True(t, k8serrors.IsNotFound(err), "the CPRB must be gone once the finalizer is removed")
}

// TestReconcile_PrunesStaleRoleBindings removes a binding left in a namespace that is no longer part
// of any project.
func TestReconcile_PrunesStaleRoleBindings(t *testing.T) {
	t.Parallel()
	stale := serviceRoleBinding("global-viewer", "gone", "d8:project:viewer")
	r, c := newReconciler(t, cprb("global-viewer", "d8:project:viewer"), project("alpha", false, "alpha"), stale)

	reconcileCPRB(t, r, "global-viewer")

	name := rolebinding.CPRBServiceName("global-viewer")
	err := c.Get(context.Background(), client.ObjectKey{Namespace: "gone", Name: name}, &rbacv1.RoleBinding{})
	assert.True(t, k8serrors.IsNotFound(err), "the stale binding must be pruned")
	assert.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "alpha", Name: name}, &rbacv1.RoleBinding{}))
}

// TestReconcile_RecreatesOnRoleRefChange verifies the immutable roleRef is handled by delete+recreate.
func TestReconcile_RecreatesOnRoleRefChange(t *testing.T) {
	t.Parallel()
	old := serviceRoleBinding("global-viewer", "alpha", "d8:project:viewer")
	r, c := newReconciler(t, cprb("global-viewer", "d8:project:admin"), project("alpha", false, "alpha"), old)

	reconcileCPRB(t, r, "global-viewer")

	rb := &rbacv1.RoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "alpha", Name: rolebinding.CPRBServiceName("global-viewer")}, rb))
	assert.Equal(t, "d8:project:admin", rb.RoleRef.Name, "roleRef must be updated to the new role")
}

// TestReconcile_ForbiddenRoleIsNotFannedOut verifies defense-in-depth: a disallowed role is cleaned
// up and never propagated, even if it slipped past the webhook.
func TestReconcile_ForbiddenRoleIsNotFannedOut(t *testing.T) {
	t.Parallel()
	r, c := newReconciler(t, cprb("escalation", "cluster-admin"), project("alpha", false, "alpha"))

	reconcileCPRB(t, r, "escalation")

	err := c.Get(context.Background(), client.ObjectKey{Namespace: "alpha", Name: rolebinding.CPRBServiceName("escalation")}, &rbacv1.RoleBinding{})
	assert.True(t, k8serrors.IsNotFound(err), "a forbidden role must not be fanned out")

	got := &v1alpha3.ClusterProjectRoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "escalation"}, got))
	assert.Equal(t, int32(0), got.Status.BoundProjects)
	require.Len(t, got.Status.Conditions, 1)
	assert.Equal(t, corev1.ConditionFalse, got.Status.Conditions[0].Status)
}

// TestReconcile_SkipsDeletionTimestampedProject verifies a terminating project receives no binding
// and is not counted in BoundProjects.
func TestReconcile_SkipsDeletionTimestampedProject(t *testing.T) {
	t.Parallel()
	deleting := project("alpha", false, "alpha")
	deleting.Finalizers = []string{v1alpha3.ProjectFinalizer}
	r, c := newReconciler(t, cprb("global-viewer", "d8:project:viewer"), deleting, project("beta", false, "beta"))

	// deleting the project sets its DeletionTimestamp (the finalizer keeps it listed)
	require.NoError(t, c.Delete(context.Background(), deleting))

	reconcileCPRB(t, r, "global-viewer")

	name := rolebinding.CPRBServiceName("global-viewer")
	err := c.Get(context.Background(), client.ObjectKey{Namespace: "alpha", Name: name}, &rbacv1.RoleBinding{})
	assert.True(t, k8serrors.IsNotFound(err), "a terminating project must not receive the binding")
	assert.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "beta", Name: name}, &rbacv1.RoleBinding{}))

	got := &v1alpha3.ClusterProjectRoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "global-viewer"}, got))
	assert.Equal(t, int32(1), got.Status.BoundProjects, "only the live project is bound")
}

// TestReconcile_StatusUnchangedNoWrite is the unit-level guard for the self-triggered reconcile
// hot-loop fix: a second reconcile that changes nothing must not rewrite status (which the fake
// client would surface as a bumped resourceVersion). The full loop elimination still needs envtest.
func TestReconcile_StatusUnchangedNoWrite(t *testing.T) {
	t.Parallel()
	r, c := newReconciler(t, cprb("global-viewer", "d8:project:viewer"), project("alpha", false, "alpha"))

	reconcileCPRB(t, r, "global-viewer")
	first := &v1alpha3.ClusterProjectRoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "global-viewer"}, first))

	reconcileCPRB(t, r, "global-viewer")
	second := &v1alpha3.ClusterProjectRoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "global-viewer"}, second))

	assert.Equal(t, first.ResourceVersion, second.ResourceVersion,
		"an unchanged reconcile must not rewrite the status and re-enqueue the object")
}
