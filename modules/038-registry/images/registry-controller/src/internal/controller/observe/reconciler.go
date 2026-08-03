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

// Package observe turns the state of the resources into metrics.
//
// A controller and not a module hook, and one exporter and not several. The resources are
// where the state of this module lives — which upstream is in effect, whether the cache is
// complete, what each node applied — and a controller already watches all of them. A hook
// would poll the same objects on its own schedule and reach the same conclusions a beat
// later, which is how two answers to one question appear.
//
// It is also the only way some of this state can be reported at all. The node agent cannot
// be scraped: it is a static pod that has to work when the API server does not, so it can
// carry no kube-rbac-proxy — that sidecar authenticates against the very API server the
// agent is designed to outlive. What the agent knows therefore reaches Prometheus by way
// of the resource it already writes.
//
// This reconciler converges nothing. It runs on every replica rather than only on the one
// holding the lease, so that the picture of the cluster does not depend on the health of a
// single pod: the thing reporting a problem should not be the thing most likely to have
// one. Duplicate series are the price, and the alerts aggregate over the pod label.
package observe

import (
	"context"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/metrics"
	"github.com/deckhouse/registry-controller/internal/register"
)

// ControllerName identifies this reconciler in --disable-controllers.
const ControllerName = "observe"

// resyncInterval re-reads the state even when nothing changed.
//
// Needed because the gauges live in a process that can be restarted or rescheduled: after
// a restart nothing has changed, so nothing would trigger a reconciliation, and the
// exporter would serve zeros until something in the cluster happened to move.
const resyncInterval = time.Minute

func init() {
	register.RegisterController(ControllerName, &registryv1alpha1.RegistryConfig{}, &Reconciler{})
}

// Reconciler exports the state of the registry resources.
type Reconciler struct {
	register.Base
}

// SkipLeaderElection: this converges nothing, and reporting must not stop because another
// replica holds the lease.
func (r *Reconciler) SkipLeaderElection() bool { return true }

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// Everything funnels to the singleton, because the export is a whole picture rather
	// than a per-object fact: a node's metric is only meaningful next to the
	// configuration that produced it.
	toSingleton := handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{Name: registryv1alpha1.SingletonName},
		}}
	})

	w.Watches(&registryv1alpha1.RegistryStorage{}, toSingleton)
	w.Watches(&registryv1alpha1.RegistryNode{}, toSingleton)
	w.Watches(&registryv1alpha1.RegistryUpstream{}, toSingleton)
}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	// Errors are not returned. A failure to read state is a failure to describe the
	// cluster, not a failure of the cluster, and turning it into a retry storm would make
	// the observer the loudest thing in the log at the moment an operator needs the log.
	if err := r.observeConfig(ctx); err != nil {
		ctrl.LoggerFrom(ctx).Info("cannot read the registry configuration for metrics", "error", err.Error())
	}
	if err := r.observeStorage(ctx); err != nil {
		ctrl.LoggerFrom(ctx).Info("cannot read the registry storage for metrics", "error", err.Error())
	}
	if err := r.observeNodes(ctx); err != nil {
		ctrl.LoggerFrom(ctx).Info("cannot read the node layouts for metrics", "error", err.Error())
	}

	return ctrl.Result{RequeueAfter: resyncInterval}, nil
}

func (r *Reconciler) observeConfig(ctx context.Context) error {
	config := &registryv1alpha1.RegistryConfig{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, config)
	if err != nil {
		metrics.ConfigValid.Set(0)
		return client.IgnoreNotFound(err)
	}

	metrics.ConfigValid.Set(boolean(isTrue(config.Status.Conditions, registryv1alpha1.ConditionValid)))
	metrics.Managed.Set(boolean(config.Spec.Mode == registryv1alpha1.ModeManaged))

	// Which upstream is actually in effect, which is not necessarily the configured one:
	// a failed preflight probe holds the cluster on the last one that worked, and that
	// difference is invisible in the spec.
	metrics.EffectiveUpstream.Reset()
	if effective := config.Status.EffectiveUpstream; effective != nil {
		metrics.EffectiveUpstream.WithLabelValues(effective.Host).Set(1)
	}

	return nil
}

func (r *Reconciler) observeStorage(ctx context.Context) error {
	storage := &registryv1alpha1.RegistryStorage{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage)
	if err != nil {
		// No cache configured, so there is nothing to describe. The gauges are cleared
		// rather than left at their last value, which would keep reporting a storage that
		// no longer exists.
		metrics.StorageReplicaFull.Reset()
		metrics.StorageStaleData.Reset()
		metrics.StorageReplicas.Set(0)
		metrics.StorageSafeToDropUpstream.Set(0)
		metrics.StorageAllReplicasFull.Set(0)
		return client.IgnoreNotFound(err)
	}

	metrics.StorageReplicas.Set(float64(len(storage.Status.Replicas)))
	metrics.StorageSafeToDropUpstream.Set(boolean(storage.Status.SafeToDropUpstream))
	metrics.StorageAllReplicasFull.Set(boolean(storage.Status.AllReplicasFull))

	metrics.StorageReplicaFull.Reset()
	for i := range storage.Status.Replicas {
		replica := &storage.Status.Replicas[i]
		metrics.StorageReplicaFull.
			WithLabelValues(replica.Node, string(replica.Role)).
			Set(boolean(replica.Full && replica.Error == ""))
	}

	return nil
}

func (r *Reconciler) observeNodes(ctx context.Context) error {
	list := &registryv1alpha1.RegistryNodeList{}
	if err := r.Client.List(ctx, list); err != nil {
		return err
	}

	metrics.NodeReconciled.Reset()
	metrics.NodeFromCache.Reset()
	metrics.StorageStaleData.Reset()

	for i := range list.Items {
		node := &list.Items[i]
		status := &node.Status

		metrics.NodeReconciled.WithLabelValues(node.Name).
			Set(boolean(status.Reconciled && status.ObservedGeneration == node.Generation))

		// Whether the agent is routing from a copy on the node rather than from the
		// cluster's answer. The node pulls images either way, which is the point of the
		// copy — and also why nothing else would ever mention it.
		metrics.NodeFromCache.WithLabelValues(node.Name).
			Set(boolean(hasReason(status.Conditions, registryv1alpha1.ConditionReconciled,
				registryv1alpha1.ReasonAppliedFromCache)))

		// Cache data left on a node that no longer runs a cache. Kept deliberately, so
		// that turning the cache back on refills from what is already there — but kept
		// silently it would be a disk nobody accounts for.
		if status.StaleStorageDataBytes > 0 {
			metrics.StorageStaleData.WithLabelValues(node.Name).
				Set(float64(status.StaleStorageDataBytes))
		}
	}

	metrics.Nodes.Set(float64(len(list.Items)))
	return nil
}

func isTrue(conditions []metav1.Condition, name string) bool {
	condition := apimeta.FindStatusCondition(conditions, name)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

func hasReason(conditions []metav1.Condition, name, reason string) bool {
	condition := apimeta.FindStatusCondition(conditions, name)
	return condition != nil && condition.Reason == reason
}

func boolean(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
