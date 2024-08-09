/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package project

import (
	"context"
	"sync"

	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/helm"
	"controller/pkg/validate"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	Finalizer = "projects.deckhouse.io/project-exists"
)

type Interface interface {
	Init(ctx context.Context) error
	Handle(ctx context.Context, project *v1alpha2.Project) (ctrl.Result, error)
	Delete(ctx context.Context, project *v1alpha2.Project) (ctrl.Result, error)
}
type manager struct {
	init        sync.WaitGroup
	log         logr.Logger
	client      client.Client
	helmClient  helm.Interface
	defaultPath string
}

func New(client client.Client, helmClient helm.Interface, log logr.Logger, defaultPath string) Interface {
	return &manager{
		init:        sync.WaitGroup{},
		log:         log.WithName("project-manager"),
		client:      client,
		helmClient:  helmClient,
		defaultPath: defaultPath,
	}
}

func (m *manager) Init(ctx context.Context) error {
	m.init.Add(1)
	defer m.init.Done()

	m.log.Info("ensuring default project templates")
	if err := m.ensureDefaultProjectTemplates(ctx, m.defaultPath); err != nil {
		m.log.Error(err, "failed to ensure default project templates")
		return err
	}
	m.log.Info("ensured default project templates")

	return nil
}

// Handle ensures project`s resources
func (m *manager) Handle(ctx context.Context, project *v1alpha2.Project) (ctrl.Result, error) {
	m.init.Wait()

	// get a project template for the project
	m.log.Info("getting project template for project", "project", project.Name)
	projectTemplate, err := m.projectTemplateByName(ctx, project.Spec.ProjectTemplateName)
	if err != nil {
		m.log.Error(err, "failed to get project template", "project", project.Name)
		if statusErr := m.setProjectStatus(ctx, project, v1alpha2.ProjectStateGettingTemplateError, err.Error()); statusErr != nil {
			m.log.Error(statusErr, "failed to set project status", "project", project.Name)
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	// check if the project template exists
	if projectTemplate == nil {
		m.log.Info("project template not found for project", "project", project.Name)
		if statusErr := m.setProjectStatus(ctx, project, v1alpha2.ProjectStateTemplateNotFound, "The project template not found"); statusErr != nil {
			m.log.Error(statusErr, "failed to set project status", "project", project.Name)
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	// validate the project against the project template
	m.log.Info("validating project spec", "project", project.Name, "projectTemplate", projectTemplate.Name)
	if err = validate.Project(project, projectTemplate); err != nil {
		m.log.Error(err, "failed to validate project spec", "project", project.Name, "projectTemplate", projectTemplate.Name)
		if statusErr := m.setProjectStatus(ctx, project, v1alpha2.ProjectStateValidationError, err.Error()); statusErr != nil {
			m.log.Error(statusErr, "failed to set project status", "project", project.Name, "projectTemplate", projectTemplate.Name)
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	// upgrade project`s resources
	m.log.Info("upgrading resources for project", "project", project.Name, "projectTemplate", projectTemplate.Name)
	upgradeOpts := &helm.UpgradeOptions{
		ReleaseName:      project.Name,
		ReleaseNamespace: project.Name,
		ProjectName:      project.Name,
		ProjectTemplate:  project.Spec.ProjectTemplateName,
		Values:           valuesFromProjectAndTemplate(project, projectTemplate),
	}
	if err = m.helmClient.Upgrade(ctx, upgradeOpts); err != nil {
		m.log.Error(err, "failed to upgrade resources for project", "project", project.Name, "projectTemplate", projectTemplate.Name)
		if statusErr := m.setProjectStatus(ctx, project, v1alpha2.ProjectStateUpgradeError, err.Error()); statusErr != nil {
			m.log.Error(statusErr, "failed to set project status", "project", project.Name, "projectTemplate", projectTemplate.Name)
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	// set deployed status
	m.log.Info("setting deployed status for project", "project", project.Name, "projectTemplate", projectTemplate.Name)
	if statusErr := m.setProjectStatus(ctx, project, v1alpha2.ProjectStateDeployed, "The project is ensured"); statusErr != nil {
		m.log.Error(statusErr, "failed to set project status", "project", project.Name, "projectTemplate", projectTemplate.Name)
		return ctrl.Result{Requeue: true}, nil
	}

	// set finalizer
	if project.Labels == nil {
		project.Labels = make(map[string]string)
	}
	project.Labels[helm.ProjectTemplateLabel] = project.Spec.ProjectTemplateName
	if project.Annotations != nil {
		delete(project.Annotations, helm.ProjectRequireSyncAnnotation)
	}
	if !controllerutil.ContainsFinalizer(project, Finalizer) {
		controllerutil.AddFinalizer(project, Finalizer)
	}
	if err = m.client.Update(ctx, project); err != nil {
		m.log.Error(err, "failed to update project", "project", project.Name)
		return ctrl.Result{Requeue: true}, nil
	}
	m.log.Info("project reconciled", "project", project.Name, "projectTemplate", projectTemplate.Name)
	return ctrl.Result{}, nil
}

// Delete deletes project`s resources
func (m *manager) Delete(ctx context.Context, project *v1alpha2.Project) (ctrl.Result, error) {
	// delete resources
	if err := m.helmClient.Delete(ctx, project.Name); err != nil {
		m.log.Error(err, "failed to delete project", "project", project.Name)
		return ctrl.Result{Requeue: true}, err
	}

	// remove finalizer
	if controllerutil.ContainsFinalizer(project, Finalizer) {
		controllerutil.RemoveFinalizer(project, Finalizer)
		if err := m.client.Update(ctx, project); err != nil {
			m.log.Error(err, "failed to remove finalizer", "project", project.Name)
			return ctrl.Result{Requeue: true}, err
		}
	}
	m.log.Info("successfully deleted project", "project", project.Name)
	return ctrl.Result{}, nil
}
