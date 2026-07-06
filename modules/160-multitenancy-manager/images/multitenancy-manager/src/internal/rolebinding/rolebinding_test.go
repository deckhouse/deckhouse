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

package rolebinding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/apis/deckhouse.io/v1alpha3"
)

func newClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{rbacv1.AddToScheme, v1alpha3.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func upsertParams(name, namespace, role string) UpsertParams {
	return UpsertParams{
		Name:        PRBServiceName(name),
		Namespace:   namespace,
		Project:     "proj",
		OwnerLabel:  v1alpha3.ResourceLabelOwnedByPRB,
		OwnerName:   name,
		RelatedWith: "proj/" + name,
		Subjects:    []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: "User", Name: "alice"}},
		RoleRef:     role,
	}
}

func TestServiceNames(t *testing.T) {
	assert.Equal(t, "d8:prb:viewers", PRBServiceName("viewers"))
	assert.Equal(t, "d8:cprb:admins", CPRBServiceName("admins"))
}

func TestIsRoleAllowed(t *testing.T) {
	allowed := []string{
		"d8:project:viewer",
		"d8:namespace:admin",
		"d8:project-capability:manage-rbac",
		"d8:namespace-capability:view",
		"d8:custom:my-role",
	}
	for _, name := range allowed {
		assert.Truef(t, IsRoleAllowed(name), "expected %q to be allowed", name)
	}

	denied := []string{
		"cluster-admin",
		"d8:system:masters",
		"d8:user-authz:admin",
		"admin",
		"",
	}
	for _, name := range denied {
		assert.Falsef(t, IsRoleAllowed(name), "expected %q to be denied", name)
	}
}

func TestProjectNamespaceNames(t *testing.T) {
	// no status: only the main namespace
	p := &v1alpha3.Project{}
	p.Name = "foo"
	assert.Equal(t, []string{"foo"}, ProjectNamespaceNames(p))

	// status without the main namespace: it is appended
	p.Status.Namespaces = []v1alpha3.NamespaceStatus{{Name: "foo-extra"}}
	assert.ElementsMatch(t, []string{"foo-extra", "foo"}, ProjectNamespaceNames(p))

	// status with the main namespace: not duplicated
	p.Status.Namespaces = []v1alpha3.NamespaceStatus{{Name: "foo"}, {Name: "foo-extra"}}
	got := ProjectNamespaceNames(p)
	assert.ElementsMatch(t, []string{"foo", "foo-extra"}, got)
	assert.Len(t, got, 2)
}

func TestCopySubjects(t *testing.T) {
	assert.Nil(t, CopySubjects(nil))

	in := []rbacv1.Subject{{Kind: "User", Name: "alice"}}
	out := CopySubjects(in)
	assert.Equal(t, in, out)

	// mutating the copy must not affect the source
	out[0].Name = "bob"
	assert.Equal(t, "alice", in[0].Name)
}

func TestUpsertServiceRoleBinding_Create(t *testing.T) {
	c := newClient(t)
	require.NoError(t, UpsertServiceRoleBinding(context.Background(), c, upsertParams("viewers", "proj", "d8:project:viewer"), nil))

	rb := &rbacv1.RoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: PRBServiceName("viewers")}, rb))
	assert.Equal(t, "d8:project:viewer", rb.RoleRef.Name)
	assert.Equal(t, "ClusterRole", rb.RoleRef.Kind)
	assert.Equal(t, "viewers", rb.Labels[v1alpha3.ResourceLabelOwnedByPRB])
	assert.Equal(t, "proj", rb.Labels[v1alpha3.ResourceLabelProject])
	assert.Equal(t, v1alpha3.ResourceHeritageMultitenancy, rb.Labels[v1alpha3.ResourceLabelHeritage])
	assert.Equal(t, "proj/viewers", rb.Annotations[v1alpha3.ResourceAnnotationRelatedWith])
	require.Len(t, rb.Subjects, 1)
	assert.Equal(t, "alice", rb.Subjects[0].Name)
}

