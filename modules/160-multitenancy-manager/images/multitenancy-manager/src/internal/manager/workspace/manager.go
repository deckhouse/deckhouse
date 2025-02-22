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

package workspace

import (
	"context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
)

const defaultRequeue = time.Minute

type Manager struct {
	client client.Client
	logger logr.Logger
}

func New(client client.Client, logger logr.Logger) *Manager {
	return &Manager{
		client: client,
		logger: logger.WithName("workspace-manager"),
	}
}

func (m *Manager) Handle(ctx context.Context, workspace *v1alpha1.Workspace) (ctrl.Result, error) {
	project := m.projectByWorkspace(ctx, workspace)
	if project == nil {
		m.logger.Info("the workspace skipped: the project not found or not deployed", "workspace", workspace.Name, workspace.Namespace)
		return ctrl.Result{RequeueAfter: defaultRequeue}, nil
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Finalizers: []string{
				v1alpha1.WorkspaceFinalizer,
			},
			Name: project.Name + "-" + workspace.Name,
			Labels: map[string]string{
				v1alpha2.ResourceLabelHeritage: v1alpha2.ResourceHeritageMultitenancy,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(project, v1alpha1.SchemeGroupVersion.WithKind("Project")),
			},
		},
	}

	if err := m.client.Create(ctx, ns); err != nil {
		m.logger.Error(err, "failed to create the namespace", "project", project.Name, "workspace", workspace.Name, "namespace", ns.Name)
		return ctrl.Result{}, err
	}

	m.logger.Info("the workspace reconciled", "workspace", workspace.Name)
	return ctrl.Result{}, nil
}

func (m *Manager) Delete(ctx context.Context, workspace *v1alpha1.Workspace) (ctrl.Result, error) {
	project := m.projectByWorkspace(ctx, workspace)
	if project == nil {
		controllerutil.RemoveFinalizer(workspace, v1alpha1.WorkspaceFinalizer)
		return ctrl.Result{}, m.client.Update(ctx, workspace)
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: project.Name + "-" + workspace.Name,
		},
	}

	if err := m.client.Delete(ctx, ns); err != nil {
		m.logger.Error(err, "failed to delete the namespace", "project", project.Name, "workspace", workspace.Name)
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(workspace, v1alpha1.WorkspaceFinalizer)
	return ctrl.Result{}, m.client.Update(ctx, workspace)
}

func (m *Manager) projectByWorkspace(ctx context.Context, workspace *v1alpha1.Workspace) *v1alpha2.Project {
	project := new(v1alpha2.Project)
	if err := m.client.Get(ctx, client.ObjectKey{Name: workspace.Namespace}, project); err != nil {
		return nil
	}

	if project.Status.State != v1alpha2.ProjectStateDeployed {
		return nil
	}

	return project
}
