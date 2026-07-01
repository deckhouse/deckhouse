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

package virtualcontrolplaneconfiguration

import (
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	name                    = "virtual-control-plane-configuration-controller"
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
)

func BuildController(mgr manager.Manager) error {
	r := &reconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      ptr.To(true),
			RateLimiter: workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](
				5*time.Second,
				5*time.Minute,
			),
		}).
		Named(name).
		For(&controlplanev1alpha1.VirtualControlPlane{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.mapConfigSecretToVirtualControlPlanes),
			builder.WithPredicates(secretPredicate()),
		).
		Complete(r)
}
