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
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/adoption"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// laggingCacheRecheck is when the tables are built again after an object was missing from the
// cache but not from the API.
const laggingCacheRecheck = 2 * time.Second

// tablesRequest is the one request of the table reconciler: it builds the tables of both
// instances in one pass, since the object an instance chooses by a host is one in the whole cluster.
var tablesRequest = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: naming.DexNamespace, Name: "client-tables"}}

// TableReconciler keeps the table of the clients of every shared dex-authenticator instance: a
// record per DexAuthenticator with the secrets of its Dex client and of its cookies, in a Secret
// the pods of the instance mount.
type TableReconciler struct {
	client client.Client
	// reader bypasses the cache: the table Secrets are read through it, since the cache holds the
	// metadata of Secrets only, and an object missing from the cache is looked for through it
	// before its record goes.
	reader client.Reader
	log    logr.Logger

	state   *State
	notify  chan<- event.GenericEvent
	metrics *Metrics
	// gatewayAPI tells that HTTPRoute objects are watched.
	gatewayAPI bool
	// referenceGrants tells that the cluster has ReferenceGrant objects: the reconciler keeps the
	// grants of the namespaces with a served DexAuthenticator (syncReferenceGrants).
	referenceGrants bool

	now func() time.Time
	// limit is how many bytes the two keys of a table Secret may take, and share how many of them
	// the records of one namespace may take.
	limit int
	share int

	// highest is the highest generation of a table read or built since the start.
	highest int64

	// generations are the generations of the records written since the start. Reconcile is the
	// only one to touch them, and the controller runs it one at a time.
	generations map[types.NamespacedName]int64
}

// NewTableReconciler constructs a TableReconciler. Decisions go to state, and the objects they
// change to notify.
func NewTableReconciler(c client.Client, reader client.Reader, log logr.Logger, state *State, notify chan<- event.GenericEvent, metrics *Metrics, gatewayAPI bool) *TableReconciler {
	return &TableReconciler{
		client:      c,
		reader:      reader,
		log:         log,
		state:       state,
		notify:      notify,
		metrics:     metrics,
		gatewayAPI:  gatewayAPI,
		now:         time.Now,
		limit:       TableLimit,
		share:       NamespaceShare,
		generations: map[types.NamespacedName]int64{},
	}
}

// readSettings returns the settings of the module, and false when the chart does not render them.
func readSettings(ctx context.Context, c client.Client) (Settings, bool, error) {
	cm := &corev1.ConfigMap{}
	err := c.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: SettingsConfigMapName}, cm)
	if apierrors.IsNotFound(err) {
		return Settings{}, false, nil
	}
	if err != nil {
		return Settings{}, false, fmt.Errorf("get ConfigMap %s: %w", SettingsConfigMapName, err)
	}
	settings, err := ParseSettings(cm)
	if err != nil {
		return Settings{}, false, reconcile.TerminalError(err)
	}
	return settings, true, nil
}

