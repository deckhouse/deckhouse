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
		UpdateFunc: func(event.UpdateEvent) bool {
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

	for _, pendingOperation := range approver.approveQueue {
		canApprove := approver.tryApprove(pendingOperation)

		if canApprove {
			original := pendingOperation.DeepCopy()
			pendingOperation.Spec.Approved = true

			if err := r.client.Patch(ctx, &pendingOperation, client.MergeFrom(original)); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to approve ControlPlaneOperation %q: %w", pendingOperation.Name, err)
			}
		}
	}

	return reconcile.Result{}, nil
}
