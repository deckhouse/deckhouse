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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

// resyncPeriod relists periodically as a guard against missed watch events.
const resyncPeriod = 10 * time.Minute

// PeerStore receives the expected membership of the NodeGroup.
type PeerStore interface {
	Upsert(peer domain.Peer)
	Delete(name string)
}

// NodeWatcher feeds Node add/update/delete events of one NodeGroup into a
// PeerStore through a label-filtered shared informer. When the API becomes
// unreachable the cache freezes on the last known state, which keeps the
// membership view local, as the fencing fast path requires.
type NodeWatcher struct {
	factory  informers.SharedInformerFactory
	informer cache.SharedIndexInformer
	store    PeerStore
	logger   *log.Logger
}

func NewNodeWatcher(client kubernetes.Interface, nodeGroup string, store PeerStore, logger *log.Logger) (*NodeWatcher, error) {
	selector := labels.SelectorFromSet(labels.Set{domain.NodeGroupLabel: nodeGroup}).String()

	factory := informers.NewSharedInformerFactoryWithOptions(client, resyncPeriod,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = selector
		}),
	)

	watcher := &NodeWatcher{
		factory:  factory,
		informer: factory.Core().V1().Nodes().Informer(),
		store:    store,
		logger:   logger,
	}

	if _, err := watcher.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    watcher.upsert,
		UpdateFunc: func(_, newObj any) { watcher.upsert(newObj) },
		DeleteFunc: watcher.delete,
	}); err != nil {
		return nil, fmt.Errorf("add node event handler: %w", err)
	}

	return watcher, nil
}

func (w *NodeWatcher) upsert(obj any) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		w.logger.Warn("unexpected object type in the node informer", "type", fmt.Sprintf("%T", obj))

		return
	}

	w.store.Upsert(domain.Peer{Name: node.Name, IP: internalIP(node)})
}

func (w *NodeWatcher) delete(obj any) {
	// A missed delete is delivered as a tombstone on relist.
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	node, ok := obj.(*corev1.Node)
	if !ok {
		w.logger.Warn("unexpected object type in the node informer", "type", fmt.Sprintf("%T", obj))

		return
	}

	w.store.Delete(node.Name)
}

// Run starts the informer and blocks until ctx is cancelled.
func (w *NodeWatcher) Run(ctx context.Context) {
	w.factory.Start(ctx.Done())
	<-ctx.Done()
	w.factory.Shutdown()
}

// WaitForSync blocks until the initial cache fill completes. A false return
// means ctx ended (shutdown); while ctx is alive and the API is unreachable it
// keeps blocking instead of returning false, which is transient
// unavailability, not a configuration error.
func (w *NodeWatcher) WaitForSync(ctx context.Context) bool {
	return cache.WaitForCacheSync(ctx.Done(), w.informer.HasSynced)
}
