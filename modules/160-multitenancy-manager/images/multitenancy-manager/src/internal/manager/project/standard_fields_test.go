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

package project

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/quota"
	rolebinding "controller/internal/rolebinding"
)

func newManager(t *testing.T, objs ...client.Object) (*Manager, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme, rbacv1.AddToScheme, v1alpha1.AddToScheme, v1alpha2.AddToScheme, v1alpha3.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).
		WithStatusSubresource(&v1alpha3.Project{}, &v1alpha1.ProjectQuota{}).Build()
	return New(c, nil, logr.Discard()), c
}

func TestReconcileQuota(t *testing.T) {
	ctx := context.Background()
	m, c := newManager(t)
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj", UID: "proj-uid"}}
	project.Spec.Quota = corev1.ResourceList{
		"requests.cpu":  resource.MustParse("1"),
		"limits.memory": resource.MustParse("15Gi"),
	}

	requireNoConflict(t, m, project)

	pq := &v1alpha1.ProjectQuota{}
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "proj"}, pq))
	assert.Equal(t, project.Spec.Quota, pq.Spec.Hard)
	assert.Equal(t, v1alpha3.ManagedByController, pq.Labels[v1alpha3.ResourceLabelManagedBy])
	require.Len(t, pq.OwnerReferences, 1)
	assert.Equal(t, types.UID("proj-uid"), pq.OwnerReferences[0].UID)
	assert.True(t, *pq.OwnerReferences[0].Controller)

	// clearing the quota deletes the ProjectQuota made from it
	project.Spec.Quota = nil
	requireNoConflict(t, m, project)
	err := c.Get(ctx, client.ObjectKey{Name: "proj"}, pq)
	assert.True(t, apierrors.IsNotFound(err))
}

// The main namespace of a new project with a quota waits for its d8-project-quota like a namespace made
// for a ProjectNamespace: it is labelled pending until the ProjectQuota controller has the quota
// calculated. A namespace that has the label keeps it, and an older one is not held.
// The main namespace is held by the state of the project, not by its age: a project that has not
// deployed yet is held however long its first passes fail, and a deployed one, which runs workloads,
// is not, an upgrade of the module included.
func TestReconcileQuotaHoldsTheMainNamespace(t *testing.T) {
	ctx := context.Background()
	anHourAgo := metav1.NewTime(time.Now().Add(-time.Hour))
	m, c := newManager(t,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "proj", CreationTimestamp: metav1.Now()}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "failing", CreationTimestamp: anHourAgo}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "done", Labels: map[string]string{quota.NamespaceLabel: quota.LabelEnforced}}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "live", CreationTimestamp: anHourAgo}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "live", Name: "app"}})
	quotaOf := corev1.ResourceList{"pods": resource.MustParse("10")}
	failing := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "failing"}, Spec: v1alpha3.ProjectSpec{Quota: quotaOf}}
	failing.SetState(v1alpha3.ProjectStateError)
	live := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "live"}, Spec: v1alpha3.ProjectSpec{Quota: quotaOf}}
	live.SetState(v1alpha3.ProjectStateDeployed)

	requireNoConflict(t, m, &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}, Spec: v1alpha3.ProjectSpec{Quota: quotaOf}})
	requireNoConflict(t, m, failing)
	requireNoConflict(t, m, &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "done"}, Spec: v1alpha3.ProjectSpec{Quota: quotaOf}})
	requireNoConflict(t, m, &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "none"}})
	requireNoConflict(t, m, live)

	ns := new(corev1.Namespace)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "proj"}, ns))
	assert.Equal(t, quota.LabelPending, ns.Labels[quota.NamespaceLabel])
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "failing"}, ns))
	assert.Equal(t, quota.LabelPending, ns.Labels[quota.NamespaceLabel],
		"a new project whose first passes fail for an hour is still held until its quota is calculated")
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "done"}, ns))
	assert.Equal(t, quota.LabelEnforced, ns.Labels[quota.NamespaceLabel])
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "live"}, ns))
	assert.NotContains(t, ns.Labels, quota.NamespaceLabel,
		"a deployed project with workloads that gets a quota, as on an upgrade, is not held")
}

