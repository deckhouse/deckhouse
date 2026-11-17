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

package dexauthenticator

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"user-authn-controller/internal/adoption"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/migration"
	"user-authn-controller/internal/naming"
)

// announcements is how many decisions of the table reconciler may wait for the reconciler of the
// objects; the sender waits when there are more.
const announcements = 1024

// Options are the settings of the process the reconcilers need before they start.
type Options struct {
	// GatewayAPI tells that the cluster has HTTPRoute objects and the module publishes through
	// them: only then they are watched and written.
	GatewayAPI bool
	// ReferenceGrants tells that the cluster has ReferenceGrant objects: the controller grants the
	// HTTPRoute objects of every namespace with a served DexAuthenticator the Service of its
	// instance in d8-user-authn.
	ReferenceGrants bool
}

// Register wires the two reconcilers onto the manager.
//
// Required RBAC: dexauthenticators get, list, watch and dexauthenticators/status update;
// configmaps and pods get, list, watch in d8-user-authn; secrets get, create, update there (the
// table of an instance); oauth2clients get, list, watch, create, update, delete in d8-user-authn;
// services and ingresses, and httproutes with the Gateway API, get, list, watch, create, update,
// delete cluster-wide; referencegrants list, create, update, delete in d8-user-authn with
// ReferenceGrants; events create, patch.
func Register(mgr manager.Manager, opts Options) error {
	metrics, err := NewMetrics(ctrlmetrics.Registry)
	if err != nil {
		return fmt.Errorf("register the dexauthenticator metrics: %w", err)
	}

	state := NewState()
	events := make(chan event.GenericEvent, announcements)
	log := mgr.GetLogger().WithName("dexauthenticator")

	legacy := migration.NewDexAuthenticatorLegacy(mgr.GetClient(), mgr.GetAPIReader(), log.WithName("legacy"))
	tables := NewTableReconciler(mgr.GetClient(), mgr.GetAPIReader(), log.WithName("table"), state, events, metrics, legacy, opts.GatewayAPI)
	tables.referenceGrants = opts.ReferenceGrants
	if err := tables.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup dexauthenticator table controller: %w", err)
	}

	objects := New(mgr.GetClient(), mgr.GetAPIReader(), mgr.GetEventRecorder(recorderName), log, state, NewPodStatus(mgr.GetClient()), legacy, opts.GatewayAPI)
	if err := objects.SetupWithManager(mgr, events); err != nil {
		return fmt.Errorf("setup dexauthenticator controller: %w", err)
	}
	if err := mgr.Add(legacySweep{legacy: legacy, announce: events, log: log.WithName("legacy")}); err != nil {
		return fmt.Errorf("add the sweep of the per-object dex-authenticators: %w", err)
	}
	return nil
}

// legacySweepRetry is how long the sweep waits before it lists again after a failure.
const legacySweepRetry = 30 * time.Second

// legacySweep gives the reconciler of the objects, once at the start of the leader, every
// DexAuthenticator that still has a dex-authenticator of its own: the one of an object deleted
// while the controller was down is removed by that reconcile, and no event would bring it.
type legacySweep struct {
	legacy   *migration.DexAuthenticatorLegacy
	announce chan<- event.GenericEvent
	log      logr.Logger
}

// NeedLeaderElection makes the sweep run on the leader only.
func (legacySweep) NeedLeaderElection() bool { return true }

// Start lists what is left and announces it, trying again until the list succeeds or ctx ends.
func (s legacySweep) Start(ctx context.Context) error {
	for {
		keys, err := s.legacy.Pending(ctx)
		if err == nil {
			for _, key := range keys {
				obj := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name}}
				select {
				case s.announce <- event.GenericEvent{Object: obj}:
				case <-ctx.Done():
					return nil
				}
			}
			return nil
		}
		s.log.Error(err, "list what is left of the per-object dex-authenticators, trying again", "after", legacySweepRetry)
		select {
		case <-time.After(legacySweepRetry):
		case <-ctx.Done():
			return nil
		}
	}
}

// InformerCacheByObject scopes the informers of the reconcilers. The Service, Ingress and
// HTTPRoute informers hold the objects of the controller only: every such object of the cluster
// would be a lot of memory for nothing, and an object of somebody else under a name of ours is
// found when its create conflicts. The pods are those of the instances, the ConfigMap is the one
// with the settings. The Secret and OAuth2Client informers are those of the dexclient reconciler.
func InformerCacheByObject(opts Options) map[client.Object]cache.ByObject {
	managedEverywhere := cache.ByObject{
		Namespaces: map[string]cache.Config{cache.AllNamespaces: {LabelSelector: adoption.ManagedSelector()}},
	}
	byObject := map[client.Object]cache.ByObject{
		&corev1.Service{}:       managedEverywhere,
		&networkingv1.Ingress{}: managedEverywhere,
		&corev1.Pod{}: {
			Namespaces: map[string]cache.Config{naming.DexNamespace: {
				LabelSelector: labels.SelectorFromSet(labels.Set{InstanceAppLabel: InstanceAppValue}),
			}},
		},
		&corev1.ConfigMap{}: {
			Namespaces: map[string]cache.Config{naming.DexNamespace: {
				FieldSelector: fields.OneTermEqualSelector("metadata.name", SettingsConfigMapName),
			}},
		},
	}
	if opts.GatewayAPI {
		byObject[controller.Object(HTTPRouteGVK)] = managedEverywhere
	}
	return byObject
}

