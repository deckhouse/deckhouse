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

package virtualcontrolplaneapprover

import (
	"context"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	operationsapprover "control-plane-manager/internal/controllers/operations-approver"
)

const (
	maxConcurrentReconciles = 10
	cacheSyncTimeout        = 3 * time.Minute
	approverRequestName     = "approver"
)

func rateLimiter() workqueue.TypedRateLimiter[reconcile.Request] {
	return workqueue.NewTypedMaxOfRateLimiter(
		workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 3*time.Second),
		&workqueue.TypedBucketRateLimiter[reconcile.Request]{
			Limiter: rate.NewLimiter(rate.Limit(maxConcurrentReconciles), maxConcurrentReconciles),
		},
	)
}

func mapToNamespace(_ context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      approverRequestName,
		},
	}}
}

func BuildController(mgr manager.Manager) error {
	r := newReconciler(mgr.GetClient())

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      ptr.To(true),
			RateLimiter:             rateLimiter(),
		}).
		Named(constants.VirtualControlPlaneApproverControllerName).
		Watches(
			&controlplanev1alpha1.ControlPlaneOperation{},
			handler.EnqueueRequestsFromMapFunc(mapToNamespace),
			builder.WithPredicates(operationsapprover.OperationPredicate()),
		).
		Complete(r)
}
