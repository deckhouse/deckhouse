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
	"sync"
	"time"

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	templatemanager "controller/pkg/manager/template"

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
)

const controllerName = "d8-template-controller"

func Register(runtimeManager manager.Manager, defaultPath string, log logr.Logger) error {
	r := &reconciler{
		init:            new(sync.WaitGroup),
		log:             log.WithName(controllerName),
		client:          runtimeManager.GetClient(),
		templateManager: templatemanager.New(runtimeManager.GetClient(), log),
	}

	r.init.Add(1)

	templateController, err := controller.New(controllerName, runtimeManager, controller.Options{Reconciler: r})
	if err != nil {
		log.Error(err, "failed to create template controller")
		return err
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
				log.Info("failed to init template manager - try to retry", "error", e.Error())
				return true
			},
			func() error {
				return r.templateManager.Init(ctx, runtimeManager.GetWebhookServer().StartedChecker(), r.init, defaultPath)
			},
		)
	})); err != nil {
		r.log.Error(err, "failed to init template manager")
		return err
	}

	r.log.Info("initializing template controller")
	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha1.ProjectTemplate{}).
		WithEventFilter(predicate.Or[client.Object](
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{})).
		Complete(templateController)
}

var _ reconcile.Reconciler = &reconciler{}

type reconciler struct {
	init            *sync.WaitGroup
	templateManager *templatemanager.Manager
	client          client.Client
	log             logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	r.log.Info("reconciling the template", "template", req.Name)
	template := &v1alpha1.ProjectTemplate{}
	if err := r.client.Get(ctx, req.NamespacedName, template); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info("the template not found", "template", req.Name)
			return reconcile.Result{}, nil
		}
		r.log.Error(err, "error getting the template", "template", req.Name)
		return reconcile.Result{}, nil
	}

	// handle the project template deletion
	if !template.DeletionTimestamp.IsZero() {
		r.log.Info("the template was deleted", "template", template.Name)
		return reconcile.Result{}, nil
	}

	// ensure template
	r.log.Info("ensuring the template", "template", template.Name)
	return r.templateManager.Handle(ctx, template)
}
