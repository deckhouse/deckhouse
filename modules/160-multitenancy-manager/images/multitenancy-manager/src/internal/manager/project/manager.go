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
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
	namespacemanager "controller/internal/manager/namespace"
	"controller/internal/render"
	"controller/internal/startup"
	"controller/internal/validate"
)

// helmClient is the subset of *helm.Client that the project manager depends on. Depending on the
// interface rather than the concrete client lets Handle/upgradeResources be unit-tested with a fake
// (the concrete *helm.Client satisfies it).
type helmClient interface {
	UpgradeManifests(ctx context.Context, project *v1alpha3.Project, manifests string) error
	Delete(ctx context.Context, projectName string) error
}

const (
	DeckhouseNamespacePrefix  = "d8-"
	KubernetesNamespacePrefix = "kube-"

	DeckhouseProjectName = "deckhouse"
	DefaultProjectName   = "default"

	VirtualTemplate = "virtual"

	// MinimalTemplate renders the project namespace and nothing else. It is what a project gets
	// when it names no template: the CRD schema defaults the field, and the controller falls back
	// to the same value for the explicit empty string the schema cannot default.
	MinimalTemplate = "simple"
)

// namespaceDeletionPoll is how often a deleting project checks whether its namespace is gone. The
// namespace controller drives the deletion; this loop only observes it, so a few seconds is enough
// to keep "kubectl delete project" responsive without hammering the API server.
const namespaceDeletionPoll = 3 * time.Second

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

func (m *Manager) Init(ctx context.Context, checker healthz.Checker, init *sync.WaitGroup, migration *startup.Migration) error {
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

	// Wait for leftover-project migration so ensureTemplateName cannot persist "simple"
	// on a template-less Project before Migrate infers the real template and stamps Helm.
	if err := migration.Wait(ctx); err != nil {
		return fmt.Errorf("wait for namespace migration: %w", err)
	}

	// Rebuild virtual-project inventory from the live namespace list. Status is
	// otherwise only refreshed when a watch event reaches HandleVirtual, and a
	// deleted namespace can leave a stale name behind (the Project spec does not
	// change, so a restart alone does not reconcile).
	if err := m.refreshVirtualProjects(ctx); err != nil {
		return fmt.Errorf("refresh virtual projects: %w", err)
	}

	m.logger.Info("the virtual projects ensured")
	init.Done()

	return nil
}

