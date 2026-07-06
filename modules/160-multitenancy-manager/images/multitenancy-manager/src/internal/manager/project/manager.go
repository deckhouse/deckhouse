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
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
	"controller/internal/render"
	rolebinding "controller/internal/rolebinding"
	"controller/internal/validate"
)

// helmClient is the subset of *helm.Client that the project manager depends on. Depending on the
// interface rather than the concrete client lets Handle/upgradeResources be unit-tested with a fake
// (the concrete *helm.Client satisfies it).
type helmClient interface {
	Upgrade(ctx context.Context, project *v1alpha3.Project, template *v1alpha1.ProjectTemplate) (helm.ReleaseOutcome, error)
	UpgradeManifests(ctx context.Context, project *v1alpha3.Project, manifests string) (helm.ReleaseOutcome, error)
	AnalyzeRendered(project *v1alpha3.Project, template *v1alpha1.ProjectTemplate) (helm.ReleaseOutcome, error)
	AnalyzeManifests(project *v1alpha3.Project, manifests string) (helm.ReleaseOutcome, error)
	Delete(ctx context.Context, projectName string) error
}

const (
	DeckhouseNamespacePrefix  = "d8-"
	KubernetesNamespacePrefix = "kube-"

	DeckhouseProjectName = "deckhouse"
	DefaultProjectName   = "default"

	VirtualTemplate = "virtual"
)

type Manager struct {
	client     client.Client
	helmClient helmClient
	logger     logr.Logger
}

func New(client client.Client, helmClient helmClient, logger logr.Logger) *Manager {
	return &Manager{
		client:     client,
		helmClient: helmClient,
		logger:     logger.WithName("project-manager"),
	}
}

