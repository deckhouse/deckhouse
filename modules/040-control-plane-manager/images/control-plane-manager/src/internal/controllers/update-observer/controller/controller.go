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
}

func RegisterController(mgr manager.Manager) error {
	r := &reconciler{
		client: mgr.GetClient(),
	}

	// The watch alone does not cover a cluster where the ConfigMap does not exist at the moment
	// this Pod starts: an informer replays Create events for objects that are there, and delivers
	// nothing at all for one that is not. Without this first enqueue the controller would sit idle
	// until some unrelated NodeGroup event happened to wake it.
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
		Complete(r)
}

// bootstrapConfigMap makes sure the cluster ConfigMap exists, and does nothing beyond that.
//
// It answers the one case the watch cannot: an informer replays Create events for objects that
// already exist when it starts, but delivers nothing for an object that does not exist at all. If
// the ConfigMap is present this returns on the first check and the watch takes over; if it is
// absent this reconciles until it is there.
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

		logger.Info("Cluster ConfigMap does not exist, creating it",
			"namespace", common.KubeSystemNamespace, "name", common.ConfigMapName)
		// Reconcile reports its own failures and never returns an error, so the next tick is what
		// decides whether this worked: it re-checks existence rather than trusting the call.
		if _, reconcileErr := r.Reconcile(ctx, reconcile.Request{}); reconcileErr != nil {
			logger.Warn("Bootstrap reconcile failed, retrying", log.Err(reconcileErr))
		}

		return false, nil
	})

	// A cancelled context is a shutdown, not a failure — returning it would make the manager log
	// this runnable as having errored on every graceful stop.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil
	}

	return err
}

// getConfigMapSpecPredicate reacts to the ConfigMap this controller owns, but only when its
// data["spec"] block actually changed. This controller now writes that block itself, so the
// filter's job is no longer to separate an external writer's changes from its own — it is to keep
// the write from feeding back: fillConfigMap stamps lastReconciliationTime on every pass, so any
// broader comparison would spin forever.
func getConfigMapSpecPredicate() predicate.Predicate {
	parseSpec := func(cm *corev1.ConfigMap) Spec {
		var spec Spec
		_ = yaml.Unmarshal([]byte(cm.Data["spec"]), &spec)
		return spec
	}

	isTarget := func(cm *corev1.ConfigMap) bool {
		// Namespace is checked too so correctness does not depend on the cache scoping in
		// manager.go: a widened informer must not turn a same-named ConfigMap elsewhere into a
		// trigger for the singleton this controller owns.
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
			// Comparing data.spec alone is what keeps this controller from re-triggering itself:
			// fillConfigMap stamps lastReconciliationTime on every pass, so any broader comparison
			// would spin forever.
			//
			// The status escape hatch is separate: once the cluster is UpToDate, Reconcile returns
			// without a requeue, so data.spec is the only remaining wake-up. If status is then
			// wiped externally, nothing would ever restore it. Reacting to a *missing* status (not
			// to its contents) re-arms recovery without reintroducing the self-trigger loop.
			if newCM.Data["spec"] != "" && newCM.Data["status"] == "" {
				return true
			}
			return parseSpec(oldCM) != parseSpec(newCM)
		},

		// Deletion has to wake the controller up: this object is the only durable record of
		// maxUsedKubernetesVersion, and a `kubectl delete cm` that went through (the admission
		// webhook that forbids it can be unavailable) must be followed by a recreate rather than
		// silence until the next unrelated event.
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

	// TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
	logger.Info("E2E-KV observer",
		"spec_desired", clusterCfg.DesiredVersion,
		"spec_mode", string(clusterCfg.UpdateMode),
		"spec_max_used", clusterCfg.MaxUsedVersion,
		"status_current", clusterState.CurrentVersion,
		"reconcile_trigger", string(reconcileTrigger),
	)

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

	// A missing k8s-version label is the real first-run signal: dhctl seeds the ConfigMap at
	// bootstrap with labels and data.spec only, so this controller's first pass over it has no
	// version label yet. ResourceVersion cannot serve as that signal — the seeded object arrives
	// with a non-empty one.
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
