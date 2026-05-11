// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// indexName is the name under which the per-package indexer is
// registered on each informer's cache. It is private because callers list
// workloads through the Service, not through the indexer directly.
const indexName = "package"

// workloadKind is a stable map key for per-kind state (indexers, sync funcs).
// It is intentionally a string so the value can be used directly in log
// fields and error messages without a separate conversion.
type workloadKind string

const (
	kindDeployment  workloadKind = "Deployment"
	kindStatefulSet workloadKind = "StatefulSet"
)

// Monitor watches Deployments and StatefulSets carrying a configurable
// package label and emits a per-package []WorkloadStatus on every change
// via the Reconcile callback. The monitor is reduction-agnostic — it does
// not know about Health, transitions, or subscribers. That belongs one
// layer up in the health package.
type Monitor struct {
	factory  informers.SharedInformerFactory
	indexers map[workloadKind]cache.Indexer
	syncs    map[workloadKind]cache.InformerSynced
	queue    workqueue.TypedRateLimitingInterface[string]

	reconcile Reconcile
	labelKey  string

	logger *log.Logger

	// once guards Run against accidental re-entry.
	once sync.Once
	// cancel is set by Run (under once) and called by Stop.
	cancel context.CancelFunc
	// done is closed when the worker goroutine has fully exited; Stop
	// blocks on it to provide synchronous shutdown.
	done chan struct{}
}

// Reconcile is invoked from the monitor's worker goroutine with every
// known WorkloadStatus for the given package.
type Reconcile func(name string, status []WorkloadStatus)

// NewMonitor constructs a Monitor. It wires up the shared informer
// factory, the typed informers, and the per-package indexer, but does
// not start any goroutines or perform any I/O; that happens in Start.
// All three arguments are required.
//
// The factory installs a server-side label selector on labelKey so the
// API server only sends workloads that carry the package label. This
// matches what the local indexer requires anyway and avoids caching every
// Deployment/StatefulSet in the cluster.
func NewMonitor(client kubernetes.Interface, reconcile Reconcile, labelKey string, logger *log.Logger) (*Monitor, error) {
	// Resync period is 0: the watch is authoritative and we have nothing
	// useful to do on a periodic full re-list.
	s := &Monitor{
		factory: informers.NewSharedInformerFactoryWithOptions(client, 0,
			informers.WithTransform(stripUnusedFields),
			informers.WithTweakListOptions(func(o *metav1.ListOptions) {
				o.LabelSelector = labelKey
			}),
		),
		indexers: make(map[workloadKind]cache.Indexer, 2),
		syncs:    make(map[workloadKind]cache.InformerSynced, 2),
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "health"},
		),

		reconcile: reconcile,
		labelKey:  labelKey,
		logger:    logger,
		done:      make(chan struct{}),
	}

	if err := s.registerInformer(kindDeployment, s.factory.Apps().V1().Deployments().Informer()); err != nil {
		return nil, fmt.Errorf("register Deployment informer: %w", err)
	}
	if err := s.registerInformer(kindStatefulSet, s.factory.Apps().V1().StatefulSets().Informer()); err != nil {
		return nil, fmt.Errorf("register StatefulSet informer: %w", err)
	}

	return s, nil
}

// registerInformer attaches the package indexer and event handlers to a
// SharedIndexInformer. Both must be installed before factory.Start so that
// the initial list is indexed and dispatched correctly.
func (m *Monitor) registerInformer(kind workloadKind, inf cache.SharedIndexInformer) error {
	index := cache.Indexers{
		indexName: m.packageIndexFunc,
	}

	if err := inf.AddIndexers(index); err != nil {
		return fmt.Errorf("add '%s' indexer: %w", kind, err)
	}

	if _, err := inf.AddEventHandler(m.eventHandler()); err != nil {
		return fmt.Errorf("add '%s' event handler: %w", kind, err)
	}

	m.indexers[kind] = inf.GetIndexer()
	m.syncs[kind] = inf.HasSynced

	return nil
}

// packageIndexFunc returns the package label value for an object, or an
// empty slice if the object has no package label. Objects without the
// label are simply not indexed under any package and can therefore never
// reach reconcile via this indexer.
func (m *Monitor) packageIndexFunc(obj any) ([]string, error) {
	meta, err := metaFor(obj)
	if err != nil || meta == nil {
		return nil, err
	}

	v, ok := meta.GetLabels()[m.labelKey]
	if !ok || v == "" {
		return nil, nil
	}

	return []string{v}, nil
}

