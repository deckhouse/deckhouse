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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha1"
	templatemanager "controller/internal/manager/template"
)

const controllerName = "d8-template-controller"

func Register(runtimeManager manager.Manager, templatesPath string, logger logr.Logger) error {
	r := &reconciler{
		init:    new(sync.WaitGroup),
		logger:  logger.WithName(controllerName),
		client:  runtimeManager.GetClient(),
		manager: templatemanager.New(runtimeManager.GetClient(), logger),
	}

	r.init.Add(1)

	templateController, err := controller.New(controllerName, runtimeManager, controller.Options{Reconciler: r})
	if err != nil {
		return fmt.Errorf("create template controller: %w", err)
	}

	// init template manager, it has to ensure default templates
	if err = runtimeManager.Add(manager.RunnableFunc(func(ctx context.Context) error {
		return retry.OnError(
			wait.Backoff{
				Steps:    10,
				Duration: 100 * time.Millisecond,
				Factor:   2.0,
				Jitter:   0.1,
			},
			func(e error) bool {
				logger.Info("failed to init template manager, try to retry", "error", e.Error())
				return true
			},
			func() error {
				return r.manager.Init(ctx, runtimeManager.GetWebhookServer().StartedChecker(), r.init, templatesPath)
			},
		)
	})); err != nil {
		return fmt.Errorf("init project manager: %w", err)
	}

	r.logger.Info("initialize template controller")
	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.ProjectTemplate{}).
		WithEventFilter(predicate.Or[client.Object](
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{})).
		Complete(templateController)
}

var _ reconcile.Reconciler = &reconciler{}

type reconciler struct {
	init    *sync.WaitGroup
	manager *templatemanager.Manager
	client  client.Client
	logger  logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	r.logger.Info("reconcile the template", "template", req.Name)
	template := new(v1alpha1.ProjectTemplate)
	if err := r.client.Get(ctx, req.NamespacedName, template); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info("the template not found", "template", req.Name)
			return reconcile.Result{}, nil
		}
		r.logger.Error(err, "failed to get the template", "template", req.Name)
		return reconcile.Result{}, err
	}

	// handle the project template deletion
	if !template.DeletionTimestamp.IsZero() {
		r.logger.Info("the template deleted", "template", template.Name)
		return reconcile.Result{}, nil
	}

	// ensure template
	r.logger.Info("ensure the template", "template", template.Name)
	return r.manager.Handle(ctx, template)
}
