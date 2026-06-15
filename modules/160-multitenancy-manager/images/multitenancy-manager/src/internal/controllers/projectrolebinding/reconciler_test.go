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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/rolebinding"
)

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
		WithStatusSubresource(&v1alpha3.ProjectRoleBinding{}).
		Build()
	return &Reconciler{Client: c}, c
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
