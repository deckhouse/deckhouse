/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package project

import (
	"context"
	"errors"
	"reflect"

	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/helm"
	projectmanager "controller/pkg/manager/project"

	"github.com/go-logr/logr"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const controllerName = "d8-project-controller"

func Register(ctx context.Context, runtimeManager manager.Manager, helmClient helm.Interface, log logr.Logger, defaultPath string) error {
	r := &reconciler{
		log:            log.WithName(controllerName),
		client:         runtimeManager.GetClient(),
		projectManager: projectmanager.New(runtimeManager.GetClient(), helmClient, log),
	}

	// wait for cache sync
	go func() {
		if ok := runtimeManager.GetCache().WaitForCacheSync(ctx); !ok {
			r.log.Error(errors.New("sync cash error"), "Sync cache failed")
		}
	}()

	// init project manager, project manager have to ensure default templates
	if err := runtimeManager.Add(manager.RunnableFunc(func(ctx context.Context) error {
		return r.projectManager.Init(ctx, defaultPath)
	})); err != nil {
		r.log.Error(err, "failed to init project manager")
		return err
	}

	projectController, err := controller.New(controllerName, runtimeManager, controller.Options{Reconciler: r})
	if err != nil {
		log.Error(err, "failed to create project controller")
		return err
	}

	r.log.Info("initializing project controller")
	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha2.Project{}).
		WithEventFilter(predicate.Or[client.Object](
			predicate.AnnotationChangedPredicate{},
			predicate.GenerationChangedPredicate{},
			customPredicate[client.Object]{log: log})).
		Complete(projectController)
}

type reconciler struct {
	projectManager projectmanager.Interface
	client         client.Client
	log            logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	r.log.Info("reconciling project", "project", req.Name)
	project := &v1alpha2.Project{}
	if err := r.client.Get(ctx, req.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info("project not found", "project", req.Name)
			return reconcile.Result{}, nil
		}
		r.log.Error(err, "error getting project", "project", req.Name)
		return reconcile.Result{}, nil
	}

	// handle deletion
	if !project.DeletionTimestamp.IsZero() {
		r.log.Info("deleting project", "project", project.Name)
		return r.projectManager.Delete(ctx, project)
	}

	// ensuring the project
	r.log.Info("ensuring project", "project", project.Name)
	return r.projectManager.Handle(ctx, project)
}

type customPredicate[T metav1.Object] struct {
	predicate.TypedFuncs[T]
	log logr.Logger
}

func (p customPredicate[T]) Update(e event.TypedUpdateEvent[T]) bool {
	if isNil(e.ObjectOld) {
		p.log.Error(nil, "Update event has no old object to update", "event", e)
		return false
	}
	if isNil(e.ObjectNew) {
		p.log.Error(nil, "Update event has no new object for update", "event", e)
		return false
	}

	if e.ObjectNew.GetAnnotations() != nil {
		if val, ok := e.ObjectNew.GetAnnotations()[helm.ProjectRequireSyncAnnotation]; ok && val == "true" {
			return true
		}
	}

	return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration()
}

func (p customPredicate[T]) Delete(_ event.TypedDeleteEvent[T]) bool {
	p.log.Info("new project deletion event")
	return true
}

func isNil(arg any) bool {
	if v := reflect.ValueOf(arg); !v.IsValid() || ((v.Kind() == reflect.Ptr ||
		v.Kind() == reflect.Interface ||
		v.Kind() == reflect.Slice ||
		v.Kind() == reflect.Map ||
		v.Kind() == reflect.Chan ||
		v.Kind() == reflect.Func) && v.IsNil()) {
		return true
	}
	return false
}