// Reconcile builds the tables of the instances from the DexAuthenticator objects and writes the
// ones that changed.
func (r *TableReconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	settings, ok, err := readSettings(ctx, r.client)
	if err != nil || !ok {
		return reconcile.Result{}, err
	}

	in := buildInput{
		invalid:     map[types.NamespacedName]invalidObject{},
		previous:    map[string]table{},
		retained:    map[types.NamespacedName]bool{},
		generations: r.generations,
		settings:    settings,
		now:         r.now(),
		limit:       r.limit,
		newSecrets:  newSecrets,

		namespaceShare: r.share,
	}

	list := controller.List(controller.DexAuthenticatorGVK)
	if err := r.client.List(ctx, list); err != nil {
		return reconcile.Result{}, fmt.Errorf("list DexAuthenticators: %w", err)
	}
	live := make(map[types.NamespacedName]struct{}, len(list.Items))
	for i := range list.Items {
		obj := &list.Items[i]
		if obj.GetDeletionTimestamp() != nil {
			// As good as gone: its routes are being removed, and its record goes after them.
			continue
		}
		key := client.ObjectKeyFromObject(obj)
		live[key] = struct{}{}
		a, err := Decode(obj)
		if err != nil {
			in.invalid[key] = invalidObject{uid: obj.GetUID(), generation: obj.GetGeneration(), err: err}
			continue
		}
		in.authenticators = append(in.authenticators, a)
	}

	secrets := make(map[string]*corev1.Secret, 2)
	var result reconcile.Result
	for _, instance := range naming.AuthenticatorInstances() {
		secret, previous, err := r.readTable(ctx, instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		secrets[instance], in.previous[instance] = secret, previous

		for _, record := range previous.file.Clients {
			key := record.key()
			if _, isLive := live[key]; isLive {
				continue
			}
			keep, lagging, err := r.keepsRecord(ctx, key)
			if err != nil {
				return reconcile.Result{}, err
			}
			if keep {
				in.retained[key] = true
			}
			if lagging {
				result.RequeueAfter = laggingCacheRecheck
			}
		}
	}

	in.routed = map[types.NamespacedName][]string{}
	for _, a := range in.authenticators {
		previous := in.previous[a.Instance()]
		if _, found := previous.record(a.Key()); found && previous.owners[a.Key().String()] == a.UID {
			continue
		}
		hosts, err := r.routedHosts(ctx, a.Key())
		if err != nil {
			return reconcile.Result{}, err
		}
		if len(hosts) > 0 {
			in.routed[a.Key()] = hosts
		}
	}

	for _, previous := range in.previous {
		r.highest = max(r.highest, previous.file.Generation)
	}
	in.floor = r.highest

	built, err := buildTables(in)
	if err != nil {
		return reconcile.Result{}, err
	}
	// Remembered before the tables are written, as the generations of the records below are.
	for _, next := range built.tables {
		r.highest = max(r.highest, next.file.Generation)
	}
	// The generations are remembered before the tables are written. A pass that fails after it
	// wrote one of them is followed by one that finds that table as it should be: its records must
	// get the generation they were written with, not the one remembered before the failed pass, or
	// a route would be made for a record no pod has loaded yet. A record that was not written
	// differs from the table on the next pass and gets a newer generation.
	generations := make(map[types.NamespacedName]int64, len(built.entries))
	for key, entry := range built.entries {
		if entry.Record != nil {
			generations[key] = entry.Generation
		}
	}
	r.generations = generations

	for _, instance := range naming.AuthenticatorInstances() {
		next := built.tables[instance]
		if !next.changed && secrets[instance] != nil {
			continue
		}
		if err := r.writeTable(ctx, instance, secrets[instance], next); err != nil {
			return reconcile.Result{}, err
		}
	}

	r.metrics.observe(in.authenticators, built, r.limit)
	r.announce(ctx, r.state.set(maps.Clone(built.entries)))
	if r.referenceGrants {
		if err := r.syncReferenceGrants(ctx, built.entries); err != nil {
			return reconcile.Result{}, err
		}
	}
	return result, nil
}

// readTable reads the table Secret of an instance; nil and an empty table when there is none. A
// table that cannot be read is written anew: nothing else holds its secrets, and an instance
// without a table serves nobody.
func (r *TableReconciler) readTable(ctx context.Context, instance string) (*corev1.Secret, table, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: naming.DexNamespace, Name: TableSecretName(instance)}
	err := r.reader.Get(ctx, key, secret)
	if apierrors.IsNotFound(err) {
		return nil, table{owners: map[string]types.UID{}}, nil
	}
	if err != nil {
		return nil, table{}, fmt.Errorf("get Secret %s: %w", key, err)
	}
	previous, err := parseTable(secret.Data)
	if err != nil {
		r.log.Error(err, "the table cannot be read, writing it anew: the sessions of its clients are lost", "secret", key.Name)
		return secret, table{owners: map[string]types.UID{}}, nil
	}
	return secret, previous, nil
}

// keepsRecord reports whether the record of key, which is not among the objects of the cache,
// stays in the table, and whether it stays only because the cache lags behind the API. An object that is missing from the cache only keeps it: taking the record
// of a living object away and giving it back would change its secrets and sign its users out. A
// deleted object keeps it while it has a route: the route goes first, so that no request for its
// host reaches an instance that no longer knows the host.
func (r *TableReconciler) keepsRecord(ctx context.Context, key types.NamespacedName) (bool, bool, error) {
	live := controller.Object(controller.DexAuthenticatorGVK)
	switch err := r.reader.Get(ctx, key, live); {
	case err == nil:
		if live.GetDeletionTimestamp() == nil {
			return true, true, nil
		}
	case !apierrors.IsNotFound(err):
		return false, false, fmt.Errorf("get DexAuthenticator %s: %w", key, err)
	}

	routes, err := r.hasRoutes(ctx, key)
	if err != nil {
		return false, false, err
	}
	return routes, false, nil
}

