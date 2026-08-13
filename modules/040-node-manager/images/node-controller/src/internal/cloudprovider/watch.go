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

package cloudprovider

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

// IsRegistration reports whether an object is a cloud provider registration Secret: it lives in
// the registration namespace, is named with the shared prefix and carries the label. Matching on
// the prefix rather than on one full name is what lets a second provider's Secret trigger a
// reconcile at all — every provider publishes both prefix and prefix + "-<provider>".
//
// This is the single definition of a registration: Load and the watches all resolve through it, so
// a Secret can never be a provider to one and invisible to the other.
func IsRegistration(obj client.Object) bool {
	if obj.GetNamespace() != SecretNamespace {
		return false
	}
	if !strings.HasPrefix(obj.GetName(), SecretNamePrefix) {
		return false
	}
	_, ok := obj.GetLabels()[SecretLabel]
	return ok
}

// RegistrationPredicate filters a watch down to the registration Secrets.
func RegistrationPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(IsRegistration)
}

// RegistrationRequests returns one request per registration Secret, for controllers keyed by the
// Secret itself rather than by NodeGroup. A failed List yields no requests: the caller is an event
// mapper, which has nowhere to return an error to, and the controller's resync covers the miss.
func RegistrationRequests(ctx context.Context, r client.Reader) []reconcile.Request {
	secrets := &corev1.SecretList{}
	if err := r.List(ctx, secrets,
		client.InNamespace(SecretNamespace),
		client.HasLabels{SecretLabel},
	); err != nil {
		log.FromContext(ctx).Error(err, "list cloud provider registration secrets for enqueue")
		return nil
	}

	reqs := make([]reconcile.Request, 0, len(secrets.Items))
	for i := range secrets.Items {
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{
			Name:      secrets.Items[i].Name,
			Namespace: secrets.Items[i].Namespace,
		}})
	}
	return reqs
}

// LazyInstanceClassSource watches every InstanceClass kind the cloud providers register — the
// ones registered right now and the ones registered at any later point in the pod's life.
//
// A plain builder watch cannot do this: the watch list of a controller is closed once it starts,
// and the InstanceClass kind and API version are data in the provider registration Secret, which
// on a cluster that enables its cloud provider late appears only after the controller started.
// This source therefore subscribes to the registration Secrets (their informer exists either
// way) and, per registered GVK, starts the very source.Kind the builder would have started —
// same handler, same predicates, just at the moment the registration actually exists.
//
// Starting one is fire-and-forget by design: source.Kind returns as soon as it has spawned its
// own goroutine, which then polls for the informer until the CRD exists. A registration that
// precedes its CRD therefore needs no retry here — hence the log wording below, which claims
// only that the watch was registered, not that it is already delivering events.
//
// Watches are only ever added, never removed: a registration that changes its version leaves the
// old handler on the old informer. Its events stay harmless (the workqueue deduplicates), and
// once the old version stops being served its reflector only retries under backoff — a couple of
// log lines per minute, no crash, with the new version's watch delivering throughout. The cost is
// that one orphaned informer until the next pod restart, which every Deckhouse release performs
// anyway. Unsubscribing would mean hand-rolling the event translation source.Kind gives us for
// free, because it discards the handler registration RemoveEventHandler would need.
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
				return IsRegistration(secret)
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

				registrations, err := loadRegistrations(ctx, informers)
				if err != nil {
					logger.V(1).Info("list instance class registrations", "error", err.Error())
					continue
				}
				for _, gvk := range (Registry{registrations: registrations}).InstanceClassGVKs() {
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
					logger.Info("instance class watch registered; it attaches once the CRD is served", "gvk", gvk.String())
				}
			}
		}()
		return nil
	})
}
