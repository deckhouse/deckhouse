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
	"fmt"
	"reflect"
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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha2"
	namespacemanager "controller/internal/manager/namespace"
)

const controllerName = "d8-namespace-controller"

func Register(runtimeManager manager.Manager, logger logr.Logger) error {
	r := &reconciler{
		init:    new(sync.WaitGroup),
		logger:  logger.WithName(controllerName),
		client:  runtimeManager.GetClient(),
		manager: namespacemanager.New(runtimeManager.GetClient(), logger),
	}

	r.init.Add(1)

	namespaceController, err := controller.New(controllerName, runtimeManager, controller.Options{Reconciler: r})
	if err != nil {
		return fmt.Errorf("create namespace controller: %w", err)
	}

	// init namespace manager
	if err = runtimeManager.Add(manager.RunnableFunc(func(ctx context.Context) error {
		return retry.OnError(
			wait.Backoff{
				Steps:    10,
				Duration: 100 * time.Millisecond,
				Factor:   2.0,
				Jitter:   0.1,
			},
			func(e error) bool {
				logger.Info("failed to init namespace manager, try to retry", "error", e.Error())
				return true
			},
			func() error {
				return r.manager.Init(ctx, runtimeManager.GetWebhookServer().StartedChecker(), r.init)
			},
		)
	})); err != nil {
		return fmt.Errorf("init namespace manager: %w", err)
	}

	r.logger.Info("initialize namespace controller")
	return ctrl.NewControllerManagedBy(runtimeManager).
		For(&corev1.Namespace{}).
		WithEventFilter(customPredicate[client.Object]{logger: logger}).
		Complete(namespaceController)
}

var _ reconcile.Reconciler = &reconciler{}

type reconciler struct {
	init    *sync.WaitGroup
	manager *namespacemanager.Manager
	client  client.Client
	logger  logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait for init
	r.init.Wait()

	r.logger.Info("reconcile the namespace", "template", req.Name)
	namespace := new(corev1.Namespace)
	if err := r.client.Get(ctx, req.NamespacedName, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info("the namespace not found", "namespace", req.Name)
			return reconcile.Result{}, nil
		}
		r.logger.Error(err, "failed to get the namespace", "namespace", req.Name)
		return reconcile.Result{}, err
	}

	// handle the namespace deletion
	if !namespace.DeletionTimestamp.IsZero() {
		r.logger.Info("the namespace deleted", "namespace", namespace.Name)
		return reconcile.Result{}, nil
	}

	// ensure namespace
	r.logger.Info("ensure the project for the namespace", "namespace", namespace.Name)
	return r.manager.Handle(ctx, namespace)
}

type customPredicate[T metav1.Object] struct {
	predicate.TypedFuncs[T]
	logger logr.Logger
}

func (p customPredicate[T]) Create(e event.TypedCreateEvent[T]) bool {
	if isNil(e.Object) {
		p.logger.Error(nil, "create event has no object", "event", e)
		return false
	}

	// skip namespace that does not require to be adopted
	_, ok := e.Object.GetAnnotations()[v1alpha2.NamespaceAnnotationAdopt]
	return ok
}

func (p customPredicate[T]) Update(e event.TypedUpdateEvent[T]) bool {
	if isNil(e.ObjectOld) {
		p.logger.Error(nil, "update event has no old object to update", "event", e)
		return false
	}
	if isNil(e.ObjectNew) {
		p.logger.Error(nil, "update event has no new object for update", "event", e)
		return false
	}

	// skip namespace that does not require to be adopted
	_, ok := e.ObjectNew.GetAnnotations()[v1alpha2.NamespaceAnnotationAdopt]
	return ok
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