func (m *Manager) Init(ctx context.Context, checker healthz.Checker, init *sync.WaitGroup) error {
	m.logger.Info("wait until webhook server start")
	check := func(ctx context.Context) (bool, error) {
		if err := checker(&http.Request{}); err != nil {
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

	// Refresh the namespace set from the live cluster BEFORE rendering. The schema-based renderer fans
	// its namespaced objects (NetworkPolicy, PodLoggingConfig) into every project namespace by reading
	// project.Status.Namespaces; without this pre-collect the render would use the stale status and
	// miss an additional namespace created in this very reconcile (the status is otherwise refreshed
	// only afterwards, and a status-only change does not re-trigger the render). The post-collect below
	// still runs to persist the final status.
	if nsStatus, err := m.collectNamespaceStatus(ctx, project); err != nil {
		m.logger.Error(err, "failed to pre-collect the project namespaces", "project", project.Name)
	} else {
		project.Status.Namespaces = nsStatus
	}

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
	} else if done, err := m.handleTemplate(ctx, project); done {
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

	if project.IsConditionFalse(v1alpha3.ProjectConditionTemplateRolesAllowed) {
		project.SetState(v1alpha3.ProjectStateError)
	} else {
		project.SetState(v1alpha3.ProjectStateDeployed)
	}
	if err := m.updateProjectStatus(ctx, project); err != nil {
		m.logger.Error(err, "failed to update the project status", "project", project.Name)
		return ctrl.Result{}, err
	}

	m.logger.Info("the project reconciled", "project", project.Name, "template", project.Spec.ProjectTemplateName)
	return ctrl.Result{}, nil
}

// failAndRequeue records a terminal-but-retriable template failure: it marks the project Errored, sets
// cond to False with the error text, persists the status and returns the error so the reconcile is
// requeued. It deliberately does NOT log — controller-runtime logs the returned error exactly once
// (the single-handling rule: an error is either logged or returned, never both).
func (m *Manager) failAndRequeue(ctx context.Context, project *v1alpha3.Project, cond string, err error) (bool, error) {
	project.SetState(v1alpha3.ProjectStateError)
	project.SetConditionFalse(cond, err.Error())
	if updateErr := m.updateProjectStatus(ctx, project); updateErr != nil {
		return true, updateErr
	}
	return true, err
}

// handleTemplate runs the template-based part of the reconciliation: resolving the template,
// validating the project against it and upgrading the helm release. The bool return value reports
// whether reconciliation must stop (an error already updated the status and the caller should return).
func (m *Manager) handleTemplate(ctx context.Context, project *v1alpha3.Project) (bool, error) {
	m.logger.Info("get the project template for project", "project", project.Name, "template", project.Spec.ProjectTemplateName)
	projectTemplate, err := m.projectTemplateByName(ctx, project.Spec.ProjectTemplateName)
	if err != nil {
		return m.failAndRequeue(ctx, project, v1alpha3.ProjectConditionProjectTemplateFound,
			fmt.Errorf("get project template %q: %w", project.Spec.ProjectTemplateName, err))
	}

	if projectTemplate == nil {
		m.logger.Info("the project template not found for the project", "project", project.Name, "template", project.Spec.ProjectTemplateName)
		project.SetState(v1alpha3.ProjectStateError)
		project.SetConditionFalse(v1alpha3.ProjectConditionProjectTemplateFound, "The project template not found")
		if updateErr := m.updateProjectStatus(ctx, project); updateErr != nil {
			return true, updateErr
		}
		return true, nil
	}

	project.SetConditionTrue(v1alpha3.ProjectConditionProjectTemplateFound)
	project.SetTemplateGeneration(projectTemplate.Generation)

	m.logger.Info("validate the project spec", "project", project.Name, "template", projectTemplate.Name)
	if err = validate.Project(project, legacyTemplate(projectTemplate)); err != nil {
		m.logger.Error(err, "failed to validate the project spec", "project", project.Name, "template", projectTemplate.Name)
		project.SetState(v1alpha3.ProjectStateError)
		project.SetConditionFalse(v1alpha3.ProjectConditionProjectValidated, err.Error())
		if updateErr := m.updateProjectStatus(ctx, project); updateErr != nil {
			return true, updateErr
		}
		return true, nil
	}

	project.SetConditionTrue(v1alpha3.ProjectConditionProjectValidated)

	m.logger.Info("upgrade resources for the project", "project", project.Name, "template", projectTemplate.Name)
	filtered, refs, err := m.upgradeResources(ctx, project, projectTemplate)
	if err != nil {
		return m.failAndRequeue(ctx, project, v1alpha3.ProjectConditionProjectResourcesUpgraded,
			fmt.Errorf("upgrade project resources: %w", err))
	}

	project.SetConditionTrue(v1alpha3.ProjectConditionProjectResourcesUpgraded)
	if filtered {
		project.SetConditionFalse(v1alpha3.ProjectConditionTemplateResourcesFiltered,
			"The template renders ResourceQuota or AuthorizationRule objects that are now managed via the Project spec.quota/spec.administrators fields; such objects were filtered out.")
	} else {
		project.SetConditionTrue(v1alpha3.ProjectConditionTemplateResourcesFiltered)
	}

	if err := m.applyTemplateRolesCondition(ctx, project, refs); err != nil {
		return m.failAndRequeue(ctx, project, v1alpha3.ProjectConditionTemplateRolesAllowed, err)
	}
	return false, nil
}

// upgradeResources installs/upgrades the project release and returns the binding roleRefs the template
// renders (for the TemplateRolesAllowed check). A schema-based template is rendered natively from its
// structured fields and applied via UpgradeManifests; a template that still carries a resourcesTemplate
// string is rendered through the legacy helm engine. The bool reports whether controller-managed kinds
// (ResourceQuota/AuthorizationRule) were filtered out.
func (m *Manager) upgradeResources(ctx context.Context, project *v1alpha3.Project, template *v1alpha2.ProjectTemplate) (bool, []helm.BindingRoleRef, error) {
	if isStructured(template) {
		manifests, err := render.Manifests(template, project)
		if err != nil {
			return false, nil, fmt.Errorf("render the project template: %w", err)
		}
		outcome, err := m.helmClient.UpgradeManifests(ctx, project, manifests)
		if err != nil {
			return false, nil, err
		}
		if !outcome.Applied {
			// The release was already up to date, so the apply short-circuited without post-rendering.
			// Recompute the filtered flag and role refs from the manifests (both pure functions of the
			// manifests) so the conditions stay accurate on no-op reconciles.
			outcome, err = m.helmClient.AnalyzeManifests(project, manifests)
			if err != nil {
				// The release is already applied; an analysis hiccup must not fail the reconcile.
				m.logger.Error(err, "failed to analyze the project manifests", "project", project.Name, "template", template.Name)
				return false, nil, nil
			}
		}
		return outcome.Filtered, outcome.RoleRefs, nil
	}

	legacy := legacyTemplate(template)
	outcome, err := m.helmClient.Upgrade(ctx, project, legacy)
	if err != nil {
		return false, nil, err
	}
	if !outcome.Applied {
		outcome, err = m.helmClient.AnalyzeRendered(project, legacy)
		if err != nil {
			m.logger.Error(err, "failed to analyze the project template", "project", project.Name, "template", template.Name)
			return false, nil, nil
		}
	}
	return outcome.Filtered, outcome.RoleRefs, nil
}

// applyTemplateRolesCondition evaluates the roleRefs rendered by a template and sets the
// TemplateRolesAllowed condition: False (naming every offending binding/role) when a binding grants
// a forbidden role, True otherwise.
func (m *Manager) applyTemplateRolesCondition(ctx context.Context, project *v1alpha3.Project, refs []helm.BindingRoleRef) error {
	var offending []string
	for _, ref := range refs {
		// The disabled annotation and the allow-list only concern ClusterRole references.
		if ref.RoleKind != "ClusterRole" {
			continue
		}
		projectBinding := ref.BindingKind == v1alpha3.ProjectRoleBindingKind || ref.BindingKind == v1alpha3.ClusterProjectRoleBindingKind
		reason, err := m.roleViolation(ctx, ref.RoleName, projectBinding)
		if err != nil {
			// Fail closed: a transient API error must not let a possibly-forbidden role pass as allowed.
			// Returning the error requeues the reconcile and leaves the previous condition untouched.
			return fmt.Errorf("verify template role %q: %w", ref.RoleName, err)
		}
		if reason != "" {
			offending = append(offending, fmt.Sprintf("%s %q grants ClusterRole %q (%s)", ref.BindingKind, ref.BindingName, ref.RoleName, reason))
		}
	}

	if len(offending) > 0 {
		// The render order is non-deterministic; sort so the condition message is stable across reconciles.
		slices.Sort(offending)
		project.SetConditionFalse(v1alpha3.ProjectConditionTemplateRolesAllowed,
			fmt.Sprintf("The template renders bindings that grant roles forbidden in projects: %s.", strings.Join(offending, "; ")))
		return nil
	}
	project.SetConditionTrue(v1alpha3.ProjectConditionTemplateRolesAllowed)
	return nil
}

// roleViolation returns a non-empty reason when a ClusterRole must not be granted via a project
// binding. enforceAllowList is set for ProjectRoleBinding/ClusterProjectRoleBinding references,
// which are restricted to the PRB/CPRB allow-list; native RoleBinding/ClusterRoleBinding references
// are only checked for the disabled annotation.
func (m *Manager) roleViolation(ctx context.Context, name string, enforceAllowList bool) (string, error) {
	if enforceAllowList && !rolebinding.IsRoleAllowed(name) {
		return "not in the allowed project role list: d8:project:*, d8:namespace:*, their capabilities and d8:custom:*", nil
	}

	clusterRole := &rbacv1.ClusterRole{}
	if err := m.client.Get(ctx, client.ObjectKey{Name: name}, clusterRole); err != nil {
		if apierrors.IsNotFound(err) {
			// A missing role cannot be granted; existence is enforced by the binding webhook, not here.
			return "", nil
		}
		// Fail closed: propagate transient errors so the caller requeues instead of reporting the
		// role as allowed (the previous behaviour silently masked API-server hiccups).
		return "", fmt.Errorf("get cluster role %q: %w", name, err)
	}

	if clusterRole.Annotations[rolebinding.AnnotationDisabledForProjects] == "true" {
		return "disabled for direct use in projects", nil
	}
	return "", nil
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
