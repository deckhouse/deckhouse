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
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.yaml.in/yaml/v2"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
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
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/deckhouse/deckhouse/pkg/log"

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

var logger = log.NewLogger().Named("update-observer")

type reconciler struct {
	client client.Client

	// Carries the first enqueue when the ConfigMap does not exist yet; buffered so the sender never
	// blocks.
	bootstrap chan event.GenericEvent
}

func RegisterController(mgr manager.Manager) error {
	r := &reconciler{
		client:    mgr.GetClient(),
		bootstrap: make(chan event.GenericEvent, 1),
	}

	// An informer delivers nothing for an object that does not exist, so without this a missing
	// ConfigMap leaves the controller idle.
	if err := mgr.Add(manager.RunnableFunc(r.bootstrapConfigMap)); err != nil {
		return fmt.Errorf("add the ConfigMap bootstrap runnable: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
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
			&corev1.ConfigMap{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(getConfigMapSpecPredicate()),
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
		// Fed by bootstrapConfigMap, so its wake-up goes through the same workqueue as everything else.
		WatchesRawSource(source.Channel(r.bootstrap, &handler.EnqueueRequestForObject{})).
		Complete(r)
}

// Returns on the first check when the ConfigMap is there, and otherwise asks for a reconcile until
// it is. It *asks* rather than calls because MaxConcurrentReconciles is 1: a direct call could run
// alongside a queued pass and both would Create the object.
func (r *reconciler) bootstrapConfigMap(ctx context.Context) error {
	err := wait.PollUntilContextCancel(ctx, requeueInterval, true, func(ctx context.Context) (bool, error) {
		getErr := r.client.Get(ctx, client.ObjectKey{
			Name:      common.ConfigMapName,
			Namespace: common.KubeSystemNamespace,
		}, &corev1.ConfigMap{})
		if getErr == nil {
			return true, nil
		}

		if !apierrors.IsNotFound(getErr) {
			logger.Warn("Cannot check whether the cluster ConfigMap exists, retrying",
				slog.String("namespace", common.KubeSystemNamespace), slog.String("name", common.ConfigMapName), log.Err(getErr))
			return false, nil
		}

		logger.Info("Cluster ConfigMap does not exist, requesting a reconcile to create it",
			"namespace", common.KubeSystemNamespace, "name", common.ConfigMapName)
		// Non-blocking: the next tick re-checks existence rather than trusting this send.
		select {
		case r.bootstrap <- event.GenericEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      common.ConfigMapName,
			Namespace: common.KubeSystemNamespace,
		}}}:
		default:
		}

		return false, nil
	})

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil
	}

	return err
}

// Reacts only when data["spec"] changed: this controller writes that block itself, so the filter's
// job is to keep its own write from feeding back.
func getConfigMapSpecPredicate() predicate.Predicate {
	parseSpec := func(cm *corev1.ConfigMap) Spec {
		var spec Spec
		_ = yaml.Unmarshal([]byte(cm.Data["spec"]), &spec)
		return spec
	}

	isTarget := func(cm *corev1.ConfigMap) bool {
		// Namespace too, so correctness does not depend on the cache scoping in manager.go.
		return cm.Name == common.ConfigMapName && cm.Namespace == common.KubeSystemNamespace
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			cm, ok := e.Object.(*corev1.ConfigMap)
			if !ok {
				return false
			}
			return isTarget(cm)
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			newCM, ok1 := e.ObjectNew.(*corev1.ConfigMap)
			oldCM, ok2 := e.ObjectOld.(*corev1.ConfigMap)
			if !ok1 || !ok2 || !isTarget(newCM) {
				return false
			}
			// fillConfigMap stamps lastReconciliationTime on every pass, so anything broader than
			// data.spec would spin forever. Reacting to a *missing* status is the escape hatch: once
			// UpToDate, Reconcile stops requeueing and a wiped status would never be restored.
			if newCM.Data["spec"] != "" && newCM.Data["status"] == "" {
				return true
			}
			return parseSpec(oldCM) != parseSpec(newCM)
		},

		// The only durable record of maxUsedKubernetesVersion: a delete that got past the webhook must
		// be followed by a recreate.
		DeleteFunc: func(e event.DeleteEvent) bool {
			cm, ok := e.Object.(*corev1.ConfigMap)
			if !ok {
				return false
			}
			return isTarget(cm)
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
	configMap, err := r.getConfigMap(ctx)
	if err != nil {
		logger.Error("Failed to get configMap", "namespace", common.KubeSystemNamespace, "name", common.ConfigMapName, log.Err(err))
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	clusterCfg, err := desiredConfiguration(configMap)
	if err != nil {
		logger.Error("Failed to build the desired cluster configuration from the environment", log.Err(err))
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	reconcileTrigger := determineReconcileTrigger(configMap, clusterCfg)

	clusterState, err := r.getClusterState(ctx, clusterCfg, configMap.Labels, reconcileTrigger == ReconcileTriggerDowngradeK8s)
	if err != nil {
		logger.Error("Failed to get cluster state", log.Err(err))
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	configMap, err = fillConfigMap(configMap, clusterState, reconcileTrigger)
	if err != nil {
		logger.Error("Failed to fill configMap", log.Err(err))
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	if err = r.touchConfigMap(ctx, configMap); err != nil {
		logger.Error("Failed to write configMap", "namespace", common.KubeSystemNamespace, "name", common.ConfigMapName, log.Err(err))
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	if clusterState.Status.Phase != cluster.ClusterUpToDate {
		logger.Info("Cluster not up-to-date, requeuing", "after", requeueInterval)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	return reconcile.Result{}, nil
}

func determineReconcileTrigger(configMap *corev1.ConfigMap, clusterCfg *cluster.Configuration) ReconcileTrigger {
	previousVersion, exists := configMap.GetLabels()[common.K8sVersionLabelKey]

	// A missing k8s-version label is the first-run signal: dhctl seeds labels and data.spec only, and
	// ResourceVersion is already non-empty on that object.
	if !exists {
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
