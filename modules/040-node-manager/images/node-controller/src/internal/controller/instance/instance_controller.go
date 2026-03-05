/*
Copyright 2026 Flant JSC

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

package instance

import (
	"context"
	"fmt"
	"time"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/controller/common/machine"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

type InstanceReconciler struct {
	client.Client
	machineFactory machine.MachineFactory
}

type reconcileStep func(ctx context.Context, instance *deckhousev1alpha2.Instance) (done bool, result ctrl.Result, err error)

const (
	instanceRequeueInterval = time.Minute
)

func SetupInstanceController(mgr ctrl.Manager) error {
	if err := (&InstanceReconciler{
		Client:         mgr.GetClient(),
		machineFactory: machine.NewMachineFactory(),
	}).
		SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to setup instance reconciler: %w", err)
	}

	return nil
}

func (r *InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.machineFactory == nil {
		return fmt.Errorf("machineFactory is required")
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("instance").
		For(&deckhousev1alpha2.Instance{}).
		Watches(&capiv1beta2.Machine{}, handler.EnqueueRequestsFromMapFunc(mapObjectNameToInstance)).
		Watches(&mcmv1alpha1.Machine{}, handler.EnqueueRequestsFromMapFunc(mapObjectNameToInstance)).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(mapObjectNameToInstance),
			builder.WithPredicates(staticNodeEventPredicate()),
		).
		Complete(r)
}

func wrapReconcileError(err error) (ctrl.Result, error) {
	if apierrors.IsConflict(err) {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, err
}

func (r *InstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("instance", req.Name)
	log.V(4).Info("tick", "op", "instance.reconcile.start")

	instance := &deckhousev1alpha2.Instance{}

	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	for _, step := range []reconcileStep{
		// refresh bashible heartbeat condition by timeout rules
		r.reconcileInstanceHeartbeat,
		// derive bashible status and message from current conditions
		r.reconcileInstanceBashibleStatus,
		// handle deleting instance and run finalization flow
		r.reconcileInstanceDeletion,
		// ensure controller finalizer exists on active object
		r.reconcileInstanceEnsureFinalizer,
		// remove instance when both linked sources are confirmed missing
		r.reconcileInstanceSourceExistence,
	} {
		done, result, err := step(ctx, instance)
		if err != nil {
			return wrapReconcileError(err)
		}
		if !done {
			continue
		}
		if result != (ctrl.Result{}) {
			log.V(1).Info("instance reconcile step returned early")
			return result, nil
		}

		break
	}

	log.V(1).Info("instance reconciled")
	return ctrl.Result{RequeueAfter: instanceRequeueInterval}, nil
}

func (r *InstanceReconciler) reconcileInstanceEnsureFinalizer(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
) (bool, ctrl.Result, error) {
	ctrl.LoggerFrom(ctx).V(4).Info("tick", "op", "instance.reconcile.active")

	if err := r.ensureInstanceFinalizer(ctx, instance); err != nil {
		return false, ctrl.Result{}, err
	}

	return false, ctrl.Result{}, nil
}

func (r *InstanceReconciler) reconcileInstanceSourceExistence(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
) (bool, ctrl.Result, error) {
	deleted, err := r.reconcileLinkedSourceExistence(ctx, instance)
	if err != nil {
		return false, ctrl.Result{}, err
	}
	if deleted {
		return true, ctrl.Result{}, nil
	}

	return false, ctrl.Result{}, nil
}

func (r *InstanceReconciler) reconcileInstanceHeartbeat(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
) (bool, ctrl.Result, error) {
	if err := r.reconcileBashibleHeartbeat(ctx, instance); err != nil {
		return false, ctrl.Result{}, err
	}

	return false, ctrl.Result{}, nil
}

func (r *InstanceReconciler) reconcileInstanceBashibleStatus(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
) (bool, ctrl.Result, error) {
	if err := r.reconcileBashibleStatus(ctx, instance); err != nil {
		return false, ctrl.Result{}, err
	}

	return false, ctrl.Result{}, nil
}

func (r *InstanceReconciler) reconcileInstanceDeletion(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
) (bool, ctrl.Result, error) {
	isDeleting := instance.DeletionTimestamp != nil && !instance.DeletionTimestamp.IsZero()
	if !isDeleting {
		return false, ctrl.Result{}, nil
	}
	ctrl.LoggerFrom(ctx).V(4).Info("tick", "op", "instance.reconcile.deletion")

	fastRequeue, err := r.reconcileInstanceFinalization(ctx, instance)
	if err != nil {
		return false, ctrl.Result{}, err
	}
	if fastRequeue {
		return true, ctrl.Result{RequeueAfter: time.Second}, nil
	}

	return true, ctrl.Result{RequeueAfter: instanceRequeueInterval}, nil
}
