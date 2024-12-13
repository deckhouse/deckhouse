/*
Copyright 2024 Flant JSC

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
	"fmt"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
)

func (m *Manager) updateVirtualProject(ctx context.Context, project *v1alpha2.Project, namespaces []string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, project); err != nil {
			return fmt.Errorf("get the '%s' project: %w", project.Name, err)
		}
		project.Status.Conditions = nil
		project.Status.Namespaces = namespaces
		project.Status.TemplateGeneration = 1
		project.Status.ObservedGeneration = project.Generation
		project.Status.State = v1alpha2.ProjectStateDeployed
		return m.client.Status().Update(ctx, project)
	})
}

func (m *Manager) ensureVirtualProjects(ctx context.Context) error {
	deckhouseProject := &v1alpha2.Project{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha2.SchemeGroupVersion.String(),
			Kind:       v1alpha2.ProjectKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: DeckhouseProjectName,
			Labels: map[string]string{
				v1alpha2.ResourceLabelHeritage:      v1alpha2.ResourceHeritageDeckhouse,
				v1alpha2.ProjectLabelVirtualProject: "true",
			},
		},
		Spec: v1alpha2.ProjectSpec{
			ProjectTemplateName: VirtualTemplate,
			Description:         "This is a virtual project",
		},
	}

	if err := m.ensureProject(ctx, deckhouseProject); err != nil {
		return err
	}

	defaultProject := &v1alpha2.Project{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha2.SchemeGroupVersion.String(),
			Kind:       v1alpha2.ProjectKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultProjectName,
			Labels: map[string]string{
				v1alpha2.ResourceLabelHeritage:      v1alpha2.ResourceHeritageDeckhouse,
				v1alpha2.ProjectLabelVirtualProject: "true",
			},
		},
		Spec: v1alpha2.ProjectSpec{
			ProjectTemplateName: VirtualTemplate,
			Description:         "This is a virtual project",
		},
	}

	return m.ensureProject(ctx, defaultProject)
}

func (m *Manager) ensureProject(ctx context.Context, project *v1alpha2.Project) error {
	m.log.Info("ensuring the project", "project", project.Name)
	if err := m.client.Create(ctx, project); err != nil {
		if apierrors.IsAlreadyExists(err) {
			m.log.Info("the project already exists, try to update it")
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				existingProject := new(v1alpha2.Project)
				if err = m.client.Get(ctx, client.ObjectKey{Name: project.Name}, existingProject); err != nil {
					return fmt.Errorf("get the '%s' project: %w", project.Name, err)
				}

				existingProject.Spec = project.Spec
				existingProject.Labels = project.Labels
				existingProject.Annotations = project.Annotations

				return m.client.Update(ctx, existingProject)
			})
			if err != nil {
				return fmt.Errorf("update the '%s' project: %w", project.Name, err)
			}
		} else {
			return fmt.Errorf("create the '%s' project: %w", project.Name, err)
		}
	}

	m.log.Info("successfully ensured the project", "project", project.Name)
	return nil
}

func (m *Manager) projectTemplateByName(ctx context.Context, name string) (*v1alpha1.ProjectTemplate, error) {
	template := new(v1alpha1.ProjectTemplate)
	if err := m.client.Get(ctx, client.ObjectKey{Name: name}, template); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get the '%s' project template: %w", name, err)
	}
	return template, nil
}

func (m *Manager) updateProjectStatus(ctx context.Context, project *v1alpha2.Project, state string, templateGeneration int64, condition *v1alpha2.Condition) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, project); err != nil {
			return fmt.Errorf("get the '%s' project: %w", project.Name, err)
		}

		if project.Status.State != state && state != "" {
			project.Status.State = state
			if state == v1alpha2.ProjectStateDeploying {
				// clear conditions before reconcile
				project.Status.Conditions = []v1alpha2.Condition{}
			}
		}

		if project.Status.Namespaces == nil {
			project.Status.Namespaces = []string{}
		}

		if !slices.Contains(project.Status.Namespaces, project.Name) {
			project.Status.Namespaces = append(project.Status.Namespaces, project.Name)
		}

		if project.Status.ObservedGeneration != project.Generation {
			project.Status.ObservedGeneration = project.Generation
		}

		if templateGeneration != 0 && project.Status.TemplateGeneration != templateGeneration {
			project.Status.TemplateGeneration = templateGeneration
		}

		if condition != nil {
			project.Status.Conditions = append(project.Status.Conditions, *condition)
		}

		return m.client.Status().Update(ctx, project)
	})
}

func (m *Manager) removeFinalizer(ctx context.Context, project *v1alpha2.Project) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, project); err != nil {
			return fmt.Errorf("get the '%s' project: %w", project.Name, err)
		}
		if !controllerutil.ContainsFinalizer(project, v1alpha2.ProjectFinalizer) {
			return nil
		}
		controllerutil.RemoveFinalizer(project, v1alpha2.ProjectFinalizer)
		return m.client.Update(ctx, project)
	})
}

func (m *Manager) prepareProject(ctx context.Context, project *v1alpha2.Project) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, project); err != nil {
			return fmt.Errorf("get the '%s' project: %w", project.Name, err)
		}
		if len(project.Labels) == 0 {
			project.Labels = make(map[string]string, 1)
		}
		project.Labels[v1alpha2.ResourceLabelTemplate] = project.Spec.ProjectTemplateName
		if project.Annotations != nil {
			delete(project.Annotations, v1alpha2.ProjectAnnotationRequireSync)
		}
		if !controllerutil.ContainsFinalizer(project, v1alpha2.ProjectFinalizer) {
			controllerutil.AddFinalizer(project, v1alpha2.ProjectFinalizer)
		}
		return m.client.Update(ctx, project)
	})
}