// Handle ensures project`s resources
func (m *Manager) Handle(ctx context.Context, project *v1alpha3.Project) (ctrl.Result, error) {
	if namespacemanager.IsLeftoverWrap(project) {
		deleted, err := namespacemanager.New(m.client, m.logger).CompleteLeftover(ctx, project)
		if err != nil {
			return ctrl.Result{}, err
		}
		if deleted {
			return ctrl.Result{}, nil
		}
		if err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, project); err != nil {
			if apierrors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, fmt.Errorf("get the '%s' project: %w", project.Name, err)
		}
	}

	// add finalizer and remove labels
	if err := m.prepareProject(ctx, project); err != nil {
		m.logger.Error(err, "failed to update the project", "project", project.Name)
		return ctrl.Result{}, err
	}

	// A project always has a template now. The CRD defaults the field when it is omitted, but an
	// explicit empty string is a value the apiserver keeps as it is — and that spelling was
	// meaningful before, so manifests and GitOps repos still carry it. Normalise it here instead of
	// rejecting it, otherwise such a project would look for a template named "" and never get its
	// namespace.
	if err := m.ensureTemplateName(ctx, project); err != nil {
		m.logger.Error(err, "failed to default the project template", "project", project.Name)
		return ctrl.Result{}, err
	}

	// Defense in depth: Adopt stamps Helm ownership, but a Project that already existed
	// (or whose Adopt raced) can still reach the first upgrade without the metadata.
	if err := helm.StampReleaseOwnership(ctx, m.client, project.Name); err != nil {
		var holdoff helm.StampHoldoffError
		if errors.As(err, &holdoff) {
			return ctrl.Result{RequeueAfter: holdoff.Remaining}, nil
		}
		if errors.Is(err, helm.ErrForeignRelease) {
			project.ClearConditions()
			_, err := m.failAndRequeue(ctx, project, v1alpha3.ProjectConditionHelmOwnership, err)
			return ctrl.Result{}, err
		}
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

	if done, err := m.handleTemplate(ctx, project); done {
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

	// A template stored as v1alpha1 with a Helm resourcesTemplate comes up marked and without the
	// string (see the conversion webhook). Rendering it as the empty structured template it now looks
	// like would upgrade the release to a lone Namespace and delete every object the Helm string used
	// to produce, so the project is parked in Error until an administrator rewrites the template and
	// removes the mark. The release is left exactly as it is.
	if projectTemplate.Annotations[v1alpha2.TemplateAnnotationLegacyHelm] == "true" {
		m.logger.Info("the project template carries the legacy Helm template mark, refusing to render", "project", project.Name, "template", projectTemplate.Name)
		project.SetState(v1alpha3.ProjectStateError)
		msg := fmt.Sprintf(
			"The '%s' project template was a Helm resourcesTemplate in v1alpha1, which v1alpha2 does not carry. "+
				"Rewrite the template with structured fields and remove the %q annotation; the project release is left untouched until then.",
			projectTemplate.Name, v1alpha2.TemplateAnnotationLegacyHelm)
		if _, ok := projectTemplate.Annotations[v1alpha2.TemplateAnnotationLegacyHelmBody]; ok {
			msg += fmt.Sprintf(" The template it used to render is kept in the %q annotation.", v1alpha2.TemplateAnnotationLegacyHelmBody)
		}
		project.SetConditionFalse(v1alpha3.ProjectConditionProjectTemplateUsable, msg)
		if updateErr := m.updateProjectStatus(ctx, project); updateErr != nil {
			return true, updateErr
		}
		return true, nil
	}
	project.SetConditionTrue(v1alpha3.ProjectConditionProjectTemplateUsable)

	m.logger.Info("validate the project spec", "project", project.Name, "template", projectTemplate.Name)
	if err = validate.Project(project, projectTemplate); err != nil {
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
	if err = m.upgradeResources(ctx, project, projectTemplate); err != nil {
		return m.failAndRequeue(ctx, project, v1alpha3.ProjectConditionProjectResourcesUpgraded,
			fmt.Errorf("upgrade project resources: %w", err))
	}

	project.SetConditionTrue(v1alpha3.ProjectConditionProjectResourcesUpgraded)
	return false, nil
}

// upgradeResources renders the template natively from its structured fields and installs/upgrades
// the project release from the result. An unchanged render is a no-op on the Helm side.
func (m *Manager) upgradeResources(ctx context.Context, project *v1alpha3.Project, template *v1alpha2.ProjectTemplate) error {
	manifests, err := render.Manifests(template, project)
	if err != nil {
		return fmt.Errorf("render the project template: %w", err)
	}
	return m.helmClient.UpgradeManifests(ctx, project, manifests)
}

// HandleVirtual inventories unowned namespaces onto a virtual project. It does
// not create, stamp, or recreate namespaces — delete stays delete.
func (m *Manager) HandleVirtual(ctx context.Context, project *v1alpha3.Project) (ctrl.Result, error) {
	namespaces := new(corev1.NamespaceList)
	if err := m.client.List(ctx, namespaces); err != nil {
		m.logger.Error(err, "failed to list namespaces", "project", project.Name)
		return ctrl.Result{}, err
	}

	claimed, err := m.realProjectNames(ctx)
	if err != nil {
		m.logger.Error(err, "failed to list projects", "project", project.Name)
		return ctrl.Result{}, err
	}

	var involvedNamespaces []string
	for i := range namespaces.Items {
		ns := &namespaces.Items[i]
		if VirtualProjectName(ns) != project.Name {
			continue
		}
		if _, taken := claimed[ns.Name]; taken {
			continue
		}
		involvedNamespaces = append(involvedNamespaces, ns.Name)
	}

	if err := m.updateVirtualProject(ctx, project, involvedNamespaces); err != nil {
		m.logger.Error(err, "failed to update the virtual project", "project", project.Name)
		return ctrl.Result{}, err
	}

	m.logger.Info("the virtual project reconciled", "project", project.Name)
	return ctrl.Result{}, nil
}

func (m *Manager) refreshVirtualProjects(ctx context.Context) error {
	for _, name := range []string{DeckhouseProjectName, DefaultProjectName} {
		project := new(v1alpha3.Project)
		if err := m.client.Get(ctx, client.ObjectKey{Name: name}, project); err != nil {
			return fmt.Errorf("get the '%s' virtual project: %w", name, err)
		}
		if _, err := m.HandleVirtual(ctx, project); err != nil {
			return err
		}
	}
	return nil
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

	// The uninstall above only issues the deletes; it does not wait for them. The namespace is part
	// of the release, and its own removal is what takes the time: every object inside has to go
	// first. Dropping the finalizer here would make the Project vanish while the namespace is still
	// Terminating -- so "kubectl delete project" returns at once, where "kubectl delete ns" used to
	// block until the environment was gone, and a script that relied on that ordering breaks. The
	// finalizer therefore stays until the namespace is really gone; the Project sits in Terminating
	// exactly as long as its namespace does, which is what the old contract promised.
	namespace := new(corev1.Namespace)
	switch err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, namespace); {
	case err == nil:
		if namespace.DeletionTimestamp.IsZero() {
			// Not even terminating: the uninstall did not reach it (a foreign release owns it, or the
			// delete was dropped). Retrying the uninstall is what the next reconcile does.
			m.logger.Info("the project namespace is not terminating yet, waiting", "project", project.Name)
		}
		return ctrl.Result{RequeueAfter: namespaceDeletionPoll}, nil
	case !apierrors.IsNotFound(err):
		return ctrl.Result{}, fmt.Errorf("get the '%s' namespace: %w", project.Name, err)
	}

	// remove finalizer
	if err := m.removeFinalizer(ctx, project); err != nil {
		m.logger.Error(err, "failed to remove finalizer from the project", "project", project.Name)
		return ctrl.Result{}, err
	}

	m.logger.Info("the project deleted", "project", project.Name)
	return ctrl.Result{}, nil
}

// ensureTemplateName replaces an explicitly empty spec.projectTemplateName with the minimal template
// and persists it, so the stored object says what the controller actually does.
//
// The CRD schema defaults the field when it is absent, but an explicit "" is a value the apiserver
// keeps verbatim, and that spelling used to mean "a project without a template" — so manifests and
// GitOps repositories still carry it. Without this fallback such a project would look for a template
// named "" and never get its namespace. Virtual projects are skipped: they are platform-owned and
// carry their own template. Leftover wrap projects are skipped too: writing "simple" here would
// pin the wrong template after a failed migrate.
func (m *Manager) ensureTemplateName(ctx context.Context, project *v1alpha3.Project) error {
	if project.Spec.ProjectTemplateName != "" ||
		project.Labels[v1alpha3.ProjectLabelVirtualProject] == "true" ||
		namespacemanager.IsLeftoverWrap(project) {
		return nil
	}

	m.logger.Info("the project names no template, falling back to the minimal one",
		"project", project.Name, "template", MinimalTemplate)

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := new(v1alpha3.Project)
		if err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, current); err != nil {
			return fmt.Errorf("get the '%s' project: %w", project.Name, err)
		}
		if current.Spec.ProjectTemplateName != "" {
			return nil
		}
		current.Spec.ProjectTemplateName = MinimalTemplate
		return m.client.Update(ctx, current)
	}); err != nil {
		return fmt.Errorf("set the template of the '%s' project: %w", project.Name, err)
	}

	project.Spec.ProjectTemplateName = MinimalTemplate
	return nil
}