// metaFor returns the ObjectMeta of an informer-cache object, transparently
// unwrapping cache.DeletedFinalStateUnknown tombstones that the cache
// constructs when it detects a delete via a relist. Returning (nil, nil)
// for a nil input is intentional: callers can treat "no metadata" the same
// as "no package label."
func metaFor(obj any) (metav1.Object, error) {
	if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = d.Obj
	}

	if obj == nil {
		return nil, nil
	}

	m, ok := obj.(metav1.Object)
	if !ok {
		return nil, fmt.Errorf("object %T does not implement metav1.Object", obj)
	}

	return m, nil
}

// eventHandler returns the cache event handler attached to every informer.
// It enqueues the package label value, so events for workloads without
// the label are dropped before they reach the workqueue. The workqueue
// then coalesces bursts of events for the same package into a single
// reconcile. The same handler works for all workload kinds because the
// downstream collect() step lists by package across every indexer.
//
// UpdateFunc enqueues both the old and new package labels: when a
// workload is relabeled from package A to package B, both packages must
// be re-reconciled so A drops the workload and B picks it up. The
// workqueue dedupes when the labels are equal, so the double enqueue is
// free in the common case.
func (m *Monitor) eventHandler() cache.ResourceEventHandlerFuncs {
	enqueue := func(obj any) {
		meta, err := metaFor(obj)
		if err != nil || meta == nil {
			return
		}

		if pkg := meta.GetLabels()[m.labelKey]; pkg != "" {
			m.queue.Add(pkg)
		}
	}

	return cache.ResourceEventHandlerFuncs{
		AddFunc: enqueue,
		UpdateFunc: func(oldObj, newObj any) {
			enqueue(oldObj)
			enqueue(newObj)
		},
		DeleteFunc: enqueue,
	}
}

// stripUnusedFields drops fields the reducer never reads (PodTemplateSpec,
// ManagedFields, VolumeClaimTemplates) before objects enter the cache.
// This shrinks the cache footprint substantially without changing any
// reducer-visible behavior. Status is preserved; the reducer reads it
// directly.
func stripUnusedFields(obj any) (any, error) {
	switch o := obj.(type) {
	case *appsv1.Deployment:
		o.ManagedFields = nil
		o.Spec.Template = corev1.PodTemplateSpec{}
	case *appsv1.StatefulSet:
		o.ManagedFields = nil
		o.Spec.Template = corev1.PodTemplateSpec{}
		o.Spec.VolumeClaimTemplates = nil
	}

	return obj, nil
}

// Start launches the monitor's background goroutine and returns
// immediately. Safe to call more than once; only the first call has any
// effect. Pair every Start with a Stop.
func (m *Monitor) Start() {
	m.once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel
		go m.run(ctx)
	})
}

// Stop cancels the internal context and waits for the worker goroutine
// to fully exit (caches stopped, queue drained, in-flight reconciles
// finished). A no-op if Start was never called.
func (m *Monitor) Stop() {
	if m.cancel == nil {
		return
	}
	m.cancel()
	<-m.done
}

// run owns the lifecycle of the worker goroutine. The defers run in
// LIFO order, so close(done) fires last — after the queue is fully shut
// down — which is what makes Done a reliable "completely stopped" signal.
//
// The ctx-watcher goroutine is the load-bearing piece: workqueue.Get
// does not respect ctx, so without an explicit ShutDown call, a worker
// blocked in Get would never see ctx cancellation and run() would
// deadlock. ShutDown is idempotent, so the deferred call below is safe.
//
// A failed cache sync is treated as a fatal startup error: log and
// return rather than serve stale or empty results to reconcile.
func (m *Monitor) run(ctx context.Context) {
	defer close(m.done)
	defer m.queue.ShutDown()

	m.factory.Start(ctx.Done())

	synced := make([]cache.InformerSynced, 0, len(m.syncs))
	for _, fn := range m.syncs {
		synced = append(synced, fn)
	}

	if !cache.WaitForCacheSync(ctx.Done(), synced...) {
		m.logger.Warn("timed out waiting for caches to sync")
		return
	}
	m.logger.Info("caches synced")

	go func() {
		<-ctx.Done()
		m.queue.ShutDown()
	}()

	wait.UntilWithContext(ctx, m.runWorker, time.Second)
}
