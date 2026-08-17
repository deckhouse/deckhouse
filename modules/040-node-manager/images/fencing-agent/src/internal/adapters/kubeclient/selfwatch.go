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

package kubeclient

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

type SelfStore interface {
	Observe(signals domain.NodeSignals)
	Deleted()
}

type SelfWatcher struct {
	factory  informers.SharedInformerFactory
	informer cache.SharedIndexInformer
	store    SelfStore
	nodeName string
	logger   *log.Logger
}

func NewSelfWatcher(client kubernetes.Interface, nodeName string, store SelfStore, logger *log.Logger) (*SelfWatcher, error) {
	selector := fields.OneTermEqualSelector("metadata.name", nodeName).String()

	factory := informers.NewSharedInformerFactoryWithOptions(client, resyncPeriod,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = selector
		}),
	)

	watcher := &SelfWatcher{
		factory:  factory,
		informer: factory.Core().V1().Nodes().Informer(),
		store:    store,
		nodeName: nodeName,
		logger:   logger,
	}

	if _, err := watcher.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    watcher.observe,
		UpdateFunc: func(_, newObj any) { watcher.observe(newObj) },
		DeleteFunc: watcher.deleted,
	}); err != nil {
		return nil, fmt.Errorf("add self node event handler: %w", err)
	}

	return watcher, nil
}

func (w *SelfWatcher) observe(obj any) {
	node, ok := w.node(obj)
	if !ok {
		return
	}

	w.store.Observe(nodeSignals(node))
}

func (w *SelfWatcher) deleted(obj any) {
	// A missed delete is delivered as a tombstone on relist.
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	if _, ok := w.node(obj); !ok {
		return
	}

	w.store.Deleted()
}

// node casts and re-checks the name: the field selector is a server-side filter,
// and a watchdog decision must not depend on the API honouring it.
func (w *SelfWatcher) node(obj any) (*corev1.Node, bool) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		w.logger.Warn("unexpected object type in the self node informer", "type", fmt.Sprintf("%T", obj))

		return nil, false
	}

	if node.Name != w.nodeName {
		return nil, false
	}

	return node, true
}

// Run starts the informer and blocks until ctx is cancelled.
func (w *SelfWatcher) Run(ctx context.Context) {
	w.factory.Start(ctx.Done())
	<-ctx.Done()
	w.factory.Shutdown()
}

// WaitForSync blocks until the initial cache fill completes; a false return
// means ctx ended. Arming the watchdog before this point would hide maintenance
// annotations, so the caller must wait for it.
func (w *SelfWatcher) WaitForSync(ctx context.Context) bool {
	return cache.WaitForCacheSync(ctx.Done(), w.informer.HasSynced)
}

func nodeSignals(node *corev1.Node) domain.NodeSignals {
	annotations := domain.MaintenanceAnnotations()

	signals := domain.NodeSignals{
		UID:                string(node.UID),
		MaintenanceReasons: make([]string, 0, len(annotations)),
	}

	for _, annotation := range annotations {
		if _, ok := node.Annotations[annotation]; ok {
			signals.Maintenance = true
			signals.MaintenanceReasons = append(signals.MaintenanceReasons, annotation)
		}
	}

	if node.DeletionTimestamp != nil {
		signals.PlannedRemoval = true
		signals.RemovalReason = domain.RemovalReasonDeleting
	}

	for _, taint := range node.Spec.Taints {
		if taint.Key == domain.ClusterAutoscalerDeleteTaint {
			signals.PlannedRemoval = true
			signals.RemovalReason = domain.RemovalReasonAutoscaler
		}
	}

	return signals
}
