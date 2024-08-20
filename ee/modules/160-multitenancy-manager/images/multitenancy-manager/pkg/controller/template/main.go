/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template

import (
	"context"
	"sync"

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	templatemanager "controller/pkg/manager/template"

	"github.com/go-logr/logr"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const controllerName = "d8-template-controller"

func Register(runtimeManager manager.Manager, log logr.Logger, defaultPath string) error {
	r := &reconciler{
		init:            &sync.WaitGroup{},
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

	// init template manager
	if err = runtimeManager.Add(manager.RunnableFunc(func(ctx context.Context) error {
		return r.templateManager.Init(ctx, runtimeManager.GetWebhookServer().StartedChecker(), r.init, defaultPath)
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
	templateManager templatemanager.Interface
	client          client.Client
	log             logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	r.log.Info("reconciling template", "template", req.Name)
	template := &v1alpha1.ProjectTemplate{}
	if err := r.client.Get(ctx, req.NamespacedName, template); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info("template not found", "template", req.Name)
			return reconcile.Result{}, nil
		}
		r.log.Error(err, "error getting template", "template", req.Name)
		return reconcile.Result{}, nil
	}

	// handle deletion
	if !template.DeletionTimestamp.IsZero() {
		r.log.Info("template was deleted", "template", template.Name)
		return reconcile.Result{}, nil
	}

	// ensure template
	r.log.Info("ensuring template", "template", template.Name)
	return r.templateManager.Handle(ctx, template)
}
