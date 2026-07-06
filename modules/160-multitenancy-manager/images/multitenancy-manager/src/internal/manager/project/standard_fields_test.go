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

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/apis/deckhouse.io/v1alpha3"
)

func newManager(t *testing.T, objs ...client.Object) (*Manager, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme, rbacv1.AddToScheme, v1alpha3.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return New(c, nil, logr.Discard()), c
}

func TestReconcileQuota(t *testing.T) {
	m, c := newManager(t)
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
	project.Spec.Quota = corev1.ResourceList{
		"requests.cpu":  resource.MustParse("1"),
		"limits.memory": resource.MustParse("15Gi"),
	}

	assert.NoError(t, m.reconcileQuota(context.Background(), project))

	rq := &corev1.ResourceQuota{}
	assert.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: v1alpha3.ProjectQuotaName}, rq))
	assert.Equal(t, project.Spec.Quota, rq.Spec.Hard)
	assert.Equal(t, v1alpha3.ManagedByController, rq.Labels[v1alpha3.ResourceLabelManagedBy])

	// clearing the quota deletes the ResourceQuota
	project.Spec.Quota = nil
	assert.NoError(t, m.reconcileQuota(context.Background(), project))
	err := c.Get(context.Background(), client.ObjectKey{Namespace: "proj", Name: v1alpha3.ProjectQuotaName}, rq)
	assert.True(t, apierrors.IsNotFound(err))
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
