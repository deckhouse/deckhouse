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

package template

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
)

const (
	managedByHelm = "Helm"
)

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

	init.Done()

	return nil
}

// Adopt handles the explicit adoption flow: a namespace carrying the projects.deckhouse.io/adopt
// annotation is turned into a bare (template-less) Project that the user then manages. This is
// distinct from the auto-wrap flow (Wrap) and intentionally does not mark the project as
// managed-by-namespace.
func (m *Manager) Adopt(ctx context.Context, namespace *corev1.Namespace) (ctrl.Result, error) {
	// set adopt label
	labels := namespace.GetLabels()
	if len(labels) == 0 {
		labels = make(map[string]string)
	}
	labels[helm.ResourceLabelManagedBy] = managedByHelm
	namespace.SetLabels(labels)

	// set adopt annotations
	annotations := namespace.GetAnnotations()
	if len(annotations) == 0 {
		annotations = make(map[string]string)
	}
	annotations[helm.ResourceAnnotationReleaseName] = namespace.GetName()
	annotations[helm.ResourceAnnotationReleaseNamespace] = ""
	namespace.SetAnnotations(annotations)

	if err := m.client.Update(ctx, namespace); err != nil {
		m.logger.Error(err, "failed to update the namespace", "namespace", namespace.GetName())
		return ctrl.Result{}, fmt.Errorf("failed to update the '%s' namespace: %w", namespace.GetName(), err)
	}

	project := &v1alpha3.Project{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha3.SchemeGroupVersion.String(),
			Kind:       v1alpha3.ProjectKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace.Name,
		},
	}

	m.logger.Info("ensure the project", "project", project.Name)
	if err := m.client.Create(ctx, project); err != nil {
		if apierrors.IsAlreadyExists(err) {
			m.logger.Info("project already exists", "project", project.Name)
			delete(namespace.Annotations, v1alpha3.NamespaceAnnotationAdopt)
			if err = m.client.Update(ctx, namespace); err != nil {
				m.logger.Error(err, "failed to update the namespace", "namespace", project.Name)
				return ctrl.Result{}, fmt.Errorf("failed to update the '%s' namespace: %w", namespace.GetName(), err)
			}
			return ctrl.Result{}, nil
		}

		m.logger.Error(err, "failed to ensure the project", "project", project.Name)
		return ctrl.Result{}, fmt.Errorf("create the '%s' project: %w", project.Name, err)
	}

	m.logger.Info("the project ensured", "project", project.Name)

	return ctrl.Result{}, nil
}

// Wrap implements the auto-wrap flow: when allowNamespacesWithoutProjects is enabled, a
// user-created namespace is wrapped into a managed-by-namespace Project (template-less, named after
// the namespace). The namespace gets a finalizer so its deletion cascades to the project. Wrap is
// idempotent and never touches namespaces owned by a regular project.
func (m *Manager) Wrap(ctx context.Context, namespace *corev1.Namespace) (ctrl.Result, error) {
	project := new(v1alpha3.Project)
	err := m.client.Get(ctx, client.ObjectKey{Name: namespace.Name}, project)
	switch {
	case apierrors.IsNotFound(err):
		return m.createManagedProject(ctx, namespace)
	case err != nil:
		return ctrl.Result{}, fmt.Errorf("get the '%s' project: %w", namespace.Name, err)
	}

	// A project with the namespace name already exists. Only manage it when it is a
	// managed-by-namespace project; regular and detached projects must be left untouched (and our
	// finalizer/marker, if any lingers from a previous detach, removed).
	if !isManagedByNamespace(project) {
		if err := m.clearNamespaceManaged(ctx, namespace); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := m.ensureNamespaceManaged(ctx, namespace); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, m.syncParameters(ctx, namespace, project)
}

// HandleDeletion cascades a namespace deletion to its managed-by-namespace Project and then clears
// the namespace finalizer so the namespace can disappear.
func (m *Manager) HandleDeletion(ctx context.Context, namespace *corev1.Namespace) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(namespace, v1alpha3.NamespaceFinalizerManagedProject) {
		return ctrl.Result{}, nil
	}

	project := new(v1alpha3.Project)
	err := m.client.Get(ctx, client.ObjectKey{Name: namespace.Name}, project)
	switch {
	case err == nil:
		// only cascade to a project still owned by this namespace; a detached project survives.
		if isManagedByNamespace(project) {
			m.logger.Info("delete the managed project on namespace deletion", "namespace", namespace.Name, "project", project.Name)
			if err := m.client.Delete(ctx, project); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("delete the '%s' managed project: %w", project.Name, err)
			}
		}
	case !apierrors.IsNotFound(err):
		return ctrl.Result{}, fmt.Errorf("get the '%s' project: %w", namespace.Name, err)
	}

	return ctrl.Result{}, m.clearNamespaceManaged(ctx, namespace)
}

