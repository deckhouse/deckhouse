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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
	rolebinding "controller/internal/rolebinding"
)

func disabledRole(name string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{rolebinding.AnnotationDisabledForProjects: "true"},
		},
	}
}

func conditionByType(project *v1alpha3.Project, condName string) *v1alpha3.Condition {
	for i := range project.Status.Conditions {
		if project.Status.Conditions[i].Type == condName {
			return &project.Status.Conditions[i]
		}
	}
	return nil
}

func TestRoleViolation(t *testing.T) {
	m, _ := newManager(t,
		disabledRole("d8:project:secret-reader"),
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "d8:project:admin"}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "view"}},
	)
	ctx := context.Background()

	// the disabled annotation is a violation regardless of the binding kind
	assert.Contains(t, m.roleViolation(ctx, "d8:project:secret-reader", true), "disabled")
	assert.Contains(t, m.roleViolation(ctx, "d8:project:secret-reader", false), "disabled")

	// an allowed role without the annotation is fine
	assert.Empty(t, m.roleViolation(ctx, "d8:project:admin", true))

	// a role outside the allow-list is a violation only for project bindings
	assert.Contains(t, m.roleViolation(ctx, "view", true), "allowed project role list")
	assert.Empty(t, m.roleViolation(ctx, "view", false))

	// a missing role is not flagged here; existence is enforced by the binding webhook
	assert.Empty(t, m.roleViolation(ctx, "d8:project:ghost", false))
}

func TestApplyTemplateRolesCondition(t *testing.T) {
	m, _ := newManager(t,
		disabledRole("d8:project:secret-reader"),
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "d8:project:admin"}},
	)
	ctx := context.Background()

	t.Run("disabled role flips the condition to false", func(t *testing.T) {
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
		m.applyTemplateRolesCondition(ctx, project, []helm.BindingRoleRef{
			{BindingKind: v1alpha3.ProjectRoleBindingKind, BindingName: "secret", RoleKind: "ClusterRole", RoleName: "d8:project:secret-reader"},
		})
		assert.True(t, project.IsConditionFalse(v1alpha3.ProjectConditionTemplateRolesAllowed))
		cond := conditionByType(project, v1alpha3.ProjectConditionTemplateRolesAllowed)
		assert.NotNil(t, cond)
		assert.Contains(t, cond.Message, "d8:project:secret-reader")
		assert.Contains(t, cond.Message, "disabled")
	})

	t.Run("allowed role keeps the condition true", func(t *testing.T) {
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
		m.applyTemplateRolesCondition(ctx, project, []helm.BindingRoleRef{
			{BindingKind: v1alpha3.ProjectRoleBindingKind, BindingName: "ok", RoleKind: "ClusterRole", RoleName: "d8:project:admin"},
		})
		assert.False(t, project.IsConditionFalse(v1alpha3.ProjectConditionTemplateRolesAllowed))
		cond := conditionByType(project, v1alpha3.ProjectConditionTemplateRolesAllowed)
		assert.NotNil(t, cond)
		assert.Empty(t, cond.Message)
	})

	t.Run("non-clusterrole ref is ignored", func(t *testing.T) {
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
		m.applyTemplateRolesCondition(ctx, project, []helm.BindingRoleRef{
			{BindingKind: "RoleBinding", BindingName: "rb", RoleKind: "Role", RoleName: "anything"},
		})
		assert.False(t, project.IsConditionFalse(v1alpha3.ProjectConditionTemplateRolesAllowed))
	})

	t.Run("native binding to a non-allow-list ClusterRole is allowed", func(t *testing.T) {
		// the allow-list is a PRB/CPRB policy; native RoleBindings may bind any existing role.
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
		m.applyTemplateRolesCondition(ctx, project, []helm.BindingRoleRef{
			{BindingKind: "ClusterRoleBinding", BindingName: "crb", RoleKind: "ClusterRole", RoleName: "cluster-admin"},
		})
		assert.False(t, project.IsConditionFalse(v1alpha3.ProjectConditionTemplateRolesAllowed))
	})

	t.Run("offending bindings are listed deterministically", func(t *testing.T) {
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
		m.applyTemplateRolesCondition(ctx, project, []helm.BindingRoleRef{
			{BindingKind: v1alpha3.ProjectRoleBindingKind, BindingName: "b", RoleKind: "ClusterRole", RoleName: "d8:project:secret-reader"},
			{BindingKind: v1alpha3.ProjectRoleBindingKind, BindingName: "a", RoleKind: "ClusterRole", RoleName: "kube-system-admin"},
		})
		assert.True(t, project.IsConditionFalse(v1alpha3.ProjectConditionTemplateRolesAllowed))
		cond := conditionByType(project, v1alpha3.ProjectConditionTemplateRolesAllowed)
		assert.NotNil(t, cond)
		// "a" (allow-list violation) sorts before "b" (disabled annotation)
		assert.Less(t, strings.Index(cond.Message, `"a"`), strings.Index(cond.Message, `"b"`))
	})
}
