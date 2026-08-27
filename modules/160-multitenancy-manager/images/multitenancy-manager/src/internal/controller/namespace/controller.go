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

package namespace

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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha3"
	namespacemanager "controller/internal/manager/namespace"
	projectmanager "controller/internal/manager/project"
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

	r.logger.Info("reconcile the namespace", "namespace", req.Name)
	namespace := new(corev1.Namespace)
	if err := r.client.Get(ctx, req.NamespacedName, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info("the namespace not found", "namespace", req.Name)
			return reconcile.Result{}, nil
		}
		r.logger.Error(err, "failed to get the namespace", "namespace", req.Name)
		return reconcile.Result{}, err
	}

	// A namespace on its way out is left alone: the project owns it now, and deleting it only makes
	// the project reconcile recreate it.
	if !namespace.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	if !isAdoptionCandidate(namespace) {
		return reconcile.Result{}, nil
	}

	return r.manager.Adopt(ctx, namespace)
}

// isAdoptionCandidate reports whether a namespace has to be turned into a project of its own: it
// must not be the default namespace, a reserved (d8-/kube-) namespace, a deckhouse-managed
// namespace (heritage=deckhouse), or a namespace already owned by a project. The latter covers both
// a project's main namespace and the additional namespaces created by a ProjectNamespace — neither
// must become a separate project.
func isAdoptionCandidate(obj metav1.Object) bool {
	name := obj.GetName()
	if name == projectmanager.DefaultProjectName {
		return false
	}
	if strings.HasPrefix(name, projectmanager.DeckhouseNamespacePrefix) || strings.HasPrefix(name, projectmanager.KubernetesNamespacePrefix) {
		return false
	}
	if obj.GetLabels()[v1alpha3.ResourceLabelHeritage] == v1alpha3.ResourceHeritageDeckhouse {
		return false
	}
	if _, owned := obj.GetLabels()[v1alpha3.ResourceLabelProject]; owned {
		return false
	}
	return true
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
	return isAdoptionCandidate(e.Object)
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
	return isAdoptionCandidate(e.ObjectNew)
}

// Delete is intentionally ignored: a namespace that disappears is recreated by the reconcile of the
// project that owns it, and a namespace that never belonged to a project has nothing to clean up.
func (p customPredicate[T]) Delete(_ event.TypedDeleteEvent[T]) bool {
	return false
}

func isNil(arg any) bool {
	if v := reflect.ValueOf(arg); !v.IsValid() || ((v.Kind() == reflect.Pointer ||
		v.Kind() == reflect.Interface ||
		v.Kind() == reflect.Slice ||
		v.Kind() == reflect.Map ||
		v.Kind() == reflect.Chan ||
		v.Kind() == reflect.Func) && v.IsNil()) {
		return true
	}
	return false
}