// createManagedProject creates the managed-by-namespace Project for the namespace and then sets the
// namespace finalizer. The project is created first so a doomed creation (e.g. a naming conflict)
// does not leave a dangling finalizer on the namespace.
func (m *Manager) createManagedProject(ctx context.Context, namespace *corev1.Namespace) (ctrl.Result, error) {
	project := &v1alpha3.Project{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha3.SchemeGroupVersion.String(),
			Kind:       v1alpha3.ProjectKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespace.Name,
			Labels: map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace},
		},
		Spec: v1alpha3.ProjectSpec{
			Parameters: mirrorParameters(namespace),
		},
	}

	m.logger.Info("auto-wrap the namespace into a managed project", "namespace", namespace.Name)
	if err := m.client.Create(ctx, project); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, fmt.Errorf("create the '%s' managed project: %w", project.Name, err)
		}
		// lost a race with another reconcile; the next pass syncs it.
	}

	return ctrl.Result{}, m.ensureNamespaceManaged(ctx, namespace)
}

// syncParameters mirrors the namespace user labels/annotations into the managed project's
// spec.parameters.namespace so the project stays a faithful representation of the namespace. The
// update is skipped when nothing changed, which keeps the namespace->project reconcile from looping.
func (m *Manager) syncParameters(ctx context.Context, namespace *corev1.Namespace, project *v1alpha3.Project) error {
	desired := mirrorParameters(namespace)
	if reflect.DeepEqual(project.Spec.Parameters, desired) {
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := new(v1alpha3.Project)
		if err := m.client.Get(ctx, client.ObjectKey{Name: namespace.Name}, current); err != nil {
			return fmt.Errorf("get the '%s' project: %w", namespace.Name, err)
		}
		// the label may have been removed (detached) between read and write.
		if !isManagedByNamespace(current) {
			return nil
		}
		if reflect.DeepEqual(current.Spec.Parameters, desired) {
			return nil
		}
		current.Spec.Parameters = desired
		return m.client.Update(ctx, current)
	})
}

// ensureNamespaceManaged marks the namespace as actively auto-wrapped. It adds the managed-project
// finalizer (so the namespace deletion is observed and cascades to the project) and stamps the
// project-managed-by-namespace marker label on the namespace itself. The marker lets the
// d8-multitenancy-manager admission policy tell a namespace-managed namespace apart from a regular
// project's main namespace: per card-16/ADR-2 the namespace is the source of truth, so the user may
// edit its labels/annotations and delete it (cascading the project), whereas a regular project's
// namespace stays protected. The marker carries the multitenancy.deckhouse.io/ prefix, so it is
// filtered out of the spec.parameters.namespace mirror and never feeds a reconcile loop.
func (m *Manager) ensureNamespaceManaged(ctx context.Context, namespace *corev1.Namespace) error {
	if controllerutil.ContainsFinalizer(namespace, v1alpha3.NamespaceFinalizerManagedProject) &&
		namespace.Labels[v1alpha3.ProjectLabelManagedByNamespace] == v1alpha3.ManagedByNamespace {
		return nil
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := new(corev1.Namespace)
		if err := m.client.Get(ctx, client.ObjectKey{Name: namespace.Name}, current); err != nil {
			return fmt.Errorf("get the '%s' namespace: %w", namespace.Name, err)
		}
		if !current.DeletionTimestamp.IsZero() {
			return nil
		}
		changed := controllerutil.AddFinalizer(current, v1alpha3.NamespaceFinalizerManagedProject)
		if current.Labels == nil {
			current.Labels = make(map[string]string)
		}
		if current.Labels[v1alpha3.ProjectLabelManagedByNamespace] != v1alpha3.ManagedByNamespace {
			current.Labels[v1alpha3.ProjectLabelManagedByNamespace] = v1alpha3.ManagedByNamespace
			changed = true
		}
		if !changed {
			return nil
		}
		return m.client.Update(ctx, current)
	})
}

