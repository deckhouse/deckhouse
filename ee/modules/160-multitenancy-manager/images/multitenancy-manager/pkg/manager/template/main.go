/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template

import (
	"context"
	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/helm"
	"controller/pkg/validate"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Interface interface {
	Handle(ctx context.Context, template *v1alpha1.ProjectTemplate) (ctrl.Result, error)
}
type manager struct {
	log    logr.Logger
	client client.Client
}

func New(client client.Client, log logr.Logger) Interface {
	return &manager{
		client: client,
		log:    log.WithName("template-manager"),
	}
}

func (m *manager) Handle(ctx context.Context, template *v1alpha1.ProjectTemplate) (ctrl.Result, error) {
	if err := validate.ProjectTemplate(template); err != nil {
		template.Status.Message = err.Error()
		if err = m.client.Status().Update(ctx, template); err != nil {
			m.log.Error(err, "failed to update template", "template", template.Name)
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, nil
	}
	projects, err := m.projectsByTemplate(ctx, template)
	if err != nil {
		m.log.Error(err, "failed to get projects for template", "template", template.Name)
		return ctrl.Result{Requeue: true}, err
	}
	if len(projects) == 0 {
		m.log.Info("no projects found for template", "template", template.Name)
		return ctrl.Result{}, nil
	}
	m.log.Info("processing projects for template", "template", template.Name, "projectsNum", len(projects))
	for _, project := range projects {
		m.log.Info("trigger project to update", "template", template.Name, "project", project.Name)
		if project.Annotations == nil {
			project.Annotations = map[string]string{}
		}
		project.Annotations[helm.ProjectRequireSyncAnnotation] = "true"
		if err = m.client.Update(ctx, project); err != nil {
			m.log.Error(err, "failed to update project", "template", template.Name, "project", project.Name)
			return ctrl.Result{Requeue: true}, nil
		}
	}
	template.Status.Ready = true
	if err = m.client.Status().Update(ctx, template); err != nil {
		m.log.Error(err, "failed to update template status", "template", template.Name)
		return ctrl.Result{Requeue: true}, err
	}
	m.log.Info("template reconciled", "template", template.Name)
	return ctrl.Result{}, nil
}

func (m *manager) projectsByTemplate(ctx context.Context, template *v1alpha1.ProjectTemplate) ([]*v1alpha2.Project, error) {
	projects := new(v1alpha2.ProjectList)
	if err := m.client.List(ctx, projects, client.MatchingLabels{helm.ProjectTemplateLabel: template.Name}); err != nil {
		return nil, err
	}
	if len(projects.Items) == 0 {
		return nil, nil
	}
	var result []*v1alpha2.Project
	for _, project := range projects.Items {
		if project.Status.Sync {
			if project.Annotations != nil {
				if _, ok := project.Annotations[helm.ProjectRequireSyncAnnotation]; ok {
					m.log.Info("skipping project due to sync annotation", "project", project.Name)
					continue
				}
			}
			result = append(result, project.DeepCopy())
		}
	}
	return result, nil
}
