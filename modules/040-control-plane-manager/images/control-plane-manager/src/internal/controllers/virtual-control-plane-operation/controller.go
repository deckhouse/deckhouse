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

package virtualcontrolplaneoperation

import (
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/operations/ephemeral"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	name                    = "virtual-control-plane-operation-controller"
	maxConcurrentReconciles = 10
	cacheSyncTimeout        = 3 * time.Minute
)

func BuildController(mgr manager.Manager) error {
	r := &reconciler{
		client:            mgr.GetClient(),
		operationExecutor: ephemeral.NewEphemeralExecutor(mgr.GetClient()),
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      ptr.To(true),
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](5*time.Second, 5*time.Minute),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{
					Limiter: rate.NewLimiter(rate.Limit(1), 1),
				},
			),
		}).
		Named(name).
		For(
			&controlplanev1alpha1.ControlPlaneOperation{},
			builder.WithPredicates(approvedOperationPredicate()),
		).
		Complete(r)
}
