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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"controller/apis/deckhouse.io/v1alpha3"
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
func (m *Manager) Handle(ctx context.Context, project *v1alpha3.Project) (ctrl.Result, error) {
	// add finalizer and remove labels
	if err := m.prepareProject(ctx, project); err != nil {
		m.logger.Error(err, "failed to update the project", "project", project.Name)
		return ctrl.Result{}, err
	}

	project.ClearConditions()
	project.SetObservedGeneration(project.Generation)

	if project.Spec.ProjectTemplateName == "" {
		// Optional template: only ensure the project namespace, no helm release is created.
		m.logger.Info("the project has no template, ensure namespace only", "project", project.Name)
		if err := m.ensureNamespace(ctx, project); err != nil {
			m.logger.Error(err, "failed to ensure the project namespace", "project", project.Name)
			project.SetState(v1alpha3.ProjectStateError)
			project.SetConditionFalse(v1alpha3.ProjectConditionProjectResourcesUpgraded, err.Error())
			if updateErr := m.updateProjectStatus(ctx, project); updateErr != nil {
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, err
		}
		project.SetConditionTrue(v1alpha3.ProjectConditionProjectTemplateFound)
		project.SetConditionTrue(v1alpha3.ProjectConditionProjectValidated)
		project.SetConditionTrue(v1alpha3.ProjectConditionProjectResourcesUpgraded)
	} else if err, done := m.handleTemplate(ctx, project); done {
		return ctrl.Result{}, err
	}

	// reconcile standard fields (administrators, quota) regardless of the template
	m.logger.Info("reconcile the project standard fields", "project", project.Name)
	if err := m.reconcileStandardFields(ctx, project); err != nil {
		m.logger.Error(err, "failed to reconcile the project standard fields", "project", project.Name)
		project.SetState(v1alpha3.ProjectStateError)
		project.SetConditionFalse(v1alpha3.ProjectConditionStandardFieldsApplied, err.Error())
		if updateErr := m.updateProjectStatus(ctx, project); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, err
	}
	project.SetConditionTrue(v1alpha3.ProjectConditionStandardFieldsApplied)

	// refresh namespaces and quota usage in the status
	if nsStatus, err := m.collectNamespaceStatus(ctx, project); err != nil {
		m.logger.Error(err, "failed to collect the project namespaces", "project", project.Name)
	} else {
		project.Status.Namespaces = nsStatus
	}
	if usage, err := m.collectUsage(ctx, project); err != nil {
		m.logger.Error(err, "failed to collect the project quota usage", "project", project.Name)
	} else {
		project.Status.Usage = usage
	}

	project.SetState(v1alpha3.ProjectStateDeployed)
	if err := m.updateProjectStatus(ctx, project); err != nil {
		m.logger.Error(err, "failed to update the project status", "project", project.Name)
		return ctrl.Result{}, err
	}

	m.logger.Info("the project reconciled", "project", project.Name, "template", project.Spec.ProjectTemplateName)
	return ctrl.Result{}, nil
}

// handleTemplate runs the template-based part of the reconciliation: resolving the template,
// validating the project against it and upgrading the helm release. The bool return value reports
// whether reconciliation must stop (an error already updated the status and the caller should return).
func (m *Manager) handleTemplate(ctx context.Context, project *v1alpha3.Project) (error, bool) {
	m.logger.Info("get the project template for project", "project", project.Name, "template", project.Spec.ProjectTemplateName)
	projectTemplate, err := m.projectTemplateByName(ctx, project.Spec.ProjectTemplateName)
	if err != nil {
		m.logger.Error(err, "failed to get project template", "project", project.Name, "template", project.Spec.ProjectTemplateName)
		project.SetState(v1alpha3.ProjectStateError)
		project.SetConditionFalse(v1alpha3.ProjectConditionProjectTemplateFound, err.Error())
		if updateErr := m.updateProjectStatus(ctx, project); updateErr != nil {
			return updateErr, true
		}
		return err, true
	}

	if projectTemplate == nil {
		m.logger.Info("the project template not found for the project", "project", project.Name, "template", project.Spec.ProjectTemplateName)
		project.SetState(v1alpha3.ProjectStateError)
		project.SetConditionFalse(v1alpha3.ProjectConditionProjectTemplateFound, "The project template not found")
		if updateErr := m.updateProjectStatus(ctx, project); updateErr != nil {
			return updateErr, true
		}
		return nil, true
	}

	project.SetConditionTrue(v1alpha3.ProjectConditionProjectTemplateFound)
	project.SetTemplateGeneration(projectTemplate.Generation)

	m.logger.Info("validate the project spec", "project", project.Name, "template", projectTemplate.Name)
	if err = validate.Project(project, projectTemplate); err != nil {
		m.logger.Error(err, "failed to validate the project spec", "project", project.Name, "template", projectTemplate.Name)
		project.SetState(v1alpha3.ProjectStateError)
		project.SetConditionFalse(v1alpha3.ProjectConditionProjectValidated, err.Error())
		if updateErr := m.updateProjectStatus(ctx, project); updateErr != nil {
			return updateErr, true
		}
		return nil, true
	}

	project.SetConditionTrue(v1alpha3.ProjectConditionProjectValidated)

	m.logger.Info("upgrade resources for the project", "project", project.Name, "template", projectTemplate.Name)
	filtered, err := m.helmClient.Upgrade(ctx, project, projectTemplate)
	if err != nil {
		m.logger.Error(err, "failed to upgrade the project resources", "project", project.Name, "template", projectTemplate.Name)
		project.SetState(v1alpha3.ProjectStateError)
		project.SetConditionFalse(v1alpha3.ProjectConditionProjectResourcesUpgraded, err.Error())
		if updateErr := m.updateProjectStatus(ctx, project); updateErr != nil {
			return updateErr, true
		}
		return err, true
	}

	project.SetConditionTrue(v1alpha3.ProjectConditionProjectResourcesUpgraded)
	if filtered {
		project.SetConditionFalse(v1alpha3.ProjectConditionTemplateResourcesFiltered,
			"The template renders ResourceQuota or AuthorizationRule objects that are now managed via the Project spec.quota/spec.administrators fields; such objects were filtered out.")
	} else {
		project.SetConditionTrue(v1alpha3.ProjectConditionTemplateResourcesFiltered)
	}
	return nil, false
}

// HandleVirtual handles virtual project
func (m *Manager) HandleVirtual(ctx context.Context, project *v1alpha3.Project) (ctrl.Result, error) {
	namespaces := new(corev1.NamespaceList)
	if err := m.client.List(ctx, namespaces); err != nil {
		m.logger.Error(err, "failed to list namespaces", "project", project.Name)
		return ctrl.Result{}, err
	}

	var involvedNamespaces []string
	for _, namespace := range namespaces.Items {
		if _, ok := namespace.GetLabels()[v1alpha3.ResourceLabelProject]; ok {
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
func (m *Manager) Delete(ctx context.Context, project *v1alpha3.Project) (ctrl.Result, error) {
	// delete the auto-managed cluster-scoped standard-field objects (administrators binding)
	if err := m.deleteStandardFields(ctx, project); err != nil {
		m.logger.Error(err, "failed to delete the project standard fields", "project", project.Name)
		return ctrl.Result{}, err
	}

	// delete helm-managed resources
	if err := m.helmClient.Delete(ctx, project.Name); err != nil {
		// TODO: add error to the project`s status
		m.logger.Error(err, "failed to delete the project", "project", project.Name)
		return ctrl.Result{}, err
	}

	// template-less projects own their namespace directly (no helm release deletes it)
	if project.Spec.ProjectTemplateName == "" {
		if err := m.deleteNamespace(ctx, project.Name); err != nil {
			m.logger.Error(err, "failed to delete the project namespace", "project", project.Name)
			return ctrl.Result{}, err
		}
	}

	// remove finalizer
	if err := m.removeFinalizer(ctx, project); err != nil {
		m.logger.Error(err, "failed to remove finalizer from the project", "project", project.Name)
		return ctrl.Result{}, err
	}

	m.logger.Info("the project deleted", "project", project.Name)
	return ctrl.Result{}, nil
}

func (m *Manager) deleteNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{}
	if err := m.client.Get(ctx, client.ObjectKey{Name: name}, ns); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get the '%s' namespace: %w", name, err)
	}
	if _, managed := ns.Labels[v1alpha3.ResourceLabelProject]; !managed {
		return nil
	}
	if err := m.client.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete the '%s' namespace: %w", name, err)
	}
	return nil
}