func TestReconcileQuotaLeavesADirectProjectQuota(t *testing.T) {
	ctx := context.Background()
	direct := &v1alpha1.ProjectQuota{ObjectMeta: metav1.ObjectMeta{Name: "proj"}, Spec: v1alpha1.ProjectQuotaSpec{Hard: corev1.ResourceList{"pods": resource.MustParse("5")}}}
	m, c := newManager(t, direct)
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}

	requireNoConflict(t, m, project)
	assert.NoError(t, c.Get(ctx, client.ObjectKey{Name: "proj"}, &v1alpha1.ProjectQuota{}), "a ProjectQuota created directly stays")
}

// spec.quota does not take a ProjectQuota over: one created directly keeps its limits, its labels and
// its owners, and the project says why spec.quota is not applied.
func TestReconcileQuotaDoesNotTakeADirectProjectQuotaOver(t *testing.T) {
	ctx := context.Background()
	direct := &v1alpha1.ProjectQuota{ObjectMeta: metav1.ObjectMeta{Name: "proj"}, Spec: v1alpha1.ProjectQuotaSpec{Hard: corev1.ResourceList{"pods": resource.MustParse("5")}}}
	m, c := newManager(t, direct)
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj", UID: "proj-uid"}}
	project.Spec.Quota = corev1.ResourceList{"pods": resource.MustParse("50")}

	conflict, err := m.reconcileQuota(ctx, project)
	require.NoError(t, err, "a conflict is reported, not an error, so the rest of the project is reconciled")
	assert.Contains(t, conflict, "'proj' ProjectQuota was created directly")
	assert.Contains(t, conflict, "spec.quota")

	pq := &v1alpha1.ProjectQuota{}
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "proj"}, pq))
	assert.Equal(t, direct.Spec.Hard, pq.Spec.Hard, "its limits stay")
	assert.Empty(t, pq.Labels, "it is not marked as the controller's")
	assert.Empty(t, pq.OwnerReferences, "the project does not own it")

	// the quota and the administrators are applied apart: the one that fails does not hold the other
	project.Spec.Administrators = []v1alpha3.Administrator{{Kind: "User", Name: "alice@example.com"}}
	conflict, err = m.reconcileStandardFields(ctx, project)
	require.NoError(t, err)
	assert.NotEmpty(t, conflict)
	binding := &v1alpha3.ProjectRoleBinding{}
	assert.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "proj", Name: v1alpha3.ProjectAdministratorsBinding}, binding))

	// once it is deleted, spec.quota makes its own
	require.NoError(t, c.Delete(ctx, pq))
	requireNoConflict(t, m, project)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "proj"}, pq))
	assert.Equal(t, project.Spec.Quota, pq.Spec.Hard)
	assert.Equal(t, v1alpha3.ManagedByController, pq.Labels[v1alpha3.ResourceLabelManagedBy])
}

func TestReconcileQuotaDeletesTheQuotaOfAnEarlierRelease(t *testing.T) {
	ctx := context.Background()
	legacy := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Namespace: "proj", Name: v1alpha3.ProjectQuotaName,
		Labels: map[string]string{v1alpha3.ResourceLabelManagedBy: v1alpha3.ManagedByController}}}
	m, c := newManager(t, legacy)

	requireNoConflict(t, m, &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}})
	err := c.Get(ctx, client.ObjectKey{Namespace: "proj", Name: v1alpha3.ProjectQuotaName}, new(corev1.ResourceQuota))
	assert.True(t, apierrors.IsNotFound(err))
}

func TestCollectUsage(t *testing.T) {
	ctx := context.Background()
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}

	m, _ := newManager(t)
	usage, within, err := m.collectUsage(ctx, project)
	require.NoError(t, err)
	assert.Nil(t, usage)
	assert.Nil(t, within, "a project without a quota has no QuotaWithinLimits condition")

	pq := &v1alpha1.ProjectQuota{ObjectMeta: metav1.ObjectMeta{Name: "proj"}, Status: v1alpha1.ProjectQuotaStatus{
		Used: corev1.ResourceList{"pods": resource.MustParse("12")},
		Conditions: []metav1.Condition{{Type: v1alpha1.ProjectQuotaConditionExceeded, Status: metav1.ConditionTrue,
			Reason: v1alpha1.ProjectQuotaReasonHardLowered, Message: "pods: used 12, hard 10"}},
	}}
	m, _ = newManager(t, pq)
	usage, within, err = m.collectUsage(ctx, project)
	require.NoError(t, err)
	assert.Equal(t, pq.Status.Used, usage)
	require.NotNil(t, within)
	assert.Equal(t, corev1.ConditionFalse, within.Status)
	assert.Contains(t, within.Message, "pods: used 12")
}

