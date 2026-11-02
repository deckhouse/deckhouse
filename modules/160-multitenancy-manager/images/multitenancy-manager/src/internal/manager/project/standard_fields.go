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
	"errors"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/quota"
	rolebinding "controller/internal/rolebinding"
)

func standardFieldLabels(project string) map[string]string {
	return map[string]string{
		v1alpha3.ResourceLabelHeritage:  v1alpha3.ResourceHeritageMultitenancy,
		v1alpha3.ResourceLabelProject:   project,
		v1alpha3.ResourceLabelManagedBy: v1alpha3.ManagedByController,
	}
}

// reconcileStandardFields applies the Project standard fields: the quota (as a ResourceQuota) and
// the administrators (as an auto-managed ProjectRoleBinding).
// The conflict is a state to report, not an error: the rest of the project is still reconciled.
func (m *Manager) reconcileStandardFields(ctx context.Context, project *v1alpha3.Project) (string, error) {
	// a failing quota does not keep the administrators from getting their binding
	var errs []error
	conflict, err := m.reconcileQuota(ctx, project)
	if err != nil {
		errs = append(errs, fmt.Errorf("reconcile quota: %w", err))
	}
	if err := m.reconcileAdministrators(ctx, project); err != nil {
		errs = append(errs, fmt.Errorf("reconcile administrators: %w", err))
	}
	return conflict, errors.Join(errs...)
}

// reconcileQuota keeps the ProjectQuota of the project from spec.quota. The ProjectQuota controller
// puts the d8-project-quota ResourceQuota into every namespace of the project from it. Clearing
// spec.quota deletes the ProjectQuota the controller made from it. A ProjectQuota created directly is
// the field's no more than its owner's: it is neither overwritten nor deleted, and a spec.quota next
// to it is reported.
// reconcileQuota returns, besides an error, the conflict that keeps spec.quota from being applied.
func (m *Manager) reconcileQuota(ctx context.Context, project *v1alpha3.Project) (string, error) {
	pq := &v1alpha1.ProjectQuota{ObjectMeta: metav1.ObjectMeta{Name: project.Name}}

	err := m.client.Get(ctx, client.ObjectKeyFromObject(pq), pq)
	if err != nil && !apierrors.IsNotFound(err) {
		return "", fmt.Errorf("get the project quota: %w", err)
	}
	found := err == nil
	direct := found && pq.Labels[v1alpha3.ResourceLabelManagedBy] != v1alpha3.ManagedByController

	if len(project.Spec.Quota) == 0 {
		switch {
		case !found:
			return "", m.deleteLegacyQuota(ctx, project)
		case direct:
			return "", m.holdMainNamespace(ctx, project)
		}
		if err := m.client.Delete(ctx, pq, client.Preconditions{UID: &pq.UID}); err != nil && !apierrors.IsNotFound(err) {
			return "", fmt.Errorf("delete the project quota: %w", err)
		}
		return "", nil
	}
	if direct {
		return fmt.Sprintf("The '%s' ProjectQuota was created directly, so spec.quota is not applied to it: "+
			"delete that ProjectQuota to let spec.quota make it, or clear spec.quota.", pq.Name), m.holdMainNamespace(ctx, project)
	}

	_, err = controllerutil.CreateOrUpdate(ctx, m.client, pq, func() error {
		if pq.Labels == nil {
			pq.Labels = make(map[string]string)
		}
		for k, v := range standardFieldLabels(project.Name) {
			pq.Labels[k] = v
		}
		pq.Spec.Hard = project.Spec.Quota.DeepCopy()
		return controllerutil.SetControllerReference(project, pq, m.client.Scheme())
	})
	if err != nil {
		return "", fmt.Errorf("upsert the project quota: %w", err)
	}
	return "", m.holdMainNamespace(ctx, project)
}