// hasRoutes reports whether an Ingress or an HTTPRoute of the object key is left.
func (r *TableReconciler) hasRoutes(ctx context.Context, key types.NamespacedName) (bool, error) {
	ingresses, httpRoutes, err := listRoutes(ctx, r.client, key, r.gatewayAPI)
	return len(ingresses)+len(httpRoutes) > 0, err
}

// routedHosts are the hosts the routes of the object key lead to it by the host: those of its
// HTTPRoute objects and of its Ingress objects that do not send the name of its Service. Only the
// routes with the heritage label count: the label-objects.deckhouse.io policy lets no user create
// or change an object with it, so a user of a namespace cannot make a route claim a host for an
// object of the namespace.
func (r *TableReconciler) routedHosts(ctx context.Context, key types.NamespacedName) ([]string, error) {
	ingresses, httpRoutes, err := listRoutes(ctx, r.client, key, r.gatewayAPI)
	if err != nil {
		return nil, err
	}
	var hosts []string
	add := func(host string) {
		if host = strings.ToLower(host); host != "" && !slices.Contains(hosts, host) {
			hosts = append(hosts, host)
		}
	}
	for i := range ingresses {
		ingress := &ingresses[i]
		if ingress.Labels[labelHeritage] != heritageValue {
			continue
		}
		if _, viaService := ingress.Annotations[upstreamVhostAnnotation]; viaService {
			continue
		}
		for _, rule := range ingress.Spec.Rules {
			add(rule.Host)
		}
	}
	for i := range httpRoutes {
		route := &httpRoutes[i]
		if route.GetLabels()[labelHeritage] != heritageValue {
			continue
		}
		names, _, err := unstructured.NestedStringSlice(route.Object, "spec", "hostnames")
		if err != nil {
			continue
		}
		for _, name := range names {
			add(name)
		}
	}
	return hosts, nil
}

// tableLabels are the labels of a table Secret: the controller finds its Secrets by the last one.
func tableLabels(instance string) map[string]string {
	return map[string]string{
		labelHeritage:           heritageValue,
		labelModule:             moduleValue,
		InstanceAppLabel:        InstanceAppValue,
		InstanceLabel:           instance,
		adoption.ManagedByLabel: adoption.ManagedByValue,
	}
}

func (r *TableReconciler) writeTable(ctx context.Context, instance string, current *corev1.Secret, next builtTable) error {
	data, err := next.data()
	if err != nil {
		return err
	}
	name := TableSecretName(instance)

	if current == nil {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: naming.DexNamespace, Labels: tableLabels(instance)},
			Type:       corev1.SecretTypeOpaque,
			Data:       data,
		}
		if err := r.client.Create(ctx, secret); err != nil {
			return fmt.Errorf("create Secret %s: %w", name, err)
		}
		r.log.Info("wrote the table", "instance", instance, "generation", next.file.Generation, "clients", len(next.file.Clients))
		return nil
	}

	updated := current.DeepCopy()
	if updated.Labels == nil {
		updated.Labels = make(map[string]string, 5)
	}
	maps.Copy(updated.Labels, tableLabels(instance))
	updated.Data = data
	// The resourceVersion of the Secret that was read makes a write over a newer table fail.
	if err := r.client.Update(ctx, updated); err != nil {
		return fmt.Errorf("update Secret %s: %w", name, err)
	}
	r.log.Info("wrote the table", "instance", instance, "generation", next.file.Generation, "clients", len(next.file.Clients))
	return nil
}

// announce sends the objects whose decision changed to the reconciler of their objects.
func (r *TableReconciler) announce(ctx context.Context, keys []types.NamespacedName) {
	for _, key := range keys {
		obj := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name}}
		select {
		case r.notify <- event.GenericEvent{Object: obj}:
		case <-ctx.Done():
			return
		}
	}
}
