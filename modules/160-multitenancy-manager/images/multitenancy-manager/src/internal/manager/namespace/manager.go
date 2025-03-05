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
	"controller/apis/deckhouse.io/v1alpha2"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sync"
	"time"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/util/wait"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

const ProjectNameEmpty = "empty"

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

func (m *Manager) Handle(ctx context.Context, namespace *corev1.Namespace) (ctrl.Result, error) {
	project := &v1alpha2.Project{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha2.SchemeGroupVersion.String(),
			Kind:       v1alpha2.ProjectKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace.Name,
		},
		Spec: v1alpha2.ProjectSpec{
			ProjectTemplateName: ProjectNameEmpty,
		},
	}

	if err := m.ensureProject(ctx, project); err != nil {
		m.logger.Error(err, "failed to ensure project", "project", project)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (m *Manager) ensureProject(ctx context.Context, project *v1alpha2.Project) error {
	m.logger.Info("ensuring the project", "project", project.Name)
	if err := m.client.Create(ctx, project); err != nil || apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create the '%s' project: %w", project.Name, err)
	}

	m.logger.Info("the project ensured", "project", project.Name)
	return nil
}
