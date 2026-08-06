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

package common

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// LazyInstanceClassSource watches every InstanceClass kind the cloud providers register — the
// ones registered right now and the ones registered at any later point in the pod's life.
//
// A plain builder watch cannot do this: the watch list of a controller is closed once it starts,
// and the InstanceClass kind and API version are data in the provider registration Secret, which
// on a cluster that enables its cloud provider late appears only after the controller started.
// This source therefore subscribes to the registration Secrets (their informer exists either
// way) and, per registered GVK, starts the very source.Kind the builder would have started —
// same handler, same predicates, just at the moment the registration actually exists.
// source.Kind waits for the CRD itself when the Secret precedes it.
//
// Watches are only ever added, never removed: a registration that changes its version leaves the
// old handler on the old informer. That is harmless as a trigger (the workqueue deduplicates)
// and costs one orphaned informer until the next pod restart — not worth the machinery of
// unsubscription.
func LazyInstanceClassSource(informers cache.Cache, eventHandler handler.EventHandler, predicates ...predicate.Predicate) source.Source {
	return source.Func(func(ctx context.Context, queue workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
		secretInformer, err := informers.GetInformer(ctx, &corev1.Secret{})
		if err != nil {
			return fmt.Errorf("get the secret informer: %w", err)
		}

		// Buffered so the informer callback never blocks; pokes arriving while one is pending
		// collapse into it — the goroutine re-reads every registration anyway. Adding the
		// handler replays the store, so registrations that predate this controller arrive the
		// same way as future ones.
		poke := make(chan struct{}, 1)
		notify := func() {
			select {
			case poke <- struct{}{}:
			default:
			}
		}
		if _, err := secretInformer.AddEventHandler(toolscache.FilteringResourceEventHandler{
			FilterFunc: func(obj any) bool {
				secret, ok := obj.(*corev1.Secret)
				if !ok {
					return false
				}
				_, registered := secret.Labels[CloudProviderRegistrationLabel]
				return registered
			},
			Handler: toolscache.ResourceEventHandlerFuncs{
				AddFunc:    func(any) { notify() },
				UpdateFunc: func(any, any) { notify() },
				// A deleted registration needs no reaction: watches are never unregistered.
			},
		}); err != nil {
			return fmt.Errorf("subscribe to the registration secrets: %w", err)
		}

		go func() {
			logger := log.FromContext(ctx)
			started := map[schema.GroupVersionKind]bool{}
			for {
				select {
				case <-ctx.Done():
					return
				case <-poke:
				}

				gvks, err := RegisteredInstanceClassGVKs(ctx, informers)
				if err != nil {
					logger.V(1).Info("list instance class registrations", "error", err.Error())
					continue
				}
				for _, gvk := range gvks {
					if started[gvk] {
						continue
					}
					obj := &unstructured.Unstructured{}
					obj.SetGroupVersionKind(gvk)
					if err := source.Kind(informers, client.Object(obj), eventHandler, predicates...).Start(ctx, queue); err != nil {
						logger.Error(err, "start instance class watch", "gvk", gvk.String())
						continue
					}
					started[gvk] = true
					logger.Info("instance class watch started", "gvk", gvk.String())
				}
			}
		}()
		return nil
	})
}
