/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package project

import (
	"context"
	"reflect"
	"strings"
	"sync"

	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/consts"
	"controller/pkg/helm"
	projectmanager "controller/pkg/manager/project"

	"github.com/go-logr/logr"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const controllerName = "d8-project-controller"

func Register(runtimeManager manager.Manager, helmClient *helm.Client, log logr.Logger) error {
	r := &reconciler{
		init:           &sync.WaitGroup{},
		log:            log.WithName(controllerName),
		client:         runtimeManager.GetClient(),
		projectManager: projectmanager.New(runtimeManager.GetClient(), helmClient, log),
	}

	r.init.Add(1)

	// init project manager, project manager have to ensure default templates
	if err := runtimeManager.Add(manager.RunnableFunc(func(ctx context.Context) error {
		return r.projectManager.Init(ctx, runtimeManager.GetWebhookServer().StartedChecker(), r.init)
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
		WatchesMetadata(&v1.Namespace{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
			if _, ok := object.GetLabels()[consts.ProjectTemplateLabel]; ok {
				return nil
			}
			if strings.HasPrefix(object.GetName(), consts.KubernetesNamespacePrefix) || strings.HasPrefix(object.GetName(), consts.DeckhouseNamespacePrefix) {
				return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: consts.DeckhouseProjectName}}}
			}
			return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: consts.OthersProjectName}}}
		})).
		Complete(projectController)
}

type reconciler struct {
	init           *sync.WaitGroup
	projectManager *projectmanager.Manager
	client         client.Client
	log            logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	// wait for init
	r.init.Wait()

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

	// handle virtual projects
	if project.Spec.ProjectTemplateName == consts.VirtualTemplate {
		r.log.Info("reconcile the virtual project", "project", req.Name)
		return r.projectManager.HandleVirtual(ctx, project)
	}

	// handle the project deletion
	if !project.DeletionTimestamp.IsZero() {
		r.log.Info("deleting the project", "project", project.Name)
		return r.projectManager.Delete(ctx, project)
	}

	// ensure the project
	r.log.Info("ensuring the project", "project", project.Name)
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

	// skip projects that do not require sync
	if annotations := e.ObjectNew.GetAnnotations(); annotations != nil {
		if val, ok := annotations[consts.ProjectRequireSyncAnnotation]; ok && val == "true" {
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
