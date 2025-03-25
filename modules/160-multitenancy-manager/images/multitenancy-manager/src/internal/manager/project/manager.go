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
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/internal/helm"
	"controller/internal/validate"
)

const (
	DeckhouseNamespacePrefix  = "d8-"
	KubernetesNamespacePrefix = "kube-"

	DeckhouseProjectName = "deckhouse"
	DefaultProjectName   = "default"

	VirtualTemplate = "virtual"
)

type Manager struct {
	client     client.Client
	helmClient *helm.Client
	logger     logr.Logger
}

func New(client client.Client, helmClient *helm.Client, logger logr.Logger) *Manager {
	return &Manager{
		client:     client,
		helmClient: helmClient,
		logger:     logger.WithName("project-manager"),
	}
}

func (m *Manager) Init(ctx context.Context, checker healthz.Checker, init *sync.WaitGroup) error {
	m.logger.Info("wait until webhook server start")
	check := func(ctx context.Context) (bool, error) {
		if err := checker(nil); err != nil {
			m.logger.Info("webhook server not startup yet")
			return false, nil
		}
		return true, nil
	}
	if err := wait.PollUntilContextTimeout(ctx, time.Second, 10*time.Second, true, check); err != nil {
		return fmt.Errorf("start webhook server: %w", err)
	}

	m.logger.Info("ensure virtual projects")
	if err := m.ensureVirtualProjects(ctx); err != nil {
		return fmt.Errorf("ensure virtual projects: %w", err)
	}

	m.logger.Info("the virtual projects ensured")
	init.Done()

	return nil
}

// Handle ensures project`s resources
func (m *Manager) Handle(ctx context.Context, project *v1alpha2.Project) (ctrl.Result, error) {
	// add finalizer and remove labels
	if err := m.prepareProject(ctx, project); err != nil {
		m.logger.Error(err, "failed to update the project", "project", project.Name)
		return ctrl.Result{}, err
	}

	project.ClearConditions()
	project.SetObservedGeneration(project.Generation)

	// get the project template for the project
	m.logger.Info("get the project template for project", "project", project.Name, "template", project.Spec.ProjectTemplateName)
	projectTemplate, err := m.projectTemplateByName(ctx, project.Spec.ProjectTemplateName)
	if err != nil {
		m.logger.Error(err, "failed to get project template", "project", project.Name, "template", project.Spec.ProjectTemplateName)
		project.SetState(v1alpha2.ProjectStateError)
		project.SetConditionFalse(v1alpha2.ProjectConditionProjectTemplateFound, err.Error())
		if updateErr := m.updateProjectStatus(ctx, project); updateErr != nil {
			m.logger.Error(updateErr, "failed to update project status", "project", project.Name, "template", project.Spec.ProjectTemplateName)
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, err
	}

	// check if the project template exists
	if projectTemplate == nil {
		m.logger.Info("the project template not found for the project", "project", project.Name, "template", project.Spec.ProjectTemplateName)
		project.SetState(v1alpha2.ProjectStateError)
		project.SetConditionFalse(v1alpha2.ProjectConditionProjectTemplateFound, "The project template not found")
		if updateErr := m.updateProjectStatus(ctx, project); updateErr != nil {
			m.logger.Error(updateErr, "failed to update the project status", "project", project.Name, "template", project.Spec.ProjectTemplateName)
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	project.SetConditionTrue(v1alpha2.ProjectConditionProjectTemplateFound)
	project.SetTemplateGeneration(projectTemplate.Generation)

	// validate the project against the project template
	m.logger.Info("validate the project spec", "project", project.Name, "template", projectTemplate.Name)
	if err = validate.Project(project, projectTemplate); err != nil {
		m.logger.Error(err, "failed to validate the project spec", "project", project.Name, "template", projectTemplate.Name)
		project.SetState(v1alpha2.ProjectStateError)
		project.SetConditionFalse(v1alpha2.ProjectConditionProjectValidated, err.Error())
		if updateErr := m.updateProjectStatus(ctx, project); updateErr != nil {
			m.logger.Error(updateErr, "failed to update the project status", "project", project.Name, "template", projectTemplate.Name)
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	project.SetConditionTrue(v1alpha2.ProjectConditionProjectValidated)

	// upgrade project`s resources
	m.logger.Info("upgrade resources for the project", "project", project.Name, "template", projectTemplate.Name)
	if err = m.helmClient.Upgrade(ctx, project, projectTemplate); err != nil {
		m.logger.Error(err, "failed to upgrade the project resources", "project", project.Name, "template", projectTemplate.Name)
		project.SetState(v1alpha2.ProjectStateError)
		project.SetConditionFalse(v1alpha2.ProjectConditionProjectResourcesUpgraded, err.Error())
		if updateErr := m.updateProjectStatus(ctx, project); updateErr != nil {
			m.logger.Error(updateErr, "failed to update the project status", "project", project.Name, "template", projectTemplate.Name)
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, err
	}

	project.SetState(v1alpha2.ProjectStateDeployed)
	project.SetConditionTrue(v1alpha2.ProjectConditionProjectResourcesUpgraded)
	if err = m.updateProjectStatus(ctx, project); err != nil {
		m.logger.Error(err, "failed to update the project status", "project", project.Name, "template", projectTemplate.Name)
		return ctrl.Result{}, err
	}

	m.logger.Info("the project reconciled", "project", project.Name, "template", projectTemplate.Name)
	return ctrl.Result{}, nil
}

// HandleVirtual handles virtual project
func (m *Manager) HandleVirtual(ctx context.Context, project *v1alpha2.Project) (ctrl.Result, error) {
	namespaces := new(corev1.NamespaceList)
	if err := m.client.List(ctx, namespaces); err != nil {
		m.logger.Error(err, "failed to list namespaces", "project", project.Name)
		return ctrl.Result{}, err
	}

	var involvedNamespaces []string
	for _, namespace := range namespaces.Items {
		if _, ok := namespace.GetLabels()[v1alpha2.ResourceLabelProject]; ok {
			continue
		}

		isDeckhouseNamespace := strings.HasPrefix(namespace.Name, DeckhouseNamespacePrefix) || strings.HasPrefix(namespace.Name, KubernetesNamespacePrefix)

		if project.Name == DeckhouseProjectName && isDeckhouseNamespace {
			involvedNamespaces = append(involvedNamespaces, namespace.Name)
		}

		if project.Name == DefaultProjectName && !isDeckhouseNamespace {
			involvedNamespaces = append(involvedNamespaces, namespace.Name)
		}
	}

	if err := m.updateVirtualProject(ctx, project, involvedNamespaces); err != nil {
		m.logger.Error(err, "failed to update the virtual project", "project", project.Name)
		return ctrl.Result{}, err
	}

	m.logger.Info("the virtual project reconciled", "project", project.Name)
	return ctrl.Result{}, nil
}

// Delete deletes project`s resources
func (m *Manager) Delete(ctx context.Context, project *v1alpha2.Project) (ctrl.Result, error) {
	// delete resources
	if err := m.helmClient.Delete(ctx, project.Name); err != nil {
		// TODO: add error to the project`s status
		m.logger.Error(err, "failed to delete the project", "project", project.Name)
		return ctrl.Result{}, err
	}

	// remove finalizer
	if err := m.removeFinalizer(ctx, project); err != nil {
		m.logger.Error(err, "failed to remove finalizer from the project", "project", project.Name)
		return ctrl.Result{}, err
	}

	m.logger.Info("the project deleted", "project", project.Name)
	return ctrl.Result{}, nil
}
