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

	"control-plane-manager/internal/controllers/update-observer/cluster"
	"control-plane-manager/internal/controllers/update-observer/common"
	v1 "control-plane-manager/internal/controllers/update-observer/pkg/v1"
	"control-plane-manager/internal/controllers/update-observer/pkg/version"
)

const (
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
	requeueInterval         = 1 * time.Minute
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
			builder.WithPredicates(getSecretPredicate()),
		).
		Watches(
			&corev1.Node{},
			&handler.Funcs{
				CreateFunc: func(ctx context.Context, e event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				},
				UpdateFunc: func(ctx context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				},
				DeleteFunc: func(ctx context.Context, e event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				},
				GenericFunc: func(ctx context.Context, e event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				},
			}).
		Watches(
			&v1.NodeGroup{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(getNodeGroupPredicate()),
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

func getNodeGroupPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			newNodeGroup, ok1 := e.ObjectNew.(*v1.NodeGroup)
			oldNodeGroup, ok2 := e.ObjectOld.(*v1.NodeGroup)
			if !ok1 || !ok2 {
				return false
			}

			return newNodeGroup.Status.Ready != oldNodeGroup.Status.Ready
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
	klog.Infof("[update-observer] Reconcile started (trigger: %s/%s)", req.Namespace, req.Name)

	configMap, err := r.getConfigMap(ctx)
	if err != nil {
		klog.Errorf("[update-observer] Failed to get configMap %s/%s: %v", common.KubeSystemNamespace, common.ConfigMapName, err)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}
	klog.Infof("[update-observer] ConfigMap fetched (exists=%v)", configMap.ResourceVersion != "")

	clusterCfg, err := r.getClusterConfiguration(ctx)
	if err != nil {
		klog.Errorf("[update-observer] Failed to get cluster configuration from secret %s/%s: %v", common.KubeSystemNamespace, common.SecretName, err)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}
	klog.Infof("[update-observer] Cluster configuration loaded (desiredVersion=%s, updateMode=%s)", clusterCfg.DesiredVersion, clusterCfg.UpdateMode)

	reconcileTrigger := determineReconcileTrigger(configMap, clusterCfg)
	klog.Infof("[update-observer] Reconcile trigger: %s", reconcileTrigger)

	clusterState, err := r.getClusterState(ctx, clusterCfg, configMap.Labels, reconcileTrigger == ReconcileTriggerDowngradeK8s)
	if err != nil {
		klog.Errorf("[update-observer] Failed to get cluster state: %v", err)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}
	klog.Infof("[update-observer] Cluster state computed (phase=%s, currentVersion=%s, progress=%s)", clusterState.Status.Phase, clusterState.CurrentVersion, clusterState.Progress)

	configMap, err = fillConfigMap(configMap, clusterState, reconcileTrigger)
	if err != nil {
		klog.Errorf("[update-observer] Failed to fill configMap: %v", err)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	if err = r.touchConfigMap(ctx, configMap); err != nil {
		klog.Errorf("[update-observer] Failed to write configMap %s/%s: %v", common.KubeSystemNamespace, common.ConfigMapName, err)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}
	klog.Infof("[update-observer] ConfigMap %s/%s successfully written", common.KubeSystemNamespace, common.ConfigMapName)

	if clusterState.Status.Phase != cluster.ClusterUpToDate {
		klog.Infof("[update-observer] Cluster not up-to-date, requeuing after %s", requeueInterval)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	klog.Infof("[update-observer] Reconcile complete, cluster is up-to-date")
	return reconcile.Result{}, nil
}

func determineReconcileTrigger(configMap *corev1.ConfigMap, clusterCfg *cluster.Configuration) ReconcileTrigger {
	previousVersion, exists := configMap.GetLabels()[common.K8sVersionLabelKey]

	if configMap.ResourceVersion == "" || !exists {
		return ReconcileTriggerInit
	}

	switch version.Compare(previousVersion, clusterCfg.DesiredVersion) {
	case 1:
		return ReconcileTriggerDowngradeK8s
	case 0:
		return ReconcileTriggerIdle
	case -1:
		return ReconcileTriggerUpgradeK8s
	}

	return ReconcileTriggerInit
}
