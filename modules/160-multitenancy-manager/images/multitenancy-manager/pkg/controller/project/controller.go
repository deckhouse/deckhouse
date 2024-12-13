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

package project

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/helm"
	projectmanager "controller/pkg/manager/project"
)

const controllerName = "d8-project-controller"

func Register(runtimeManager manager.Manager, helmClient *helm.Client, log logr.Logger) error {
	r := &reconciler{
		init:           new(sync.WaitGroup),
		log:            log.WithName(controllerName),
		client:         runtimeManager.GetClient(),
		projectManager: projectmanager.New(runtimeManager.GetClient(), helmClient, log),
	}

	r.init.Add(1)

	// init project manager, project manager have to ensure default templates
	if err := runtimeManager.Add(manager.RunnableFunc(func(ctx context.Context) error {
		return retry.OnError(
			wait.Backoff{
				Steps:    10,
				Duration: 100 * time.Millisecond,
				Factor:   2.0,
				Jitter:   0.1,
			},
			func(e error) bool {
				log.Info("failed to init project manager - retrying", "error", e.Error())
				return true
			},
			func() error {
				return r.projectManager.Init(ctx, runtimeManager.GetWebhookServer().StartedChecker(), r.init)
			},
		)
	})); err != nil {
		return fmt.Errorf("failed to init project manager: %w", err)
	}

	projectController, err := controller.New(controllerName, runtimeManager, controller.Options{Reconciler: r})
	if err != nil {
		return fmt.Errorf("failed to create project controller: %w", err)
	}

	r.log.Info("initializing project controller")
	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&v1alpha2.Project{}).
		WithEventFilter(predicate.Or[client.Object](
			predicate.AnnotationChangedPredicate{},
			predicate.GenerationChangedPredicate{},
			customPredicate[client.Object]{log: log})).
		Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
			if _, ok := object.GetLabels()[v1alpha2.ResourceLabelTemplate]; ok {
				return nil
			}
			if strings.HasPrefix(object.GetName(), projectmanager.KubernetesNamespacePrefix) || strings.HasPrefix(object.GetName(), projectmanager.DeckhouseNamespacePrefix) {
				return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: projectmanager.DeckhouseProjectName}}}
			}
			return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: projectmanager.DefaultProjectName}}}
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

	r.log.Info("reconciling the project", "project", req.Name)
	project := &v1alpha2.Project{}
	if err := r.client.Get(ctx, req.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info("the project not found", "project", req.Name)
			return reconcile.Result{}, nil
		}
		r.log.Error(err, "error getting the project", "project", req.Name)
		return reconcile.Result{}, nil
	}

	// handle virtual projects
	if project.Spec.ProjectTemplateName == projectmanager.VirtualTemplate {
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
	if val, ok := e.ObjectNew.GetAnnotations()[v1alpha2.ProjectAnnotationRequireSync]; ok && val == "true" {
		return true
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
