// Copyright 2023 Flant JSC
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

package watcher

import (
	"context"
	"time"

	v1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const leaseLabel = "deckhouse.io/documentation-builder-sync"
const resyncTimeout = time.Minute

type watcher struct {
	kClient   *kubernetes.Clientset
	namespace string
	logger    *log.Logger
}

// nolint: revive
func New(kClient *kubernetes.Clientset, namespace string, logger *log.Logger) *watcher {
	return &watcher{
		kClient:   kClient,
		namespace: namespace,
		logger:    logger,
	}
}

func (w *watcher) Watch(ctx context.Context, addHandler, deleteHandler func(ctx context.Context, backend string)) {
	tweakListOptions := func(options *metav1.ListOptions) {
		options.LabelSelector = leaseLabel
	}

	factory := informers.NewSharedInformerFactoryWithOptions(
		w.kClient,
		resyncTimeout,
		informers.WithNamespace(w.namespace),
		informers.WithTweakListOptions(tweakListOptions),
	)

	informer := factory.Coordination().V1().Leases().Informer()
	// nolint:errcheck
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			lease, ok := obj.(*v1.Lease)
			if !ok {
				w.logger.Error("cast object to lease error")
				return
			}

			if lease != nil {
				holderIdentity := lease.Spec.HolderIdentity
				if holderIdentity != nil {
					addHandler(ctx, *holderIdentity)
					return
				}
			}

			w.logger.Error(`lease "holderIdentity" is empty`)
		},
		DeleteFunc: func(obj interface{}) {
			lease, ok := obj.(*v1.Lease)
			if !ok {
				w.logger.Error("cast object to lease error")
				return
			}

			if lease != nil {
				holderIdentity := lease.Spec.HolderIdentity
				if holderIdentity != nil {
					deleteHandler(ctx, *holderIdentity)
					return
				}
			}

			w.logger.Error(`lease "holderIdentity" is empty`)
		},
	})

	go informer.Run(ctx.Done())

	// Wait for the first sync of the informer cache, should not take long
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		w.logger.Fatal("unable to sync caches", log.Err(ctx.Err()))
	}
}