// clearNamespaceManaged reverses ensureNamespaceManaged: it removes the managed-project finalizer
// and the project-managed-by-namespace marker label. It runs both on detach (the project lost the
// managed-by-namespace label and becomes a regular project, whose namespace must be re-protected by
// the admission policy) and during teardown (releasing the finalizer so the namespace can vanish).
func (m *Manager) clearNamespaceManaged(ctx context.Context, namespace *corev1.Namespace) error {
	if !controllerutil.ContainsFinalizer(namespace, v1alpha3.NamespaceFinalizerManagedProject) &&
		namespace.Labels[v1alpha3.ProjectLabelManagedByNamespace] != v1alpha3.ManagedByNamespace {
		return nil
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := new(corev1.Namespace)
		if err := m.client.Get(ctx, client.ObjectKey{Name: namespace.Name}, current); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("get the '%s' namespace: %w", namespace.Name, err)
		}
		changed := controllerutil.RemoveFinalizer(current, v1alpha3.NamespaceFinalizerManagedProject)
		if _, ok := current.Labels[v1alpha3.ProjectLabelManagedByNamespace]; ok {
			delete(current.Labels, v1alpha3.ProjectLabelManagedByNamespace)
			changed = true
		}
		if !changed {
			return nil
		}
		return m.client.Update(ctx, current)
	})
}

func isManagedByNamespace(project *v1alpha3.Project) bool {
	return project.Labels[v1alpha3.ProjectLabelManagedByNamespace] == v1alpha3.ManagedByNamespace
}

// mirrorParameters builds the spec.parameters.namespace value from the namespace user-defined
// labels/annotations, dropping the controller/platform-managed keys. It returns nil when there is
// nothing to mirror.
func mirrorParameters(namespace *corev1.Namespace) map[string]any {
	labels := filterUserMeta(namespace.GetLabels())
	annotations := filterUserMeta(namespace.GetAnnotations())

	ns := map[string]any{}
	if len(labels) > 0 {
		ns["labels"] = toAnyMap(labels)
	}
	if len(annotations) > 0 {
		ns["annotations"] = toAnyMap(annotations)
	}
	if len(ns) == 0 {
		return nil
	}
	return map[string]any{"namespace": ns}
}

// managedMetaPrefixes are the label/annotation key prefixes (or exact keys) owned by the platform;
// they are never mirrored into the project parameters, both to keep the mirror clean and to prevent
// the controller-applied project/heritage labels from feeding a reconcile loop.
var managedMetaPrefixes = []string{
	"projects.deckhouse.io/",
	"multitenancy.deckhouse.io/",
	"heritage",
	"app.kubernetes.io/managed-by",
	"kubernetes.io/metadata.name",
	"meta.helm.sh/",
	"kubectl.kubernetes.io/",
}

func filterUserMeta(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		managed := false
		for _, prefix := range managedMetaPrefixes {
			if k == prefix || strings.HasPrefix(k, prefix) {
				managed = true
				break
			}
		}
		if !managed {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func toAnyMap(in map[string]string) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
