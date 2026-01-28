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

package controller

import (
	"context"
	"time"
	"update-observer/cluster"
	"update-observer/common"

	"golang.org/x/mod/semver"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
	requeueInterval         = 1 * time.Minute
	cronRequeueInterval     = 30 * time.Minute
	nodeListPageSize        = 50
)

type ReconcileTrigger string

const (
	ReconcileTriggerInit         ReconcileTrigger = "init"
	ReconcileTriggerUpgradeK8s   ReconcileTrigger = "upgradeK8s"
	ReconcileTriggerDowngradeK8s ReconcileTrigger = "downgradeK8s"
	ReconcileTriggerIdle         ReconcileTrigger = "idle"
)

type reconciler struct {
	client client.Client
}

func RegisterController(manager manager.Manager) error {
	r := &reconciler{
		client: manager.GetClient(),
	}

	return ctrl.NewControllerManagedBy(manager).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      ptr.To(true),
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 3*time.Second),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{
					Limiter: rate.NewLimiter(rate.Limit(1), 1),
				},
			),
		}).
		Named(common.ControllerName).
		Watches(
			&corev1.Secret{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(
				getSecretPredicate(),
			),
		).
		Complete(r)
}

func getSecretPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			secret, ok := e.Object.(*corev1.Secret)
			if !ok {
				return false
			}
			return secret.Name == common.SecretName
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			secret, ok := e.ObjectNew.(*corev1.Secret)
			if !ok {
				return false
			}
			return secret.Name == common.SecretName
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},

		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	klog.Info("Reconcile started")

	configMap, err := r.getConfigMap(ctx)
	if err != nil {
		klog.Error("Failed to get configMap", err)
		return reconcile.Result{}, err
	}

	clusterCfg, err := r.getClusterConfiguration(ctx)
	if err != nil {
		klog.Info("Error occurred while getting cluster configuration", err)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	reconcileTrigger := determineReconcileTrigger(configMap, clusterCfg)

	clusterState, err := r.getClusterState(ctx, clusterCfg, reconcileTrigger == ReconcileTriggerDowngradeK8s)
	if err != nil {
		klog.Error("Error encountered while getting cluster state", err)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	configMap, err = fillConfigMap(configMap, clusterState, reconcileTrigger)
	if err != nil {
		klog.Error("Failed to fill configMap", err)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	if err = r.touchConfigMap(ctx, configMap); err != nil {
		klog.Error("Failed to touch configMap", err)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	if clusterState.Status.Phase != cluster.ClusterUpToDate {
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	return reconcile.Result{RequeueAfter: cronRequeueInterval}, nil
}

func determineReconcileTrigger(configMap *corev1.ConfigMap, clusterCfg *cluster.Configuration) ReconcileTrigger {
	previousVersion, exists := configMap.GetLabels()[common.K8sVersionLabelKey]

	if configMap.ResourceVersion == "" || !exists {
		return ReconcileTriggerInit
	}

	switch semver.Compare(previousVersion, clusterCfg.DesiredVersion) {
	case 1:
		return ReconcileTriggerDowngradeK8s
	case 0:
		return ReconcileTriggerIdle
	case -1:
		return ReconcileTriggerUpgradeK8s
	}

	return ReconcileTriggerInit
}
