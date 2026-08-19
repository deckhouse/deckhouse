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
	"bytes"
	"context"
	"fmt"
	"maps"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// IsRegistrationSecret reports whether an object is a registration Secret: right namespace, name prefix
// and label. It is the single definition of one — Load and every watch resolve through it.
func IsRegistrationSecret(obj client.Object) bool {
	if obj.GetNamespace() != SecretNamespace {
		return false
	}
	if !strings.HasPrefix(obj.GetName(), SecretNamePrefix) {
		return false
	}
	_, ok := obj.GetLabels()[SecretLabel]
	return ok
}

// IsRegistrationSecretKey reports whether a reconcile key names a registration Secret. No label check:
// a key carries none, and the watch behind it already filtered on IsRegistration.
func IsRegistrationSecretKey(key types.NamespacedName) bool {
	if key.Namespace != SecretNamespace {
		return false
	}
	return strings.HasPrefix(key.Name, SecretNamePrefix)
}

// RegistrationSecretPredicate filters a watch down to the registration Secrets.
func RegistrationSecretPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(IsRegistrationSecret)
}

// RegistrationSecretsRequests returns one request per registration Secret, for controllers keyed by the
// Secret itself. A failed List yields none: an event mapper cannot return an error, and the
// controller resync covers the miss.
func RegistrationSecretsRequests(ctx context.Context, r client.Reader) []reconcile.Request {
	secrets := &corev1.SecretList{}
	if err := r.List(ctx, secrets,
		client.InNamespace(SecretNamespace),
		client.HasLabels{SecretLabel},
	); err != nil {
		log.FromContext(ctx).Error(err, "list cloud provider registration secrets for enqueue")
		return nil
	}

	ret := make([]reconcile.Request, 0, len(secrets.Items))
	for i := range secrets.Items {
		ret = append(ret, reconcile.Request{NamespacedName: types.NamespacedName{
			Name:      secrets.Items[i].Name,
			Namespace: secrets.Items[i].Namespace,
		}})
	}

	return ret
}

// NodeGroupHandler enqueues the NodeGroups that run on the registration the event carries. Pair it
// with RegistrationPredicate.
//
// The provider is decoded from the event object rather than looked up, so a delete answers the same
// question as a create at a point where the Secret is already gone.
//
// An update compares the raw data first: one that changed none of it enqueues nothing. A real edit
// resolves both sides, because an edit that renames the provider moves NodeGroups off it, and the
// group that just left is in no set the new object can produce.
func NodeGroupHandler(r client.Reader) handler.EventHandler {
	enqueue := func(ctx context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request], carried ...Provider) {
		for _, req := range nodeGroupRequests(ctx, r, carried...) {
			q.Add(req)
		}
	}

	// Create, delete and generic all ask the same question of the same object.
	enqueueObject := func(ctx context.Context, obj client.Object, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return
		}

		enqueue(ctx, q, FromSecretData(secret.Data))
	}

	return handler.Funcs{
		CreateFunc: func(ctx context.Context, e event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			enqueueObject(ctx, e.Object, q)
		},
		DeleteFunc: func(ctx context.Context, e event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			enqueueObject(ctx, e.Object, q)
		},
		GenericFunc: func(ctx context.Context, e event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			enqueueObject(ctx, e.Object, q)
		},
		UpdateFunc: func(ctx context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			before, okBefore := e.ObjectOld.(*corev1.Secret)
			after, okAfter := e.ObjectNew.(*corev1.Secret)
			if !okBefore || !okAfter {
				return
			}
			if maps.EqualFunc(before.Data, after.Data, bytes.Equal) {
				return
			}
			enqueue(ctx, q, FromSecretData(before.Data), FromSecretData(after.Data))
		},
	}
}

// nodeGroupRequests returns one request per NodeGroup the carried providers run.
func nodeGroupRequests(ctx context.Context, r client.Reader, carried ...Provider) []reconcile.Request {
	logger := log.FromContext(ctx)

	clusterProvider, err := clusterProviderType(ctx, r)
	if err != nil {
		logger.Error(err, "read the cluster provider for a cloud provider registration event")
		return nil
	}

	ngList := &v1.NodeGroupList{}
	if err := r.List(ctx, ngList); err != nil {
		logger.Error(err, "list NodeGroups for a cloud provider registration event")
		return nil
	}

	// The providers the event carries, resolved by the rules a reconcile uses. A registration that
	// is not the cluster's own leaves the default zero, and no NodeGroup runs on it.
	defaultProvider, _ := byType(carried, clusterProvider)
	changed := NewProviders(carried, defaultProvider)

	ret := make([]reconcile.Request, 0, len(ngList.Items))
	for i := range ngList.Items {
		ng := &ngList.Items[i]
		if _, ok := changed.ForNodeGroup(ng); ok {
			ret = append(ret, reconcile.Request{NamespacedName: types.NamespacedName{Name: ng.Name}})
		}
	}

	return ret
}

// LazyInstanceClassSource watches every InstanceClass kind the providers register, including the
// ones registered after this controller started — which a builder watch cannot do, since its watch
// list closes at start and the kind is data in the registration Secret.
//
// Starting a watch is fire-and-forget: source.Kind spawns a goroutine that polls for the informer
// until the CRD exists, so a registration preceding its CRD needs no retry here.
//
// Watches are only added, never removed. A re-versioned registration leaves an orphaned informer
// retrying under backoff until the next pod restart; unsubscribing would mean hand-rolling the
// event translation source.Kind gives for free.
func LazyInstanceClassSource(informers cache.Cache, eventHandler handler.EventHandler, predicates ...predicate.Predicate) source.Source {
	return source.Func(func(ctx context.Context, queue workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
		secretInformer, err := informers.GetInformer(ctx, &corev1.Secret{})
		if err != nil {
			return fmt.Errorf("get the secret informer: %w", err)
		}

		// Buffered so the informer callback never blocks; pokes collapse, the goroutine re-reads
		// everything anyway. Adding the handler replays the store.
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
				return IsRegistrationSecret(secret)
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
					logger.V(1).Info("list instance class providers", "error", err.Error())
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
					logger.Info("instance class watch registered; it attaches once the CRD is served", "gvk", gvk.String())
				}
			}
		}()
		return nil
	})
}
