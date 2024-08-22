/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template

import (
	"context"
	"fmt"
	"sync"
	"time"

	"controller/pkg/consts"

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/validate"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

type Manager struct {
	client client.Client
	log    logr.Logger
}

func New(client client.Client, log logr.Logger) *Manager {
	return &Manager{
		client: client,
		log:    log.WithName("template-manager"),
	}
}

func (m *Manager) Init(ctx context.Context, checker healthz.Checker, init *sync.WaitGroup, defaultPath string) error {
	defer init.Done()

	m.log.Info("waiting for webhook server starting")
	check := func(ctx context.Context) (bool, error) {
		if err := checker(nil); err != nil {
			m.log.Info("webhook server not startup yet")
			return false, nil
		}
		m.log.Info("webhook server started")
		return true, nil
	}

	if err := wait.PollUntilContextTimeout(ctx, time.Second, 10*time.Second, true, check); err != nil {
		m.log.Error(err, "webhook server failed to start")
		return fmt.Errorf("webhook server failed to start: %w", err)
	}
	// to make sure that the server is started, without working server reconcile is failed
	if err := wait.PollUntilContextTimeout(ctx, time.Second, 10*time.Second, false, check); err != nil {
		m.log.Error(err, "webhook server failed to start")
		return fmt.Errorf("webhook server failed to start: %w", err)
	}
	m.log.Info("webhook server started")

	m.log.Info("ensuring default project templates")
	if err := m.ensureDefaultProjectTemplates(ctx, defaultPath); err != nil {
		m.log.Error(err, "failed to ensure default project templates")
		return err
	}
	m.log.Info("ensured default project templates")
	return nil
}

func (m *Manager) Handle(ctx context.Context, template *v1alpha1.ProjectTemplate) (ctrl.Result, error) {
	// validate project template
	if err := validate.ProjectTemplate(template); err != nil {
		if statusError := m.setTemplateStatus(ctx, template, err.Error(), false); statusError != nil {
			m.log.Error(statusError, "failed to update the template status")
			return ctrl.Result{Requeue: true}, statusError
		}
		return ctrl.Result{}, nil
	}

	// process template`s projects
	projects, err := m.projectsByTemplate(ctx, template)
	if err != nil {
		m.log.Error(err, "failed to get projects for the template", "template", template.Name)
		return ctrl.Result{Requeue: true}, nil
	}
	if len(projects) != 0 {
		m.log.Info("processing projects for the template", "template", template.Name, "projectsNum", len(projects))
		for _, project := range projects {
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err = m.client.Get(ctx, types.NamespacedName{Name: project.Name}, project); err != nil {
					m.log.Error(err, "failed to get the project", "project", project.Name)
					return err
				}
				m.log.Info("trigger the project to update", "template", template.Name, "project", project.Name)
				if project.Annotations == nil {
					project.Annotations = map[string]string{}
				}
				project.Annotations[consts.ProjectRequireSyncAnnotation] = "true"
				return m.client.Update(ctx, project)
			})
			if err != nil {
				m.log.Error(err, "failed to trigger the project", "template", template.Name, "project", project.Name)
				return ctrl.Result{Requeue: true}, nil
			}
		}
	} else {
		m.log.Info("no projects found for the template", "template", template.Name)
	}

	// set ready
	if err = m.setTemplateStatus(ctx, template, "The template is ready", true); err != nil {
		m.log.Error(err, "failed to update project status", "template", template.Name)
		return ctrl.Result{Requeue: true}, nil
	}

	m.log.Info("the template reconciled", "template", template.Name)
	return ctrl.Result{}, nil
}
