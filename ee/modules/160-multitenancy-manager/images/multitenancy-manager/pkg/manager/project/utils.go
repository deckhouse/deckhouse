/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package project

import (
	"context"
	"slices"

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/consts"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (m *Manager) ensureVirtualProjects(ctx context.Context) error {
	deckhouseProject := &v1alpha2.Project{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha2.SchemeGroupVersion.String(),
			Kind:       v1alpha2.ProjectKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: consts.DeckhouseProjectName,
			Labels: map[string]string{
				consts.HeritageLabel:       consts.MultitenancyHeritage,
				consts.ProjectVirtualLabel: "true",
			},
		},
		Spec: v1alpha2.ProjectSpec{
			ProjectTemplateName: consts.VirtualTemplate,
			Description:         "This is a virtual project",
		},
	}
	if err := m.ensureProject(ctx, deckhouseProject); err != nil {
		return err
	}
	othersProject := &v1alpha2.Project{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha2.SchemeGroupVersion.String(),
			Kind:       v1alpha2.ProjectKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: consts.OthersProjectName,
			Labels: map[string]string{
				consts.HeritageLabel:       consts.MultitenancyHeritage,
				consts.ProjectVirtualLabel: "true",
			},
		},
		Spec: v1alpha2.ProjectSpec{
			ProjectTemplateName: consts.VirtualTemplate,
			Description:         "This is a virtual project",
		},
	}
	if err := m.ensureProject(ctx, othersProject); err != nil {
		return err
	}
	return nil
}

func (m *Manager) ensureProject(ctx context.Context, project *v1alpha2.Project) error {
	m.log.Info("ensuring project", "project", project.Name)
	if err := m.client.Create(ctx, project); err != nil {
		if apierrors.IsAlreadyExists(err) {
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				existingProject := new(v1alpha2.Project)
				if err = m.client.Get(ctx, types.NamespacedName{Name: project.Name}, existingProject); err != nil {
					m.log.Error(err, "failed to fetch project")
					return err
				}

				existingProject.Spec = project.Spec
				existingProject.Labels = project.Labels
				existingProject.Annotations = project.Annotations

				m.log.Info("project already exists, try to update it")
				if err = m.client.Update(ctx, existingProject); err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				m.log.Error(err, "failed to update project")
				return err
			}
		} else {
			m.log.Error(err, "failed to create project", "project", project.Name)
			return err
		}
	}
	m.log.Info("successfully ensured project", "project", project.Name)
	return nil
}

func (m *Manager) projectTemplateByName(ctx context.Context, name string) (*v1alpha1.ProjectTemplate, error) {
	template := new(v1alpha1.ProjectTemplate)
	if err := m.client.Get(ctx, types.NamespacedName{Name: name}, template); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return template, nil
}

func (m *Manager) updateProjectStatus(ctx context.Context, project *v1alpha2.Project, state string, templateGeneration int64, condition *v1alpha2.Condition) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.client.Get(ctx, types.NamespacedName{Name: project.Name}, project); err != nil {
			return err
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

func (m *Manager) setFinalizer(ctx context.Context, project *v1alpha2.Project) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.client.Get(ctx, types.NamespacedName{Name: project.Name}, project); err != nil {
			return err
		}
		if !controllerutil.ContainsFinalizer(project, consts.ProjectFinalizer) {
			controllerutil.AddFinalizer(project, consts.ProjectFinalizer)
		}
		return m.client.Update(ctx, project)
	})
}

func (m *Manager) removeFinalizer(ctx context.Context, project *v1alpha2.Project) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.client.Get(ctx, types.NamespacedName{Name: project.Name}, project); err != nil {
			return err
		}
		if !controllerutil.ContainsFinalizer(project, consts.ProjectFinalizer) {
			return nil
		}
		controllerutil.RemoveFinalizer(project, consts.ProjectFinalizer)
		return m.client.Update(ctx, project)
	})
}

func (m *Manager) prepareProject(ctx context.Context, project *v1alpha2.Project) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.client.Get(ctx, types.NamespacedName{Name: project.Name}, project); err != nil {
			return err
		}
		if project.Labels == nil {
			project.Labels = map[string]string{}
		}
		project.Labels[consts.ProjectTemplateLabel] = project.Spec.ProjectTemplateName
		if project.Annotations != nil {
			delete(project.Annotations, consts.ProjectRequireSyncAnnotation)
		}
		return m.client.Update(ctx, project)
	})
}

func (m *Manager) makeCondition(condType, condStatus, condMessage string) *v1alpha2.Condition {
	return &v1alpha2.Condition{
		Type:               condType,
		Status:             condStatus,
		Message:            condMessage,
		LastTransitionTime: metav1.Now(),
	}
}
