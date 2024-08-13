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

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/helm"
	"controller/pkg/validate"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

type Interface interface {
	Init(ctx context.Context, checker healthz.Checker) error
	Handle(ctx context.Context, template *v1alpha1.ProjectTemplate) (ctrl.Result, error)
}
type manager struct {
	init   sync.WaitGroup
	log    logr.Logger
	client client.Client
}

func New(client client.Client, log logr.Logger) Interface {
	return &manager{
		client: client,
		log:    log.WithName("template-manager"),
		init:   sync.WaitGroup{},
	}
}

func (m *manager) Init(ctx context.Context, checker healthz.Checker) error {
	m.init.Add(1)
	defer m.init.Done()

	m.log.Info("waiting for webhook server starting")
	check := func(ctx context.Context) (bool, error) {
		if err := checker(nil); err != nil {
			m.log.Info("webhook server not startup yet")
			return false, nil
		}
		m.log.Info("webhook server started")
		return true, nil
	}

	if err := wait.PollUntilContextTimeout(ctx, time.Second, 7*time.Second, true, check); err != nil {
		m.log.Error(err, "webhook server failed to start")
		return fmt.Errorf("webhook server failed to start: %w", err)
	}
	return nil
}

func (m *manager) Handle(ctx context.Context, template *v1alpha1.ProjectTemplate) (ctrl.Result, error) {
	//wait for init
	m.init.Wait()

	// validate project template
	if err := validate.ProjectTemplate(template); err != nil {
		template.Status.Message = err.Error()
		if err = m.client.Status().Update(ctx, template); err != nil {
			m.log.Error(err, "failed to update template", "template", template.Name)
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, nil
	}

	// process template`s projects
	projects, err := m.projectsByTemplate(ctx, template)
	if err != nil {
		m.log.Error(err, "failed to get projects for template", "template", template.Name)
		return ctrl.Result{Requeue: true}, err
	}
	if len(projects) != 0 {
		m.log.Info("processing projects for template", "template", template.Name, "projectsNum", len(projects))
		for _, project := range projects {
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err = m.client.Get(ctx, types.NamespacedName{Name: project.Name}, project); err != nil {
					m.log.Error(err, "failed to get project", "project", project.Name)
					return err
				}
				m.log.Info("trigger project to update", "template", template.Name, "project", project.Name)
				if project.Annotations == nil {
					project.Annotations = map[string]string{}
				}
				project.Annotations[helm.ProjectRequireSyncAnnotation] = "true"
				return m.client.Update(ctx, project)
			})
			if err != nil {
				m.log.Error(err, "failed to trigger project", "template", template.Name, "project", project.Name)
				return ctrl.Result{Requeue: true}, err
			}
		}
	} else {
		m.log.Info("no projects found for template", "template", template.Name)
	}

	// set ready
	template.Status.Ready = true
	if err = m.client.Status().Update(ctx, template); err != nil {
		m.log.Error(err, "failed to update template status", "template", template.Name)
		return ctrl.Result{Requeue: true}, err
	}
	m.log.Info("template reconciled", "template", template.Name)
	return ctrl.Result{}, nil
}

func (m *manager) projectsByTemplate(ctx context.Context, template *v1alpha1.ProjectTemplate) ([]*v1alpha2.Project, error) {
	projects := new(v1alpha2.ProjectList)
	if err := m.client.List(ctx, projects, client.MatchingLabels{helm.ProjectTemplateLabel: template.Name}); err != nil {
		return nil, err
	}
	if len(projects.Items) == 0 {
		return nil, nil
	}
	var result []*v1alpha2.Project
	for _, project := range projects.Items {
		if project.Status.Sync {
			if project.Annotations != nil {
				if _, ok := project.Annotations[helm.ProjectRequireSyncAnnotation]; ok {
					m.log.Info("skipping project due to sync annotation", "project", project.Name)
					continue
				}
			}
			result = append(result, project.DeepCopy())
		}
	}
	return result, nil
}