// specChanged lets a DexAuthenticator through when anything but its status changed: the
// reconciler writes the status itself.
func specChanged() predicate.Predicate {
	return predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{}, predicate.LabelChangedPredicate{})
}

func named(name string) builder.Predicates {
	return builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == naming.DexNamespace && obj.GetName() == name
	}))
}

func isManagedObject(obj client.Object) bool {
	objLabels := obj.GetLabels()
	return objLabels[adoption.ManagedByLabel] == adoption.ManagedByValue && objLabels[LabelFor] != ""
}

func isTableSecret(obj client.Object) bool {
	if obj.GetNamespace() != naming.DexNamespace {
		return false
	}
	for _, instance := range naming.AuthenticatorInstances() {
		if obj.GetName() == TableSecretName(instance) {
			return true
		}
	}
	return false
}

// SetupWithManager builds the tables again on a change of any DexAuthenticator, of the settings
// and of a table Secret, and when a route of the controller is deleted: the record of a deleted
// object waits for its routes to go.
func (r *TableReconciler) SetupWithManager(mgr manager.Manager) error {
	enqueue := handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return []reconcile.Request{tablesRequest}
	})
	deletedRoute := builder.WithPredicates(predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return isManagedObject(e.Object) },
	})

	b := ctrl.NewControllerManagedBy(mgr).
		Named("dexauthenticator-table").
		Watches(controller.Object(controller.DexAuthenticatorGVK), enqueue, builder.WithPredicates(specChanged())).
		Watches(&corev1.ConfigMap{}, enqueue, named(SettingsConfigMapName)).
		Watches(&corev1.Secret{}, enqueue, builder.WithPredicates(predicate.NewPredicateFuncs(isTableSecret)), builder.OnlyMetadata).
		Watches(&networkingv1.Ingress{}, enqueue, deletedRoute)
	if r.gatewayAPI {
		b = b.Watches(controller.Object(HTTPRouteGVK), enqueue, deletedRoute)
	}
	return b.Complete(r)
}

// mapObject maps a Service, an Ingress or an HTTPRoute of the controller to its DexAuthenticator.
func mapObject(_ context.Context, obj client.Object) []reconcile.Request {
	if !isManagedObject(obj) {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetLabels()[LabelFor]}}}
}

// mapClient maps the Dex client of a DexAuthenticator to it.
func mapClient(_ context.Context, obj client.Object) []reconcile.Request {
	u, ok := controller.AsUnstructured(obj)
	if !ok || obj.GetNamespace() != naming.DexNamespace {
		return nil
	}
	id, _, _ := unstructured.NestedString(u.Object, "id")
	name, namespace, ok := naming.ParseDexAuthenticatorClientID(id)
	if !ok {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}}}
}

// SetupWithManager reconciles a DexAuthenticator on its own change, on a change of one of its
// objects, on a new decision of the table reconciler (announced) and, all of them, on a change
// of the settings.
func (r *Reconciler) SetupWithManager(mgr manager.Manager, announced <-chan event.GenericEvent) error {
	all := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
		list := controller.List(controller.DexAuthenticatorGVK)
		if err := r.client.List(ctx, list); err != nil {
			r.writer.Log.Error(err, "list DexAuthenticators after a change of the settings")
			return nil
		}
		requests := make([]reconcile.Request, 0, len(list.Items))
		for i := range list.Items {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&list.Items[i])})
		}
		return requests
	})
	objects := handler.EnqueueRequestsFromMapFunc(mapObject)

	b := ctrl.NewControllerManagedBy(mgr).
		Named("dexauthenticator").
		For(controller.Object(controller.DexAuthenticatorGVK), builder.WithPredicates(specChanged())).
		Watches(&corev1.Service{}, objects).
		Watches(&networkingv1.Ingress{}, objects).
		Watches(controller.Object(controller.OAuth2ClientGVK), handler.EnqueueRequestsFromMapFunc(mapClient)).
		Watches(&corev1.ConfigMap{}, all, named(SettingsConfigMapName)).
		WatchesRawSource(source.Channel(announced, &handler.EnqueueRequestForObject{}))
	if r.gatewayAPI {
		b = b.Watches(controller.Object(HTTPRouteGVK), objects)
	}
	return b.Complete(r)
}
