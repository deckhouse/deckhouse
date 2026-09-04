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

package namespace

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
)

// IsLeftoverWrap reports a project the previous model auto-created around a
// namespace. Those still carry project-managed-by-namespace until migration
// finishes; handmade empty-template projects do not.
func IsLeftoverWrap(project *v1alpha3.Project) bool {
	if project == nil {
		return false
	}
	return project.Labels[v1alpha3.ProjectLabelManagedByNamespace] == v1alpha3.ManagedByNamespace
}

// CompleteLeftover finishes a leftover wrap project. A live namespace is
// inferred into a real template. A terminating or missing namespace means the
// user already deleted it: the leftover Project is deleted so Helm never
// recreates the namespace. The bool is true when the Project was marked for
// deletion (or was already deleting).
func (m *Manager) CompleteLeftover(ctx context.Context, project *v1alpha3.Project) (bool, error) {
	if !IsLeftoverWrap(project) {
		return false, nil
	}

	namespace := new(corev1.Namespace)
	err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, namespace)
	switch {
	case apierrors.IsNotFound(err):
		if err := m.deleteLeftoverProject(ctx, project); err != nil {
			return false, err
		}
		return true, nil
	case err != nil:
		return false, fmt.Errorf("get the '%s' namespace: %w", project.Name, err)
	}

	if !namespace.DeletionTimestamp.IsZero() {
		if err := m.persistNamespace(ctx, namespace, applyClearRetiredMarkers); err != nil {
			return false, err
		}
		if err := m.deleteLeftoverProject(ctx, project); err != nil {
			return false, err
		}
		return true, nil
	}

	// Do not infer a template while a foreign Helm release still owns the
	// namespace. Handle will surface HelmOwnership; once the annotations are
	// gone this leftover can be migrated for real. The same holdoff as Adopt
	// applies: a brand-new leftover wrap must not steal Helm annotations that
	// arrive a moment after create.
	if helm.ForeignRelease(namespace, helm.ReleaseName(project.Name)) != "" {
		return false, nil
	}
	if helm.StampHoldoff(namespace) > 0 {
		return false, nil
	}

	if err := m.persistNamespace(ctx, namespace, func(ns *corev1.Namespace) bool {
		stamped := helm.ApplyReleaseOwnership(ns, helm.ReleaseName(project.Name))
		cleared := applyClearRetiredMarkers(ns)
		return stamped || cleared
	}); err != nil {
		return false, err
	}

	if err := m.applyInferredTemplate(ctx, project, namespace); err != nil {
		return false, err
	}
	return false, nil
}

func (m *Manager) deleteLeftoverProject(ctx context.Context, project *v1alpha3.Project) error {
	if project == nil || !project.DeletionTimestamp.IsZero() {
		return nil
	}
	if err := m.client.Delete(ctx, project); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete the leftover '%s' project: %w", project.Name, err)
	}
	return nil
}

func (m *Manager) applyInferredTemplate(ctx context.Context, project *v1alpha3.Project, namespace *corev1.Namespace) error {
	template := TemplateFor(namespace)
	parameters := ParametersFor(namespace, template)

	m.logger.Info("migrate the project to a template", "project", project.Name, "template", template)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := new(v1alpha3.Project)
		if err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, current); err != nil {
			return fmt.Errorf("get the '%s' project: %w", project.Name, err)
		}
		if !needsTemplate(current) {
			return nil
		}
		current.Spec.ProjectTemplateName = template
		// Seed parameters for leftover namespace-managed projects and for empty specs so the
		// first Helm render stays a no-op (NotRestricted / existing PSS). Do not wipe a
		// hand-made project that already carries parameters.
		overwriteParams := IsLeftoverWrap(current) || len(current.Spec.Parameters) == 0
		if overwriteParams && len(parameters) > 0 {
			current.Spec.Parameters = parameters
		}
		delete(current.Labels, v1alpha3.ProjectLabelManagedByNamespace)
		return m.client.Update(ctx, current)
	}); err != nil {
		return fmt.Errorf("update the '%s' project: %w", project.Name, err)
	}
	return nil
}
