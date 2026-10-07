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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
	namespacemanager "controller/internal/manager/namespace"
	"controller/internal/render"
	rolebinding "controller/internal/rolebinding"
	"controller/internal/startup"
	"controller/internal/validate"
)

// helmClient is the subset of *helm.Client that the project manager depends on. Depending on the
// interface rather than the concrete client lets Handle/upgradeResources be unit-tested with a fake
// (the concrete *helm.Client satisfies it).
type helmClient interface {
	Upgrade(ctx context.Context, project *v1alpha3.Project, template *v1alpha1.ProjectTemplate, beforeUpgrade helm.BeforeUpgrade) (helm.ReleaseOutcome, error)
	UpgradeManifests(ctx context.Context, project *v1alpha3.Project, manifests string, beforeUpgrade helm.BeforeUpgrade) (helm.ReleaseOutcome, error)
	AnalyzeRendered(project *v1alpha3.Project, template *v1alpha1.ProjectTemplate) (helm.ReleaseOutcome, error)
	AnalyzeManifests(project *v1alpha3.Project, manifests string) (helm.ReleaseOutcome, error)
	RenderManifests(project *v1alpha3.Project, manifests string, parametersSchema map[string]any) (string, error)
	CurrentRelease(ctx context.Context, projectName string) (*helm.ProjectRelease, error)
	Delete(ctx context.Context, projectName string) error
}

const (
	DeckhouseNamespacePrefix  = "d8-"
	KubernetesNamespacePrefix = "kube-"

	DeckhouseProjectName = "deckhouse"
	DefaultProjectName   = "default"

	VirtualTemplate = v1alpha3.VirtualProjectTemplateName

	// MinimalTemplate renders the project namespace and nothing else. It is what a project gets
	// when it names no template: the CRD schema defaults the field, and the controller falls back
	// to the same value for the explicit empty string the schema cannot default.
	MinimalTemplate = "simple"
)

// namespaceDeletionPollMax caps the interval at which a deleting project checks whether its
// namespace is gone. The interval starts at namespaceDeletionPoll and grows with the time the
// namespace has spent terminating, so a namespace that is stuck -- a foreign finalizer, a dangling
// APIService -- costs one reconcile a minute instead of twenty.
const namespaceDeletionPollMax = time.Minute

// namespaceDeletionPollFor returns the next check interval for a namespace that has been
// terminating for the given time: a quarter of it, within [namespaceDeletionPoll, namespaceDeletionPollMax].
func namespaceDeletionPollFor(terminatingFor time.Duration) time.Duration {
	return min(max(terminatingFor/4, namespaceDeletionPoll), namespaceDeletionPollMax)
}

// namespaceTerminationSummary renders what the namespace controller reports as still holding the
// namespace up, for the Project condition and the log.
func namespaceTerminationSummary(namespace *corev1.Namespace) string {
	var parts []string
	for _, cond := range namespace.Status.Conditions {
		if cond.Status != corev1.ConditionTrue {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", cond.Type, cond.Message))
	}
	if len(parts) == 0 {
		return "the namespace reports nothing remaining"
	}
	return strings.Join(parts, "; ")
}

// namespaceDeletionPoll is how often a deleting project checks whether its namespace is gone. The
// namespace controller drives the deletion; this loop only observes it, so a few seconds is enough
// to keep "kubectl delete project" responsive without hammering the API server.
const namespaceDeletionPoll = 3 * time.Second

type Manager struct {
	client     client.Client
	helmClient helmClient
	logger     logr.Logger
	// keptChecks holds, per project name, when refreshKeptObjects last read the objects of its
	// status.keptObjects.
	keptChecks sync.Map
	// manifestsClients hands out the client that applies spec.manifests of a template as the user of
	// a project (see manifests.go); without it such a project is in Error.
	manifestsClients ManifestsClients
	// manifestsApplied holds, per project name, the appliedManifests of its last apply.
	manifestsApplied sync.Map
}

func New(client client.Client, helmClient helmClient, logger logr.Logger, opts ...Option) *Manager {
	m := &Manager{
		client:     client,
		helmClient: helmClient,
		logger:     logger.WithName("project-manager"),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
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
	// The series of a handover and of the objects left from a Helm text release follow the status as
	// read, before any step that can stop the reconcile, so they outlive a restart.
	m.publishSeries(project)

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

	// ClearConditions keeps HandoverPending, the record of a handover that has not finished.
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

	// the objects of spec.manifests of the template, applied as the user of the project
	manifests, manifestsErr := m.reconcileManifests(ctx, project)

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

	// the objects left from a Helm text release stay listed until they are deleted or unlabelled
	leftRecheck, err := m.refreshKeptObjects(ctx, project)
	if err != nil {
		m.logger.Error(err, "failed to check the objects left from the Helm text release", "project", project.Name)
		leftRecheck = leftObjectsRecheck
	}

	if project.IsConditionFalse(v1alpha3.ProjectConditionTemplateRolesAllowed) || manifests.failed {
		project.SetState(v1alpha3.ProjectStateError)
	} else {
		project.SetState(v1alpha3.ProjectStateDeployed)
	}
	if err := m.updateProjectStatus(ctx, project); err != nil {
		m.logger.Error(err, "failed to update the project status", "project", project.Name)
		return ctrl.Result{}, err
	}

	// what a handover kept from a Helm text release goes once the standard fields work
	recheck, err := m.deleteKeptObjects(ctx, project)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("delete the objects kept from the Helm text release: %w", err)
	}
	recheck = sooner(sooner(recheck, leftRecheck), manifests.recheck)

	// a refusal of the API server is retried with the backoff of the controller, once the status says it
	if manifestsErr != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile the manifests of the template: %w", manifestsErr)
	}

	m.logger.Info("the project reconciled", "project", project.Name, "template", project.Spec.ProjectTemplateName)
	return ctrl.Result{RequeueAfter: recheck}, nil
}

