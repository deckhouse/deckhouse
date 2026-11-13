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
	"reflect"
	"slices"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

// resyncPeriod relists to catch missed watch events.
const resyncPeriod = 10 * time.Minute

// SelfWatcher watches the own Node by name and keeps its last state.
// It is the only watch on Nodes in the agent.
type SelfWatcher struct {
	factory  informers.SharedInformerFactory
	informer cache.SharedIndexInformer
	nodeName string
	logger   *log.Logger

	mu       sync.Mutex
	signals  domain.NodeSignals
	observed bool
	changed  chan struct{}
}

func NewSelfWatcher(client kubernetes.Interface, nodeName string, logger *log.Logger) (*SelfWatcher, error) {
	selector := fields.OneTermEqualSelector("metadata.name", nodeName).String()

	factory := informers.NewSharedInformerFactoryWithOptions(client, resyncPeriod,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = selector
		}),
	)

	watcher := &SelfWatcher{
		factory:  factory,
		informer: factory.Core().V1().Nodes().Informer(),
		nodeName: nodeName,
		logger:   logger,
		changed:  make(chan struct{}, 1),
	}

	if _, err := watcher.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    watcher.observe,
		UpdateFunc: func(_, newObj any) { watcher.observe(newObj) },
	}); err != nil {
		return nil, fmt.Errorf("add self node event handler: %w", err)
	}

	return watcher, nil
}

// Changed receives a value after the state of the own Node changes. One value
// can stand for many changes. Read the state with Snapshot.
func (w *SelfWatcher) Changed() <-chan struct{} {
	return w.changed
}

// Snapshot returns the last state of the own Node. It returns false until the
// Node is observed for the first time.
func (w *SelfWatcher) Snapshot() (domain.NodeSignals, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	signals := w.signals
	signals.InternalIPs = slices.Clone(w.signals.InternalIPs)
	signals.ExternalIPs = slices.Clone(w.signals.ExternalIPs)
	signals.MaintenanceReasons = slices.Clone(w.signals.MaintenanceReasons)

	return signals, w.observed
}

func (w *SelfWatcher) observe(obj any) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		w.logger.Warn("unexpected object type in the self node informer", "type", fmt.Sprintf("%T", obj))

		return
	}

	// The field selector is a server-side filter. Do not rely on it.
	if node.Name != w.nodeName {
		return
	}

	signals := nodeSignals(node)

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.signals.Maintenance != signals.Maintenance || !w.observed {
		w.logger.Info("own node maintenance state", "maintenance", signals.Maintenance, "annotations", signals.MaintenanceReasons)
	}

	if w.observed && reflect.DeepEqual(w.signals, signals) {
		return
	}

	w.signals, w.observed = signals, true

	select {
	case w.changed <- struct{}{}:
	default:
	}
}

// Run starts the informer and blocks until ctx is cancelled.
func (w *SelfWatcher) Run(ctx context.Context) {
	w.factory.Start(ctx.Done())
	<-ctx.Done()
	w.factory.Shutdown()
}

// WaitForSync blocks until the initial cache fill completes. It returns false when ctx ends.
func (w *SelfWatcher) WaitForSync(ctx context.Context) bool {
	return cache.WaitForCacheSync(ctx.Done(), w.informer.HasSynced)
}

func nodeSignals(node *corev1.Node) domain.NodeSignals {
	signals := domain.NodeSignals{
		UID:       string(node.UID),
		NodeGroup: node.Labels[domain.NodeGroupLabel],
	}

	for _, addr := range node.Status.Addresses {
		switch addr.Type {
		case corev1.NodeInternalIP:
			signals.InternalIPs = append(signals.InternalIPs, addr.Address)
		case corev1.NodeExternalIP:
			signals.ExternalIPs = append(signals.ExternalIPs, addr.Address)
		}
	}

	for _, annotation := range domain.MaintenanceAnnotations() {
		if _, ok := node.Annotations[annotation]; ok {
			signals.Maintenance = true
			signals.MaintenanceReasons = append(signals.MaintenanceReasons, annotation)
		}
	}

	return signals
}
