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

	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/consts"
	"controller/pkg/helm"
	"controller/pkg/validate"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

type Manager struct {
	client     client.Client
	helmClient *helm.Client
	log        logr.Logger
}

func New(client client.Client, helmClient *helm.Client, log logr.Logger) *Manager {
	return &Manager{
		client:     client,
		helmClient: helmClient,
		log:        log.WithName("project-manager"),
	}
}

func (m *Manager) Init(ctx context.Context, checker healthz.Checker, init *sync.WaitGroup) error {
	m.log.Info("waiting until webhook server start")
	check := func(ctx context.Context) (bool, error) {
		if err := checker(nil); err != nil {
			m.log.Info("webhook server not startup yet")
			return false, nil
		}
		return true, nil
	}
	if err := wait.PollUntilContextTimeout(ctx, time.Second, 10*time.Second, true, check); err != nil {
		return fmt.Errorf("webhook server failed to start: %w", err)
	}
	m.log.Info("webhook server started")

	m.log.Info("ensuring virtual projects")
	if err := m.ensureVirtualProjects(ctx); err != nil {
		return fmt.Errorf("failed to ensure virtual projects: %w", err)
	}

	m.log.Info("ensured virtual projects")
	init.Done()

	return nil
}

// Handle ensures project`s resources
func (m *Manager) Handle(ctx context.Context, project *v1alpha2.Project) (ctrl.Result, error) {
	// set deploying status
	if err := m.updateProjectStatus(ctx, project, v1alpha2.ProjectStateDeploying, 0, nil); err != nil {
		m.log.Error(err, "failed to set project status")
		return ctrl.Result{Requeue: true}, nil
	}

	// set template label, finalizer and delete sync require annotation
	m.log.Info("preparing the project", "project", project.Name, "projectTemplate", project.Spec.ProjectTemplateName)
	if err := m.prepareProject(ctx, project); err != nil {
		m.log.Error(err, "failed to prepare project")
		return ctrl.Result{Requeue: true}, nil
	}

	// get a project template for the project
	m.log.Info("getting project template for project", "project", project.Name, "projectTemplate", project.Spec.ProjectTemplateName)
	projectTemplate, err := m.projectTemplateByName(ctx, project.Spec.ProjectTemplateName)
	if err != nil {
		m.log.Error(err, "failed to get project template", "project", project.Name, "projectTemplate", project.Spec.ProjectTemplateName)
		cond := m.makeCondition(v1alpha2.ConditionTypeProjectTemplateFound, v1alpha2.ConditionTypeFalse, err.Error())
		if statusErr := m.updateProjectStatus(ctx, project, v1alpha2.ProjectStateError, 0, cond); statusErr != nil {
			m.log.Error(statusErr, "failed to set project status", "project", project.Name, "projectTemplate", project.Spec.ProjectTemplateName)
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, nil
	}
	// check if the project template exists
	if projectTemplate == nil {
		m.log.Info("the project template not found for the project", "project", project.Name, "projectTemplate", project.Spec.ProjectTemplateName)
		cond := m.makeCondition(v1alpha2.ConditionTypeProjectTemplateFound, v1alpha2.ConditionTypeFalse, "The project template not found")
		if statusErr := m.updateProjectStatus(ctx, project, v1alpha2.ProjectStateError, 0, cond); statusErr != nil {
			m.log.Error(statusErr, "failed to set the project status", "project", project.Name, "projectTemplate", project.Spec.ProjectTemplateName)
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, nil
	}

	// update conditions
	cond := m.makeCondition(v1alpha2.ConditionTypeProjectTemplateFound, v1alpha2.ConditionTypeTrue, "")
	if statusErr := m.updateProjectStatus(ctx, project, "", projectTemplate.Generation, cond); statusErr != nil {
		m.log.Error(statusErr, "failed to update the project status", "project", project.Name, "projectTemplate", project.Spec.ProjectTemplateName)
		return ctrl.Result{Requeue: true}, nil
	}

	// validate the project against the project template
	m.log.Info("validating the project spec", "project", project.Name, "projectTemplate", projectTemplate.Name)
	if err = validate.Project(project, projectTemplate); err != nil {
		m.log.Error(err, "failed to validate the project spec", "project", project.Name, "projectTemplate", projectTemplate.Name)
		cond = m.makeCondition(v1alpha2.ConditionTypeProjectValidated, v1alpha2.ConditionTypeFalse, err.Error())
		if statusErr := m.updateProjectStatus(ctx, project, v1alpha2.ProjectStateError, projectTemplate.Generation, cond); statusErr != nil {
			m.log.Error(statusErr, "failed to set the project status", "project", project.Name, "projectTemplate", projectTemplate.Name)
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, nil
	}

	// update conditions
	cond = m.makeCondition(v1alpha2.ConditionTypeProjectValidated, v1alpha2.ConditionTypeTrue, "")
	if statusErr := m.updateProjectStatus(ctx, project, "", projectTemplate.Generation, cond); statusErr != nil {
		m.log.Error(statusErr, "failed to update the project status", "project", project.Name, "projectTemplate", project.Spec.ProjectTemplateName)
		return ctrl.Result{Requeue: true}, nil
	}

	// upgrade project`s resources
	m.log.Info("upgrading resources for the project", "project", project.Name, "projectTemplate", projectTemplate.Name)
	if err = m.helmClient.Upgrade(ctx, project, projectTemplate); err != nil {
		// to avoid helm flaky errors
		m.log.Info("failed to upgrade the project resources, try again", "project", project.Name, "projectTemplate", projectTemplate.Name)
		if secondTry := m.helmClient.Upgrade(ctx, project, projectTemplate); secondTry != nil {
			cond = m.makeCondition(v1alpha2.ConditionTypeProjectResourcesUpgraded, v1alpha2.ConditionTypeFalse, err.Error())
			if statusErr := m.updateProjectStatus(ctx, project, v1alpha2.ProjectStateError, projectTemplate.Generation, cond); statusErr != nil {
				m.log.Error(statusErr, "failed to set the project status", "project", project.Name, "projectTemplate", projectTemplate.Name)
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, nil
		}
	}

	// set deployed status
	m.log.Info("setting deployed status for the project", "project", project.Name, "projectTemplate", projectTemplate.Name)
	cond = m.makeCondition(v1alpha2.ConditionTypeProjectResourcesUpgraded, v1alpha2.ConditionTypeTrue, "")
	if err = m.updateProjectStatus(ctx, project, v1alpha2.ProjectStateDeployed, projectTemplate.Generation, cond); err != nil {
		m.log.Error(err, "failed to set the project status", "project", project.Name, "projectTemplate", projectTemplate.Name)
		return ctrl.Result{Requeue: true}, nil
	}

	m.log.Info("the project reconciled", "project", project.Name, "projectTemplate", projectTemplate.Name)
	return ctrl.Result{}, nil
}

// HandleVirtual handles virtual project
func (m *Manager) HandleVirtual(ctx context.Context, project *v1alpha2.Project) (ctrl.Result, error) {
	namespaces := new(corev1.NamespaceList)
	if err := m.client.List(ctx, namespaces); err != nil {
		m.log.Error(err, "failed to list namespaces during reconciling the virtual project", "project", project.Name)
		return ctrl.Result{Requeue: true}, nil
	}
	var involvedNamespaces []string
	for _, namespace := range namespaces.Items {
		if labels := namespace.GetLabels(); labels != nil {
			if _, ok := labels[consts.ProjectTemplateLabel]; ok {
				continue
			}
		}
		isDeckhouseNamespace := strings.HasPrefix(namespace.Name, consts.DeckhouseNamespacePrefix) || strings.HasPrefix(namespace.Name, consts.KubernetesNamespacePrefix)
		if project.Name == consts.DeckhouseProjectName && isDeckhouseNamespace {
			involvedNamespaces = append(involvedNamespaces, namespace.Name)
		}
		if project.Name == consts.DefaultProjectName && !isDeckhouseNamespace {
			involvedNamespaces = append(involvedNamespaces, namespace.Name)
		}
	}
	if err := m.updateVirtualProject(ctx, project, involvedNamespaces); err != nil {
		m.log.Error(err, "failed to update the virtual project", "project", project.Name)
		return ctrl.Result{Requeue: true}, nil
	}
	m.log.Info("the virtual project reconciled", "project", project.Name)
	return ctrl.Result{}, nil
}

// Delete deletes project`s resources
func (m *Manager) Delete(ctx context.Context, project *v1alpha2.Project) (ctrl.Result, error) {
	// delete resources
	if err := m.helmClient.Delete(ctx, project.Name); err != nil {
		// TODO: add error to the project`s status
		m.log.Error(err, "failed to delete the project", "project", project.Name)
		return ctrl.Result{Requeue: true}, nil
	}

	// remove finalizer
	if err := m.removeFinalizer(ctx, project); err != nil {
		m.log.Error(err, "failed to remove finalizer from the project", "project", project.Name)
		return ctrl.Result{Requeue: true}, nil
	}

	m.log.Info("successfully deleted the project", "project", project.Name)
	return ctrl.Result{}, nil
}
