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
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
)

const managedByHelm = "Helm"

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

	if err := m.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate projects: %w", err)
	}

	init.Done()

	return nil
}

// Adopt turns a namespace that belongs to no project into a project of its own. The template is
// picked from what the namespace already carries and the parameters reproduce its current state, so
// the first render changes nothing inside the namespace.
func (m *Manager) Adopt(ctx context.Context, namespace *corev1.Namespace) (ctrl.Result, error) {
	project := new(v1alpha3.Project)
	switch err := m.client.Get(ctx, client.ObjectKey{Name: namespace.Name}, project); {
	case err == nil:
		// the project already owns this namespace, the project controller reconciles it from here.
		return ctrl.Result{}, nil
	case !apierrors.IsNotFound(err):
		return ctrl.Result{}, fmt.Errorf("get the '%s' project: %w", namespace.Name, err)
	}

	// Hand the namespace over to helm before the project exists: the release rendered for the
	// project has to own an object that predates it, and helm refuses to adopt one without the
	// ownership metadata.
	if err := m.ensureHelmAdoption(ctx, namespace.Name); err != nil {
		return ctrl.Result{}, err
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

	for i := range projects.Items {
		project := &projects.Items[i]
		if !needsTemplate(project) {
			continue
		}
		if err := m.migrateProject(ctx, project); err != nil {
			m.logger.Error(err, "failed to migrate the project, skipping it", "project", project.Name)
		}
	}

	return nil
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
	namespace := new(corev1.Namespace)
	err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, namespace)
	switch {
	case apierrors.IsNotFound(err):
		// No namespace yet, so there is no state to preserve; the minimal template renders it.
		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: project.Name}}
	case err != nil:
		return fmt.Errorf("get the '%s' namespace: %w", project.Name, err)
	default:
		if err := m.ensureHelmAdoption(ctx, project.Name); err != nil {
			return err
		}
	}

	template := TemplateFor(namespace)
	parameters := ParametersFor(namespace, template)

	m.logger.Info("migrate the project to a template", "project", project.Name, "template", template)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := new(v1alpha3.Project)
		if err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, current); err != nil {
			return fmt.Errorf("get the '%s' project: %w", project.Name, err)
		}
		if !needsTemplate(current) {
			return nil
		}
		current.Spec.ProjectTemplateName = template
		if len(parameters) > 0 {
			current.Spec.Parameters = parameters
		}
		delete(current.Labels, v1alpha3.ProjectLabelManagedByNamespace)
		return m.client.Update(ctx, current)
	}); err != nil {
		return fmt.Errorf("update the '%s' project: %w", project.Name, err)
	}

	return m.clearNamespaceManaged(ctx, project.Name)
}

// ensureHelmAdoption stamps the ownership metadata helm requires to take over an object it did not
// create. Without it the project release fails with an ownership conflict on the namespace.
func (m *Manager) ensureHelmAdoption(ctx context.Context, name string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		namespace := new(corev1.Namespace)
		if err := m.client.Get(ctx, client.ObjectKey{Name: name}, namespace); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("get the '%s' namespace: %w", name, err)
		}
		if !namespace.DeletionTimestamp.IsZero() {
			return nil
		}

		if namespace.Labels == nil {
			namespace.Labels = make(map[string]string, 1)
		}
		if namespace.Annotations == nil {
			namespace.Annotations = make(map[string]string, 2)
		}

		changed := namespace.Labels[helm.ResourceLabelManagedBy] != managedByHelm ||
			namespace.Annotations[helm.ResourceAnnotationReleaseName] != name ||
			namespace.Annotations[helm.ResourceAnnotationReleaseNamespace] != ""
		if !changed {
			return nil
		}

		namespace.Labels[helm.ResourceLabelManagedBy] = managedByHelm
		namespace.Annotations[helm.ResourceAnnotationReleaseName] = name
		namespace.Annotations[helm.ResourceAnnotationReleaseNamespace] = ""

		return m.client.Update(ctx, namespace)
	})
}

// clearNamespaceManaged strips what the retired model left on a namespace: the marker label that
// exempted it from the admission policy and the finalizer that cascaded its deletion to the project.
func (m *Manager) clearNamespaceManaged(ctx context.Context, name string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		namespace := new(corev1.Namespace)
		if err := m.client.Get(ctx, client.ObjectKey{Name: name}, namespace); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("get the '%s' namespace: %w", name, err)
		}

		_, marked := namespace.Labels[v1alpha3.ProjectLabelManagedByNamespace]
		finalizers := make([]string, 0, len(namespace.Finalizers))
		for _, finalizer := range namespace.Finalizers {
			if finalizer != v1alpha3.NamespaceFinalizerManagedProject {
				finalizers = append(finalizers, finalizer)
			}
		}
		if !marked && len(finalizers) == len(namespace.Finalizers) {
			return nil
		}

		delete(namespace.Labels, v1alpha3.ProjectLabelManagedByNamespace)
		namespace.Finalizers = finalizers

		return m.client.Update(ctx, namespace)
	})
}
