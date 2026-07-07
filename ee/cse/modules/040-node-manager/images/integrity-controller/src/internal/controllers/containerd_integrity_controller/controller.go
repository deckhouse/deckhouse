/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package containerdintegritycontroller

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"
)

const (
	name                    = "containerd-integrity-controller"
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
		For(&deckhousev1alpha1.ContainerdIntegrityPolicy{}).
		Watches(
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(r.mapNamespaceToContainerdIntegrityPolicies),
			builder.WithPredicates(namespacePredicate()),
		).
		Complete(r)
}