func TestUpsertServiceRoleBinding_RecreatesOnRoleRefChange(t *testing.T) {
	existing := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: PRBServiceName("viewers"), Namespace: "proj"},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "d8:project:viewer"},
	}
	c := newClient(t, existing)

	require.NoError(t, UpsertServiceRoleBinding(context.Background(), c, upsertParams("viewers", "proj", "d8:project:admin"), nil))

	rb := &rbacv1.RoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: PRBServiceName("viewers")}, rb))
	assert.Equal(t, "d8:project:admin", rb.RoleRef.Name, "an immutable roleRef change must recreate the binding")
}

func TestUpsertServiceRoleBinding_UpdatesSubjectsInPlace(t *testing.T) {
	c := newClient(t)
	require.NoError(t, UpsertServiceRoleBinding(context.Background(), c, upsertParams("viewers", "proj", "d8:project:viewer"), nil))

	p := upsertParams("viewers", "proj", "d8:project:viewer")
	p.Subjects = []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: "User", Name: "bob"}}
	require.NoError(t, UpsertServiceRoleBinding(context.Background(), c, p, nil))

	rb := &rbacv1.RoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: PRBServiceName("viewers")}, rb))
	require.Len(t, rb.Subjects, 1)
	assert.Equal(t, "bob", rb.Subjects[0].Name)
}

func TestPruneServiceRoleBindings(t *testing.T) {
	labels := func(project string) map[string]string {
		return map[string]string{v1alpha3.ResourceLabelOwnedByPRB: "viewers", v1alpha3.ResourceLabelProject: project}
	}
	keep := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: PRBServiceName("viewers"), Namespace: "proj", Labels: labels("proj")}}
	stale := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: PRBServiceName("viewers"), Namespace: "gone", Labels: labels("proj")}}
	foreign := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: PRBServiceName("viewers"), Namespace: "other", Labels: labels("different")}}
	c := newClient(t, keep, stale, foreign)

	selector := map[string]string{v1alpha3.ResourceLabelOwnedByPRB: "viewers", v1alpha3.ResourceLabelProject: "proj"}
	require.NoError(t, PruneServiceRoleBindings(context.Background(), c, selector, map[string]struct{}{"proj": {}}))

	assert.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: PRBServiceName("viewers")}, &rbacv1.RoleBinding{}),
		"a binding in a target namespace must be kept")
	assert.True(t, k8serrors.IsNotFound(c.Get(context.Background(), client.ObjectKey{Namespace: "gone", Name: PRBServiceName("viewers")}, &rbacv1.RoleBinding{})),
		"a binding outside the target namespaces must be pruned")
	assert.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "other", Name: PRBServiceName("viewers")}, &rbacv1.RoleBinding{}),
		"the selector is project-scoped: a binding of another project must survive")
}

func TestProjectFanoutChanged(t *testing.T) {
	base := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "p"}}
	base.Status.Namespaces = []v1alpha3.NamespaceStatus{{Name: "p", Kind: v1alpha3.NamespaceKindMain}}

	assert.False(t, ProjectFanoutChanged(base, base.DeepCopy()), "identical projects must not re-enqueue")

	// only status churn (conditions / observedGeneration / state) must be ignored
	churn := base.DeepCopy()
	churn.Status.ObservedGeneration = 7
	churn.Status.State = v1alpha3.ProjectStateDeployed
	churn.Status.Conditions = []v1alpha3.Condition{{Type: "Ready", Status: "True"}}
	assert.False(t, ProjectFanoutChanged(base, churn), "a project status write must not re-enqueue bindings")

	added := base.DeepCopy()
	added.Status.Namespaces = append(added.Status.Namespaces, v1alpha3.NamespaceStatus{Name: "p-extra", Kind: v1alpha3.NamespaceKindAdditional})
	assert.True(t, ProjectFanoutChanged(base, added), "a namespace-set change must re-enqueue bindings")

	virtual := base.DeepCopy()
	virtual.Labels = map[string]string{v1alpha3.ProjectLabelVirtualProject: "true"}
	assert.True(t, ProjectFanoutChanged(base, virtual), "a virtual-label change must re-enqueue bindings")

	deleting := base.DeepCopy()
	now := metav1.Now()
	deleting.DeletionTimestamp = &now
	assert.True(t, ProjectFanoutChanged(base, deleting), "a deletion-state change must re-enqueue bindings")
}