// holdMainNamespace labels the main namespace of a new project with a quota pending when it has no
// quota label yet: the ProjectQuota controller makes its d8-project-quota on its own time, and until
// that is calculated the quota plugin counts nothing there. The pending guard holds the objects of
// users and the manifests of the template, applied next, until the controller labels the namespace
// enforced, as it does for a namespace made for a ProjectNamespace. A project counts as new until it
// has deployed once, however long its first passes fail. The namespace of a deployed project, which
// runs workloads already, an upgrade of the module or a quota set on a running project included, is
// left as it is: holding it would refuse the pods its controllers create until the quota is calculated.
func (m *Manager) holdMainNamespace(ctx context.Context, project *v1alpha3.Project) error {
	if project.Status.State == v1alpha3.ProjectStateDeployed {
		return nil
	}
	ns := new(corev1.Namespace)
	switch err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, ns); {
	case apierrors.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("get the main namespace: %w", err)
	}
	if _, labelled := ns.Labels[quota.NamespaceLabel]; labelled || !ns.DeletionTimestamp.IsZero() {
		return nil
	}
	patch := client.MergeFromWithOptions(ns.DeepCopy(), client.MergeFromWithOptimisticLock{})
	if ns.Labels == nil {
		ns.Labels = map[string]string{}
	}
	ns.Labels[quota.NamespaceLabel] = quota.LabelPending
	if err := m.client.Patch(ctx, ns, patch); err != nil {
		return fmt.Errorf("label the main namespace pending: %w", err)
	}
	return nil
}

// deleteLegacyQuota deletes the d8-project-quota ResourceQuota that an earlier release of the
// controller made in the main namespace from spec.quota, when the project has no quota any more. A
// ResourceQuota that a ProjectQuota owns goes with the ProjectQuota.
func (m *Manager) deleteLegacyQuota(ctx context.Context, project *v1alpha3.Project) error {
	rq := new(corev1.ResourceQuota)
	switch err := m.client.Get(ctx, client.ObjectKey{Namespace: project.Name, Name: v1alpha3.ProjectQuotaName}, rq); {
	case apierrors.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("get the project quota: %w", err)
	}
	if rq.Labels[v1alpha3.ResourceLabelManagedBy] != v1alpha3.ManagedByController || len(rq.OwnerReferences) > 0 {
		return nil
	}
	if err := m.client.Delete(ctx, rq, client.Preconditions{UID: &rq.UID}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete the project quota: %w", err)
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

	// The webhook and the fan-out tell this binding by its name and managed-by label (see
	// rolebinding.IsAdministratorsBinding), and the fan-out finds the administrators among its subjects
	// by rolebinding.AdministratorSubject, so that it carries the ones naming ServiceAccounts of any
	// namespace.
	subjects := make([]rbacv1.Subject, 0, len(project.Spec.Administrators))
	for _, admin := range project.Spec.Administrators {
		subjects = append(subjects, rolebinding.AdministratorSubject(admin))
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

// collectUsage reads the usage of the project from its ProjectQuota and says whether the project is
// within the quota: nil when the project has no ProjectQuota.
func (m *Manager) collectUsage(ctx context.Context, project *v1alpha3.Project) (corev1.ResourceList, *v1alpha3.Condition, error) {
	pq := new(v1alpha1.ProjectQuota)
	if err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, pq); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("get the project quota: %w", err)
	}

	within := &v1alpha3.Condition{Type: v1alpha3.ProjectConditionQuotaWithinLimits, Status: corev1.ConditionTrue}
	if exceeded := meta.FindStatusCondition(pq.Status.Conditions, v1alpha1.ProjectQuotaConditionExceeded); exceeded != nil && exceeded.Status == metav1.ConditionTrue {
		within.Status = corev1.ConditionFalse
		within.Message = exceeded.Message
	}
	if len(pq.Status.Used) == 0 {
		return nil, within, nil
	}
	return pq.Status.Used.DeepCopy(), within, nil
}
