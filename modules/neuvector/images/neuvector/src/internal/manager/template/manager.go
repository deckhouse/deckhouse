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

package template

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/internal/validate"
)

type Manager struct {
	client client.Client
	logger logr.Logger
}

func New(client client.Client, logger logr.Logger) *Manager {
	return &Manager{
		client: client,
		logger: logger.WithName("template-manager"),
	}
}

func (m *Manager) Init(ctx context.Context, checker healthz.Checker, init *sync.WaitGroup, templatesPath string) error {
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

	m.logger.Info("ensure default project templates")
	if err := m.ensureDefaultProjectTemplates(ctx, templatesPath); err != nil {
		return fmt.Errorf("ensure default project templates: %w", err)
	}

	m.logger.Info("default project templates ensured")
	init.Done()

	return nil
}

func (m *Manager) Handle(ctx context.Context, template *v1alpha1.ProjectTemplate) (ctrl.Result, error) {
	// validate project template
	if err := validate.ProjectTemplate(template); err != nil {
		if statusError := m.setTemplateStatus(ctx, template, err.Error(), false); statusError != nil {
			m.logger.Error(statusError, "failed to update the template status", "template", template.Name)
			return ctrl.Result{}, statusError
		}
		return ctrl.Result{}, nil
	}

	// process template`s projects
	projects, err := m.projectsByTemplate(ctx, template)
	if err != nil {
		m.logger.Error(err, "failed to get projects for the template", "template", template.Name)
		return ctrl.Result{Requeue: true}, nil
	}
	if len(projects) != 0 {
		m.logger.Info("process projects for the template", "template", template.Name, "projectsNum", len(projects))
		for _, project := range projects {
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err = m.client.Get(ctx, client.ObjectKey{Name: project.Name}, project); err != nil {
					return fmt.Errorf("get the '%s' project: %w", project.Name, err)
				}
				m.logger.Info("trigger the project to update", "template", template.Name, "project", project.Name)
				if project.Annotations == nil {
					project.Annotations = map[string]string{}
				}
				project.Annotations[v1alpha2.ProjectAnnotationRequireSync] = "true"
				return m.client.Update(ctx, project)
			})
			if err != nil {
				m.logger.Error(err, "failed to trigger the project", "template", template.Name, "project", project.Name)
				return ctrl.Result{}, err
			}
		}
	} else {
		m.logger.Info("no projects found for the template", "template", template.Name)
	}

	// set ready
	if err = m.setTemplateStatus(ctx, template, "The template ready", true); err != nil {
		m.logger.Error(err, "failed to update project status", "template", template.Name)
		return ctrl.Result{}, err
	}

	m.logger.Info("the template reconciled", "template", template.Name)
	return ctrl.Result{}, nil
}
