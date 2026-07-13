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
	"cmp"
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"controller/apis/deckhouse.io/v1alpha3"
)

func standardFieldLabels(project string) map[string]string {
	return map[string]string{
		v1alpha3.ResourceLabelHeritage:  v1alpha3.ResourceHeritageMultitenancy,
		v1alpha3.ResourceLabelProject:   project,
		v1alpha3.ResourceLabelManagedBy: v1alpha3.ManagedByController,
	}
}

// ensureNamespace creates (or updates the labels of) the project namespace. It is used for
// template-less projects, where no helm release manages the namespace.
func (m *Manager) ensureNamespace(ctx context.Context, project *v1alpha3.Project) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: project.Name}}
	_, err := controllerutil.CreateOrUpdate(ctx, m.client, ns, func() error {
		if ns.Labels == nil {
			ns.Labels = make(map[string]string)
		}
		ns.Labels[v1alpha3.ResourceLabelHeritage] = v1alpha3.ResourceHeritageMultitenancy
		ns.Labels[v1alpha3.ResourceLabelProject] = project.Name
		ns.Labels[v1alpha3.ResourceLabelTemplate] = project.Spec.ProjectTemplateName
		return nil
	})
	if err != nil {
		return fmt.Errorf("ensure the '%s' namespace: %w", project.Name, err)
	}
	return nil
}

// reconcileStandardFields applies the Project standard fields: the quota (as a ResourceQuota) and
// the administrators (as an auto-managed ProjectRoleBinding).
func (m *Manager) reconcileStandardFields(ctx context.Context, project *v1alpha3.Project) error {
	if err := m.reconcileQuota(ctx, project); err != nil {
		return fmt.Errorf("reconcile quota: %w", err)
	}
	if err := m.reconcileAdministrators(ctx, project); err != nil {
		return fmt.Errorf("reconcile administrators: %w", err)
	}
	return nil
}

func (m *Manager) reconcileQuota(ctx context.Context, project *v1alpha3.Project) error {
	quota := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: v1alpha3.ProjectQuotaName, Namespace: project.Name}}

	if len(project.Spec.Quota) == 0 {
		if err := m.client.Delete(ctx, quota); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete the project quota: %w", err)
		}
		return nil
	}

	_, err := controllerutil.CreateOrUpdate(ctx, m.client, quota, func() error {
		if quota.Labels == nil {
			quota.Labels = make(map[string]string)
		}
		for k, v := range standardFieldLabels(project.Name) {
			quota.Labels[k] = v
		}
		quota.Spec.Hard = project.Spec.Quota.DeepCopy()
		return nil
	})
	if err != nil {
		return fmt.Errorf("upsert the project quota: %w", err)
	}
	return nil
}

func (m *Manager) reconcileAdministrators(ctx context.Context, project *v1alpha3.Project) error {
	binding := &v1alpha3.ProjectRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: v1alpha3.ProjectAdministratorsBinding, Namespace: project.Name}}

	if len(project.Spec.Administrators) == 0 {
		if err := m.client.Delete(ctx, binding); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete the administrators binding: %w", err)
		}
		return nil
	}

	subjects := make([]rbacv1.Subject, 0, len(project.Spec.Administrators))
	for _, admin := range project.Spec.Administrators {
		subjects = append(subjects, rbacv1.Subject{
			APIGroup: rbacv1.GroupName,
			Kind:     admin.Kind,
			Name:     admin.Name,
		})
	}

	_, err := controllerutil.CreateOrUpdate(ctx, m.client, binding, func() error {
		if binding.Labels == nil {
			binding.Labels = make(map[string]string)
		}
		for k, v := range standardFieldLabels(project.Name) {
			binding.Labels[k] = v
		}
		binding.Spec.Subjects = subjects
		binding.Spec.RoleRef = v1alpha3.RoleRef{
			Kind: "ClusterRole",
			Name: v1alpha3.ProjectAdministratorsRoleName,
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("upsert the administrators binding: %w", err)
	}
	return nil
}

// deleteStandardFields removes the controller-managed standard-field objects when the project is
// deleted. The auto-managed administrators ProjectRoleBinding is deleted explicitly so its own
// reconciler can clean up the fanned-out RoleBindings before the namespace disappears.
func (m *Manager) deleteStandardFields(ctx context.Context, project *v1alpha3.Project) error {
	binding := &v1alpha3.ProjectRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: v1alpha3.ProjectAdministratorsBinding, Namespace: project.Name}}
	if err := m.client.Delete(ctx, binding); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete the administrators binding: %w", err)
	}
	return nil
}

// collectNamespaceStatus lists the namespaces that belong to the project and classifies the project
// namespace as Main and the rest as Additional.
func (m *Manager) collectNamespaceStatus(ctx context.Context, project *v1alpha3.Project) ([]v1alpha3.NamespaceStatus, error) {
	list := new(corev1.NamespaceList)
	if err := m.client.List(ctx, list, client.MatchingLabels{v1alpha3.ResourceLabelProject: project.Name}); err != nil {
		return nil, fmt.Errorf("list the project namespaces: %w", err)
	}

	statuses := make([]v1alpha3.NamespaceStatus, 0, len(list.Items))
	for _, ns := range list.Items {
		kind := v1alpha3.NamespaceKindAdditional
		if ns.Name == project.Name {
			kind = v1alpha3.NamespaceKindMain
		}
		statuses = append(statuses, v1alpha3.NamespaceStatus{Name: ns.Name, Kind: kind})
	}

	slices.SortFunc(statuses, func(a, b v1alpha3.NamespaceStatus) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return statuses, nil
}

// collectUsage reads the current quota usage from the controller-managed ResourceQuota.
func (m *Manager) collectUsage(ctx context.Context, project *v1alpha3.Project) (corev1.ResourceList, error) {
	if len(project.Spec.Quota) == 0 {
		return nil, nil
	}

	quota := new(corev1.ResourceQuota)
	if err := m.client.Get(ctx, client.ObjectKey{Namespace: project.Name, Name: v1alpha3.ProjectQuotaName}, quota); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get the project quota: %w", err)
	}

	if len(quota.Status.Used) == 0 {
		return nil, nil
	}
	return quota.Status.Used.DeepCopy(), nil
}