func TestReconcileAdministrators(t *testing.T) {
	m, c := newManager(t)
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
	project.Spec.Administrators = []v1alpha3.Administrator{{Kind: "User", Name: "alice@example.com"}}

	assert.NoError(t, m.reconcileAdministrators(context.Background(), project))

	prb := &v1alpha3.ProjectRoleBinding{}
	assert.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: v1alpha3.ProjectAdministratorsBinding}, prb))
	assert.Equal(t, "ClusterRole", prb.Spec.RoleRef.Kind)
	assert.Equal(t, v1alpha3.ProjectAdministratorsRoleName, prb.Spec.RoleRef.Name)
	assert.Len(t, prb.Spec.Subjects, 1)
	assert.Equal(t, "alice@example.com", prb.Spec.Subjects[0].Name)
	assert.Equal(t, "User", prb.Spec.Subjects[0].Kind)

	// clearing administrators deletes the binding
	project.Spec.Administrators = nil
	assert.NoError(t, m.reconcileAdministrators(context.Background(), project))
	err := c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: v1alpha3.ProjectAdministratorsBinding}, prb)
	assert.True(t, apierrors.IsNotFound(err))
}

// The upgrade lifts administrators that name the ServiceAccounts of another namespace into
// spec.administrators. The binding the controller writes for them is the one the ProjectRoleBinding
// webhook and the fan-out recognise as the controller's, and it names each administrator the way the
// fan-out matches them, or the administrators would lose the rights.
func TestReconcileAdministrators_TheBindingOfTheController(t *testing.T) {
	m, c := newManager(t)
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
	project.Spec.Administrators = []v1alpha3.Administrator{
		{Kind: "User", Name: "system:serviceaccount:ci:runner"},
		{Kind: "Group", Name: "system:serviceaccounts:ci"},
	}

	require.NoError(t, m.reconcileAdministrators(context.Background(), project))

	prb := &v1alpha3.ProjectRoleBinding{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: v1alpha3.ProjectAdministratorsBinding}, prb))
	assert.True(t, rolebinding.IsAdministratorsBinding(prb.Name, prb.Labels[v1alpha3.ResourceLabelManagedBy]))
	assert.Equal(t, []rbacv1.Subject{
		{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: "system:serviceaccount:ci:runner"},
		{APIGroup: rbacv1.GroupName, Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:ci"},
	}, prb.Spec.Subjects)
}

func TestCollectNamespaceStatus(t *testing.T) {
	main := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "proj", Labels: map[string]string{v1alpha3.ResourceLabelProject: "proj"}}}
	extra := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "proj-extra", Labels: map[string]string{v1alpha3.ResourceLabelProject: "proj"}}}
	other := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "unrelated"}}
	m, _ := newManager(t, main, extra, other)

	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
	statuses, err := m.collectNamespaceStatus(context.Background(), project)
	assert.NoError(t, err)
	assert.Len(t, statuses, 2)
	// sorted by name: proj, proj-extra
	assert.Equal(t, "proj", statuses[0].Name)
	assert.Equal(t, v1alpha3.NamespaceKindMain, statuses[0].Kind)
	assert.Equal(t, "proj-extra", statuses[1].Name)
	assert.Equal(t, v1alpha3.NamespaceKindAdditional, statuses[1].Kind)
}

// requireNoConflict reconciles the quota of the project and expects neither an error nor a conflict.
func requireNoConflict(t *testing.T, m *Manager, project *v1alpha3.Project) {
	t.Helper()
	conflict, err := m.reconcileQuota(context.Background(), project)
	require.NoError(t, err)
	require.Empty(t, conflict)
}
