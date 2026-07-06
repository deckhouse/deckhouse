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
	"slices"
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

func Register(runtimeManager manager.Manager, logger logr.Logger, allowOrphanNamespaces bool) error {
	r := &reconciler{
		init:                  new(sync.WaitGroup),
		logger:                logger.WithName(controllerName),
		client:                runtimeManager.GetClient(),
		manager:               namespacemanager.New(runtimeManager.GetClient(), logger),
		allowOrphanNamespaces: allowOrphanNamespaces,
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
		WithEventFilter(customPredicate[client.Object]{logger: logger, allowOrphanNamespaces: allowOrphanNamespaces}).
		Complete(namespaceController)
}

var _ reconcile.Reconciler = &reconciler{}

type reconciler struct {
	init                  *sync.WaitGroup
	manager               *namespacemanager.Manager
	client                client.Client
	logger                logr.Logger
	allowOrphanNamespaces bool
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

	// handle the namespace deletion: cascade to the managed-by-namespace project and release the
	// namespace finalizer.
	if !namespace.DeletionTimestamp.IsZero() {
		r.logger.Info("the namespace is being deleted", "namespace", namespace.Name)
		return r.manager.HandleDeletion(ctx, namespace)
	}

	// explicit adoption flow takes precedence over auto-wrap.
	if _, ok := namespace.Annotations[v1alpha3.NamespaceAnnotationAdopt]; ok {
		r.logger.Info("adopt the namespace into a project", "namespace", namespace.Name)
		return r.manager.Adopt(ctx, namespace)
	}

	// auto-wrap is only active when orphan namespaces are allowed.
	if !r.allowOrphanNamespaces {
		return reconcile.Result{}, nil
	}

	// Wrap a fresh candidate, and keep an already-wrapped namespace in sync. After the first wrap
	// the project reconciler stamps the main namespace with the project-ownership label, so the
	// namespace is no longer an auto-wrap *candidate*; the managed-project finalizer is what marks
	// it as already wrapped and still needing its user labels/annotations mirrored into the managed
	// project's spec.parameters.namespace on every update (not only at create time).
	wrapped := slices.Contains(namespace.Finalizers, v1alpha3.NamespaceFinalizerManagedProject)
	if !isAutoWrapCandidate(namespace) && !wrapped {
		return reconcile.Result{}, nil
	}

	r.logger.Info("ensure the managed project for the namespace", "namespace", namespace.Name)
	return r.manager.Wrap(ctx, namespace)
}

// isAutoWrapCandidate reports whether a namespace may be auto-wrapped into a managed-by-namespace
// project: it must not be the default namespace, a reserved (d8-/kube-) namespace, a
// deckhouse-managed namespace (heritage=deckhouse), or a namespace already owned by a project. The
// latter covers both a project's main namespace and the additional namespaces created by a
// ProjectNamespace - neither must be turned into a separate managed-by-namespace project.
func isAutoWrapCandidate(obj metav1.Object) bool {
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
	logger                logr.Logger
	allowOrphanNamespaces bool
}

// shouldHandle decides whether a namespace event is relevant: namespaces carrying the managed-project
// finalizer (so sync and deletion keep working even if the flag flips), namespaces requesting
// adoption, and - when orphan namespaces are allowed - auto-wrap candidates.
func (p customPredicate[T]) shouldHandle(obj metav1.Object) bool {
	if slices.Contains(obj.GetFinalizers(), v1alpha3.NamespaceFinalizerManagedProject) {
		return true
	}
	if _, ok := obj.GetAnnotations()[v1alpha3.NamespaceAnnotationAdopt]; ok {
		return true
	}
	if !p.allowOrphanNamespaces {
		return false
	}
	return isAutoWrapCandidate(obj)
}

func (p customPredicate[T]) Create(e event.TypedCreateEvent[T]) bool {
	if isNil(e.Object) {
		p.logger.Error(nil, "create event has no object", "event", e)
		return false
	}
	return p.shouldHandle(e.Object)
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
	return p.shouldHandle(e.ObjectNew)
}

// Delete is intentionally ignored: namespaces wrapped by the controller carry a finalizer, so their
// deletion is observed as an Update (DeletionTimestamp set) and handled there.
func (p customPredicate[T]) Delete(_ event.TypedDeleteEvent[T]) bool {
	return false
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
