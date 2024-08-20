/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package project

import (
	"context"

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/consts"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (m *manager) projectTemplateByName(ctx context.Context, name string) (*v1alpha1.ProjectTemplate, error) {
	template := new(v1alpha1.ProjectTemplate)
	if err := m.client.Get(ctx, types.NamespacedName{Name: name}, template); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return template, nil
}

func (m *manager) updateProjectStatus(ctx context.Context, project *v1alpha2.Project, state string, templateGeneration int64, condition *v1alpha2.Condition) error {
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

func (m *manager) setFinalizer(ctx context.Context, project *v1alpha2.Project) error {
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

func (m *manager) removeFinalizer(ctx context.Context, project *v1alpha2.Project) error {
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

func (m *manager) prepareProject(ctx context.Context, project *v1alpha2.Project) error {
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

func (m *manager) makeCondition(condType, condStatus, condMessage string) *v1alpha2.Condition {
	return &v1alpha2.Condition{
		Type:               condType,
		Status:             condStatus,
		Message:            condMessage,
		LastTransitionTime: metav1.Now(),
	}
}