// sooner is the shorter of two recheck intervals, where zero means none.
func sooner(a, b time.Duration) time.Duration {
	if a == 0 || b > 0 && b < a {
		return b
	}
	return a
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
	refs, err := m.upgradeResources(ctx, project, projectTemplate)
	if err != nil {
		return m.failAndRequeue(ctx, project, v1alpha3.ProjectConditionProjectResourcesUpgraded,
			fmt.Errorf("upgrade project resources: %w", err))
	}

	project.SetConditionTrue(v1alpha3.ProjectConditionProjectResourcesUpgraded)

	if err := m.applyTemplateRolesCondition(ctx, project, refs); err != nil {
		return m.failAndRequeue(ctx, project, v1alpha3.ProjectConditionTemplateRolesAllowed, err)
	}
	return false, nil
}

// upgradeResources installs/upgrades the project release and returns the binding roleRefs the template
// renders (for the TemplateRolesAllowed check). A schema-based template is rendered natively from its
// structured fields onto the project namespace as it is in the cluster and applied via UpgradeManifests;
// a template that still carries a resourcesTemplate string is rendered through the legacy helm engine.
// Either way an upgrade is prepared for what it drops from the deployed revision first (see transition.go).
func (m *Manager) upgradeResources(ctx context.Context, project *v1alpha3.Project, template *v1alpha2.ProjectTemplate) ([]helm.BindingRoleRef, error) {
	if isStructured(template) {
		liveLabels := map[string]string{}
		namespace := new(corev1.Namespace)
		switch err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, namespace); {
		case err == nil:
			liveLabels = namespace.Labels
		case !apierrors.IsNotFound(err):
			return nil, fmt.Errorf("get the '%s' namespace: %w", project.Name, err)
		}

		manifests, err := render.ManifestsOnto(template, project, liveLabels)
		if err != nil {
			return nil, fmt.Errorf("render the project template: %w", err)
		}
		outcome, err := m.helmClient.UpgradeManifests(ctx, project, manifests, m.beforeUpgrade(project))
		if err != nil {
			return nil, err
		}
		if !outcome.Applied {
			// The release was already up to date, so the apply short-circuited without post-rendering.
			// Recompute the role refs from the manifests (a pure function of the manifests) so the
			// condition stays accurate on no-op reconciles.
			outcome, err = m.helmClient.AnalyzeManifests(project, manifests)
			if err != nil {
				// The release is already applied; an analysis hiccup must not fail the reconcile.
				m.logger.Error(err, "failed to analyze the project manifests", "project", project.Name, "template", template.Name)
				return nil, nil
			}
		}
		return outcome.RoleRefs, nil
	}

	legacy := LegacyTemplate(template)
	outcome, err := m.helmClient.Upgrade(ctx, project, legacy, m.beforeUpgrade(project))
	if err != nil {
		return nil, err
	}
	if !outcome.Applied {
		outcome, err = m.helmClient.AnalyzeRendered(project, legacy)
		if err != nil {
			m.logger.Error(err, "failed to analyze the project template", "project", project.Name, "template", template.Name)
			return nil, nil
		}
	}
	return outcome.RoleRefs, nil
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
	m.Forget(project.Name)

	// delete the auto-managed cluster-scoped standard-field objects (administrators binding)
	if err := m.deleteStandardFields(ctx, project); err != nil {
		m.logger.Error(err, "failed to delete the project standard fields", "project", project.Name)
		return ctrl.Result{}, err
	}

	// the user that applied the manifests of the template loses its rights first; the objects it
	// applied are in the namespaces of the project and go with them
	if err := m.deleteManifestsBinding(ctx, project); err != nil {
		m.logger.Error(err, "failed to delete the manifests binding", "project", project.Name)
		return ctrl.Result{}, err
	}

	// the cluster-scoped objects a switch left go with the project; the others go with its namespaces
	if err := m.deleteLeftClusterObjects(ctx, project); err != nil {
		m.logger.Error(err, "failed to delete the cluster-scoped objects left from the Helm text release", "project", project.Name)
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
			return ctrl.Result{RequeueAfter: namespaceDeletionPoll}, nil
		}
		// Terminating: Kubernetes is purging the contents, or something in there holds a finalizer
		// and the namespace never finishes. Say so where "kubectl describe project" shows it, and
		// check less and less often the longer it takes. The message names the moment the
		// namespace started terminating rather than the time elapsed, so an unchanged situation
		// produces an identical condition and no status write: a namespace stuck for good must not
		// cost an etcd revision per poll.
		terminatingFor := time.Since(namespace.DeletionTimestamp.Time)
		message := fmt.Sprintf("waiting for the '%s' namespace to be deleted (terminating since %s); %s",
			project.Name, namespace.DeletionTimestamp.UTC().Format(time.RFC3339), namespaceTerminationSummary(namespace))
		if !project.IsConditionFalseWithMessage(v1alpha3.ProjectConditionNamespaceDeleted, message) {
			project.SetConditionFalse(v1alpha3.ProjectConditionNamespaceDeleted, message)
			if err := m.updateProjectStatus(ctx, project); err != nil {
				// The status is a courtesy; the deletion itself does not depend on it.
				m.logger.Error(err, "failed to record the namespace deletion status", "project", project.Name)
			}
		}
		poll := namespaceDeletionPollFor(terminatingFor)
		m.logger.Info("the project namespace is still terminating, waiting",
			"project", project.Name, "terminatingFor", terminatingFor.Round(time.Second), "nextCheck", poll)
		return ctrl.Result{RequeueAfter: poll}, nil
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
		project.IsVirtual() ||
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
