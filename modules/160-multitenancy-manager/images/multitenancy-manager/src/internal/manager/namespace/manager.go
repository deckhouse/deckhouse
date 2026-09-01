/*
Copyright 2025 Flant JSC

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

package namespace

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
	"controller/internal/startup"
)

// WorkLimit bounds concurrent namespace/project writes during startup migration and
// leftover-marker sweeps. Helm upgrades stay serial on the project controller.
const WorkLimit = 8

type Manager struct {
	client client.Client
	logger logr.Logger
}

func New(client client.Client, logger logr.Logger) *Manager {
	return &Manager{
		client: client,
		logger: logger.WithName("namespace-manager"),
	}
}

// Init waits for the webhook server and then migrates the projects left over from the previous
// model. Both have to finish before the namespace controller starts reconciling: the project writes
// below go through the project validating webhook, and a reconcile racing the migration would adopt
// a namespace whose project is half-converted.
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

	if err := m.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate projects: %w", err)
	}

	migration.MarkDone()
	init.Done()

	return nil
}

// Adopt turns a namespace that belongs to no project into a project of its own. The template is
// picked from what the namespace already carries and the parameters reproduce its current state, so
// the first render changes nothing inside the namespace.
func (m *Manager) Adopt(ctx context.Context, namespace *corev1.Namespace) (ctrl.Result, error) {
	// One write: Helm ownership (needed even when a same-name Project already exists)
	// plus leftover retired markers. Callers already hold the namespace; do not Get it again.
	if err := m.persistNamespace(ctx, namespace, func(ns *corev1.Namespace) bool {
		stamped := helm.ApplyReleaseOwnership(ns, helm.ReleaseName(ns.Name))
		cleared := applyClearRetiredMarkers(ns)
		return stamped || cleared
	}); err != nil {
		return ctrl.Result{}, err
	}

	project := new(v1alpha3.Project)
	switch err := m.client.Get(ctx, client.ObjectKey{Name: namespace.Name}, project); {
	case err == nil:
		return ctrl.Result{}, nil
	case !apierrors.IsNotFound(err):
		return ctrl.Result{}, fmt.Errorf("get the '%s' project: %w", namespace.Name, err)
	}

	template := TemplateFor(namespace)
	project = &v1alpha3.Project{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha3.SchemeGroupVersion.String(),
			Kind:       v1alpha3.ProjectKind,
		},
		ObjectMeta: metav1.ObjectMeta{Name: namespace.Name},
		Spec: v1alpha3.ProjectSpec{
			ProjectTemplateName: template,
			Parameters:          ParametersFor(namespace, template),
		},
	}

	m.logger.Info("adopt the namespace into a project", "namespace", namespace.Name, "template", template)
	if err := m.client.Create(ctx, project); err != nil && !apierrors.IsAlreadyExists(err) {
		return ctrl.Result{}, fmt.Errorf("create the '%s' project: %w", project.Name, err)
	}

	return ctrl.Result{}, nil
}

// Migrate gives a template to every project that still lacks one: the projects the previous version
// auto-created around a namespace, and the template-less projects created by hand or by the retired
// adopt annotation. A failure on one project is logged and skipped rather than propagated, so a
// single broken project cannot keep the controller from starting.
func (m *Manager) Migrate(ctx context.Context) error {
	projects := new(v1alpha3.ProjectList)
	if err := m.client.List(ctx, projects); err != nil {
		return fmt.Errorf("list projects: %w", err)
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(WorkLimit)
	for i := range projects.Items {
		project := &projects.Items[i]
		if !needsTemplate(project) {
			continue
		}
		g.Go(func() error {
			if err := m.migrateProject(gctx, project); err != nil {
				m.logger.Error(err, "failed to migrate the project, skipping it", "project", project.Name)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// Retired markers live on the namespace, not the project. A previous run can have
	// written the template (needsTemplate is then false) and still left the finalizer
	// behind. Sweep every namespace so Terminating objects are not stuck on glue
	// nobody removes.
	return m.sweepRetiredMarkers(ctx)
}

// needsTemplate reports whether a project still belongs to the retired model: either it was
// auto-created around its namespace, or it has no template at all. Virtual projects are excluded —
// they are platform-owned and carry the virtual template.
func needsTemplate(project *v1alpha3.Project) bool {
	if project.Labels[v1alpha3.ProjectLabelVirtualProject] == "true" {
		return false
	}
	if project.Labels[v1alpha3.ProjectLabelManagedByNamespace] == v1alpha3.ManagedByNamespace {
		return true
	}
	return project.Spec.ProjectTemplateName == ""
}

func (m *Manager) migrateProject(ctx context.Context, project *v1alpha3.Project) error {
	if IsLeftoverWrap(project) {
		_, err := m.CompleteLeftover(ctx, project)
		return err
	}

	namespace := new(corev1.Namespace)
	err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, namespace)
	switch {
	case apierrors.IsNotFound(err):
		// Handmade project, namespace not created yet: no state to preserve.
		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: project.Name}}
	case err != nil:
		return fmt.Errorf("get the '%s' namespace: %w", project.Name, err)
	default:
		if err := m.persistNamespace(ctx, namespace, func(ns *corev1.Namespace) bool {
			stamped := helm.ApplyReleaseOwnership(ns, helm.ReleaseName(project.Name))
			cleared := applyClearRetiredMarkers(ns)
			return stamped || cleared
		}); err != nil {
			return err
		}
	}

	return m.applyInferredTemplate(ctx, project, namespace)
}

// sweepRetiredMarkers walks every namespace and peels leftover managed-by-namespace
// labels and finalizers, including on Terminating objects. Per-namespace errors are
// logged and skipped so one stuck object cannot block startup.
func (m *Manager) sweepRetiredMarkers(ctx context.Context) error {
	list := new(corev1.NamespaceList)
	if err := m.client.List(ctx, list); err != nil {
		return fmt.Errorf("list namespaces: %w", err)
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(WorkLimit)
	for i := range list.Items {
		ns := &list.Items[i]
		if !HasRetiredMarkers(ns) {
			continue
		}
		name := ns.Name
		g.Go(func() error {
			if err := m.persistNamespace(gctx, ns, applyClearRetiredMarkers); err != nil {
				m.logger.Error(err, "failed to clear retired namespace markers", "namespace", name)
			}
			return nil
		})
	}
	return g.Wait()
}

// HasRetiredMarkers reports leftover glue from the retired namespace-managed model.
func HasRetiredMarkers(obj metav1.Object) bool {
	if _, marked := obj.GetLabels()[v1alpha3.ProjectLabelManagedByNamespace]; marked {
		return true
	}
	return slices.Contains(obj.GetFinalizers(), v1alpha3.NamespaceFinalizerManagedProject)
}

// ClearRetiredMarkers strips leftover retired-model glue from an already-fetched namespace.
func (m *Manager) ClearRetiredMarkers(ctx context.Context, ns *corev1.Namespace) error {
	return m.persistNamespace(ctx, ns, applyClearRetiredMarkers)
}

// persistNamespace applies mutate to an already-fetched namespace and writes once.
// On conflict it re-gets and retries; a no-op mutate does not touch the API.
func (m *Manager) persistNamespace(ctx context.Context, ns *corev1.Namespace, mutate func(*corev1.Namespace) bool) error {
	if ns == nil {
		return nil
	}
	name := ns.Name
	if mutate(ns) {
		if err := m.client.Update(ctx, ns); err == nil {
			return nil
		} else if !apierrors.IsConflict(err) {
			return fmt.Errorf("update the '%s' namespace: %w", name, err)
		}
	} else {
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		fresh := new(corev1.Namespace)
		if err := m.client.Get(ctx, client.ObjectKey{Name: name}, fresh); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("get the '%s' namespace: %w", name, err)
		}
		if !mutate(fresh) {
			return nil
		}
		if err := m.client.Update(ctx, fresh); err != nil {
			return fmt.Errorf("update the '%s' namespace: %w", name, err)
		}
		return nil
	})
}

func applyClearRetiredMarkers(ns *corev1.Namespace) bool {
	if ns == nil {
		return false
	}
	_, marked := ns.Labels[v1alpha3.ProjectLabelManagedByNamespace]
	finalizers := make([]string, 0, len(ns.Finalizers))
	for _, finalizer := range ns.Finalizers {
		if finalizer != v1alpha3.NamespaceFinalizerManagedProject {
			finalizers = append(finalizers, finalizer)
		}
	}
	if !marked && len(finalizers) == len(ns.Finalizers) {
		return false
	}
	delete(ns.Labels, v1alpha3.ProjectLabelManagedByNamespace)
	ns.Finalizers = finalizers
	return true
}
