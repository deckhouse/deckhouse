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

package operationsapprover

import (
	"context"
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
)

var logger = log.NewLogger().Named("operations-approver-controller")

type reconciler struct {
	client client.Client
}

func Register(mgr manager.Manager) error {
	r := &reconciler{
		client: mgr.GetClient(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 3*time.Second),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{
					Limiter: rate.NewLimiter(rate.Limit(1), 1),
				},
			),
		}).
		Named("operations_approver_controller").
		For(
			&controlplanev1alpha1.ControlPlaneOperation{},
			builder.WithPredicates(getPredicates()),
		).
		Complete(r)
}

func getPredicates() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldOperation, okOld := e.ObjectOld.(*controlplanev1alpha1.ControlPlaneOperation)
			newOperation, okNew := e.ObjectNew.(*controlplanev1alpha1.ControlPlaneOperation)

			if !okOld || !okNew {
				return false
			}

			if !oldOperation.IsTerminal() && newOperation.IsTerminal() {
				return true
			}

			return false
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger.Info("Reconcile started")

	nodes := &controlplanev1alpha1.ControlPlaneNodeList{}
	if err := r.client.List(ctx, nodes, &client.ListOptions{}); err != nil {
		return reconcile.Result{}, err
	}

	if len(nodes.Items) == 0 {
		logger.Warn("no control plane nodes found")
		return reconcile.Result{}, nil
	}

	operations := &controlplanev1alpha1.ControlPlaneOperationList{}
	if err := r.client.List(ctx, operations); err != nil {
		return reconcile.Result{}, err
	}

	if len(operations.Items) == 0 {
		logger.Warn("no control plane operations found")
		return reconcile.Result{}, nil
	}

	approver := newApprover(len(nodes.Items), operations.Items)

	for _, unapprovedOperation := range approver.approveQueue {
		canApprove := approver.tryApprove(unapprovedOperation)

		if canApprove {
			original := unapprovedOperation.DeepCopy()
			unapprovedOperation.Spec.Approved = true

			if err := r.client.Patch(ctx, &unapprovedOperation, client.MergeFrom(original)); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to approve ControlPlaneOperation %q: %w", unapprovedOperation.Name, err)
			}
		}
	}

	return reconcile.Result{}, nil
}
