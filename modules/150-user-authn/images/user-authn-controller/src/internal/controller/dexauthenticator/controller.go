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

// Package dexauthenticator serves DexAuthenticator objects by the two shared dex-authenticator
// instances of the module instead of a Deployment per object.
//
// Two reconcilers do it. The table reconciler keeps, for every instance, the table of its
// clients: a record per DexAuthenticator with the hosts of its applications and the secrets of
// its Dex client and of its cookies, in a Secret the pods of the instance mount. The instance of
// an object is chosen by its namespace: the objects of the d8-* namespaces have one of their own.
// The reconciler of the objects keeps what a DexAuthenticator needs outside the instance: its Dex
// client in the namespace of the module, and in its own namespace the Service applications point
// their auth-url at and the Ingress and HTTPRoute objects that lead the proxy prefix to it. The
// names, labels and content of these objects are the ones the module chart gave them.
//
// The order matters. A route is made once every pod of the instance serves the record of the
// object, and removed before the record is: a request for a host an instance does not know is
// refused. The controller learns which table a pod serves by asking the pod; it has no right to
// change the pods.
//
// The chart rendered all of this from values a hook computed, so every change of a
// DexAuthenticator ran the whole module release, and an object the API refused failed it. The
// reconcilers run in the dexclient-controller Deployment, which already writes Secrets in every
// namespace and listens to nothing but health and metrics.
package dexauthenticator

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/adoption"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const (
	// recorderName is the Deployment the reconcilers run in.
	recorderName = "dexclient-controller"

	// notReadyRecheck is when an object is looked at again while the table reconciler has not
	// decided about it, loadRecheck while its instance has not loaded its record, and
	// refusedRecheck while the instance or the name of an object keeps it from being served.
	notReadyRecheck = 2 * time.Second
	loadRecheck     = 5 * time.Second
	refusedRecheck  = time.Minute
)

// Reconciler keeps the objects of a DexAuthenticator: its Dex client, its Service and its routes.
type Reconciler struct {
	client client.Client
	// reader bypasses the cache, to confirm a DexAuthenticator is gone before its objects are removed.
	reader   client.Reader
	writer   adoption.Writer
	recorder events.EventRecorder

	state     *State
	instances InstanceStatus

	// loaded are the records the pods of the instances are known to serve: the generation of the
	// record of each object that passed the check. Such a record is not asked about again, so a pod
	// that does not answer for a while does not hold the objects it already serves.
	loadedMu sync.Mutex
	loaded   map[types.NamespacedName]loadedRecord
	// gatewayAPI tells that the cluster has HTTPRoute objects: they are read and removed then.
	// Whether new ones are made is a setting of the module.
	gatewayAPI bool
}

// loadedRecord is a record every pod of its instance served when it was asked.
type loadedRecord struct {
	uid        types.UID
	generation int64
}

// New constructs a Reconciler that acts on the decisions of the table reconciler in state.
func New(c client.Client, reader client.Reader, recorder events.EventRecorder, log logr.Logger, state *State, instances InstanceStatus, gatewayAPI bool) *Reconciler {
	return &Reconciler{
		client:     c,
		reader:     reader,
		writer:     adoption.Writer{Client: c, Reader: reader, Log: log},
		recorder:   recorder,
		state:      state,
		instances:  instances,
		gatewayAPI: gatewayAPI,
		loaded:     map[types.NamespacedName]loadedRecord{},
	}
}

// isLoaded reports whether the record of entry is known to be served by its instance.
func (r *Reconciler) isLoaded(key types.NamespacedName, entry Entry) bool {
	r.loadedMu.Lock()
	defer r.loadedMu.Unlock()

	known, ok := r.loaded[key]
	return ok && known.uid == entry.UID && known.generation >= entry.Generation
}

func (r *Reconciler) setLoaded(key types.NamespacedName, entry Entry) {
	r.loadedMu.Lock()
	defer r.loadedMu.Unlock()

	r.loaded[key] = loadedRecord{uid: entry.UID, generation: entry.Generation}
}

func (r *Reconciler) forgetLoaded(key types.NamespacedName) {
	r.loadedMu.Lock()
	defer r.loadedMu.Unlock()

	delete(r.loaded, key)
}

// Reconcile brings the objects of the DexAuthenticator named in req to the desired state, or
// removes them when the DexAuthenticator is gone.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	settings, ok, err := readSettings(ctx, r.client)
	if err != nil || !ok {
		return reconcile.Result{}, err
	}

	obj := controller.Object(controller.DexAuthenticatorGVK)
	err = r.client.Get(ctx, req.NamespacedName, obj)
	if apierrors.IsNotFound(err) {
		// The cache may lag behind an object that was just created while an event of one of its
		// objects arrived first.
		live := controller.Object(controller.DexAuthenticatorGVK)
		switch err := r.reader.Get(ctx, req.NamespacedName, live); {
		case err == nil && live.GetDeletionTimestamp() == nil:
			return reconcile.Result{}, fmt.Errorf("DexAuthenticator %s exists but is not in the cache yet", req.NamespacedName)
		case err != nil && !apierrors.IsNotFound(err):
			return reconcile.Result{}, fmt.Errorf("get DexAuthenticator %s: %w", req.NamespacedName, err)
		}
		return reconcile.Result{}, r.cleanup(ctx, req.NamespacedName)
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get DexAuthenticator %s: %w", req.NamespacedName, err)
	}
	if obj.GetDeletionTimestamp() != nil {
		return reconcile.Result{}, r.cleanup(ctx, req.NamespacedName)
	}

	entry, known, ready := r.state.Entry(req.NamespacedName)
	if !ready || !known || entry.UID != obj.GetUID() || entry.ObjectGeneration != obj.GetGeneration() {
		// The table reconciler sends the object here again once it has decided about it as it is
		// now. Until then the condition stays as it is: a decision about an earlier generation would
		// write the present generation as observed for a spec no instance serves yet.
		return reconcile.Result{RequeueAfter: notReadyRecheck}, nil
	}

	a, err := Decode(obj)
	if err != nil {
		// What the object had stays as it is; the condition tells what is wrong with it.
		return reconcile.Result{}, r.setServed(ctx, obj, metav1.ConditionFalse, ReasonInvalidRecord, entry.Message)
	}
	if entry.Record == nil {
		if err := r.cleanup(ctx, req.NamespacedName); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, r.setServed(ctx, obj, metav1.ConditionFalse, entry.Reason, entry.Message)
	}
	if entry.Reason == ReasonTableFull {
		// The record the object asks for did not fit, and the object stays on the record it has.
		// Its objects were made for that record: they stay as they are, since making them anew
		// from the object as it is now would take away the routes of what it no longer names and
		// give none to what the record does not have.
		return reconcile.Result{}, r.setServed(ctx, obj, metav1.ConditionFalse, entry.Reason, entry.Message)
	}

	return r.apply(ctx, obj, a, entry, settings)
}

// apply makes the objects of a DexAuthenticator that has a record.
func (r *Reconciler) apply(ctx context.Context, obj *unstructured.Unstructured, a Authenticator, entry Entry, settings Settings) (reconcile.Result, error) {
	// The Dex client first: nothing leads to it until the routes are made.
	if err := r.applyClient(ctx, a, *entry.Record, settings); err != nil {
		return r.refused(ctx, obj, ReasonClientNotCreated, err)
	}

	// The routes of the hosts the object no longer names go at once, whatever the instance has
	// loaded: the record has lost those hosts already, and the pods stop serving them as they
	// load it. A route of a host the object still names stays until the route that replaces it is
	// written: the name of a route depends on the place of its application in the object, so a
	// host that moves up in the list gets a route under another name.
	routes, err := r.readRoutes(ctx, a, entry, settings)
	if err != nil {
		return reconcile.Result{}, err
	}
	if err := r.removeStaleRoutes(ctx, &routes, false); err != nil {
		return reconcile.Result{}, err
	}

	if result, done, err := r.waitForInstance(ctx, obj, a.Key(), entry); done {
		return result, err
	}

	if err := r.applyService(ctx, a, settings); err != nil {
		return r.refused(ctx, obj, "", err)
	}
	// The routes that are replaced go right before the ones that replace them: an ingress
	// controller refuses two routes of one host and path.
	if err := r.removeStaleRoutes(ctx, &routes, true); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.writeRoutes(ctx, a, routes); err != nil {
		return r.refused(ctx, obj, "", err)
	}

	if entry.Reason != "" {
		return reconcile.Result{}, r.setServed(ctx, obj, metav1.ConditionFalse, entry.Reason, entry.Message)
	}
	message := fmt.Sprintf("The %s instance serves the object.", entry.Instance)
	return reconcile.Result{}, r.setServed(ctx, obj, metav1.ConditionTrue, ReasonServed, message)
}

// waitForInstance reports whether the reconcile stops here because the instance does not serve
// the record of the object yet, with what to return then.
func (r *Reconciler) waitForInstance(ctx context.Context, obj *unstructured.Unstructured, key types.NamespacedName, entry Entry) (reconcile.Result, bool, error) {
	if r.isLoaded(key, entry) {
		return reconcile.Result{}, false, nil
	}

	status, err := r.instances.Loaded(ctx, entry.Instance)
	switch {
	case err != nil:
		r.writer.Log.Info("the instance does not tell which table it serves", "instance", entry.Instance, "error", err.Error())
		// Nothing is known, which is not a reason to call an object that is served not served: the
		// controller has just started, or a pod of the instance is restarting.
		if served := meta.FindStatusCondition(controller.ReadConditions(obj), ConditionServed); served != nil && served.Status == metav1.ConditionTrue {
			return reconcile.Result{RequeueAfter: loadRecheck}, true, nil
		}
	case status.refused(entry.Record) && status.Generation >= entry.Generation:
		// Only when every pod has the table of the record: a pod that still serves an older table
		// tells about the record it refused before.
		message := fmt.Sprintf("The %s instance refused the record of the object: see the log of its pods.", entry.Instance)
		if err := r.setServed(ctx, obj, metav1.ConditionFalse, ReasonInvalidRecord, message); err != nil {
			return reconcile.Result{}, true, err
		}
		return reconcile.Result{RequeueAfter: refusedRecheck}, true, nil
	case status.Generation >= entry.Generation:
		r.setLoaded(key, entry)
		return reconcile.Result{}, false, nil
	}

	message := fmt.Sprintf("The %s instance has not loaded the record of the object yet.", entry.Instance)
	if err := r.setServed(ctx, obj, metav1.ConditionFalse, ReasonWaitingForInstance, message); err != nil {
		return reconcile.Result{}, true, err
	}
	return reconcile.Result{RequeueAfter: loadRecheck}, true, nil
}

// refused records why an object of the DexAuthenticator could not be written. A name taken by
// somebody else's object is checked again later; any other error is retried.
func (r *Reconciler) refused(ctx context.Context, obj *unstructured.Unstructured, reason string, err error) (reconcile.Result, error) {
	foreign := errors.Is(err, adoption.ErrForeign)
	switch {
	case reason != "":
	case foreign:
		reason = ReasonForeignObject
	default:
		// Not a state of the object: the write is retried, and the condition stays what it was.
		return reconcile.Result{}, err
	}
	if statusErr := r.setServed(ctx, obj, metav1.ConditionFalse, reason, err.Error()); statusErr != nil {
		return reconcile.Result{}, errors.Join(err, statusErr)
	}
	if foreign {
		return reconcile.Result{RequeueAfter: refusedRecheck}, nil
	}
	return reconcile.Result{}, err
}

// managed reports whether obj is an object the controller made for the DexAuthenticator name.
func managed(name string) func(client.Object) bool {
	return func(obj client.Object) bool {
		labels := obj.GetLabels()
		return labels[adoption.ManagedByLabel] == adoption.ManagedByValue && labels[LabelFor] == name
	}
}

// managedClient reports whether obj is the Dex client the controller made for the client id.
func managedClient(id string) func(client.Object) bool {
	return func(obj client.Object) bool {
		u, ok := controller.AsUnstructured(obj)
		if !ok {
			return false
		}
		have, _, _ := unstructured.NestedString(u.Object, "id")
		return have == id && u.GetLabels()[adoption.ManagedByLabel] == adoption.ManagedByValue
	}
}

// getClient reads the Dex client from the API: the cache keeps no secret.
func (r *Reconciler) getClient(ctx context.Context, id string) (*unstructured.Unstructured, error) {
	obj := controller.Object(controller.OAuth2ClientGVK)
	found, err := r.writer.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: naming.ToFnvLikeDex(id)}, obj)
	if !found {
		return nil, err
	}
	return obj, nil
}

func (r *Reconciler) applyClient(ctx context.Context, a Authenticator, record Record, settings Settings) error {
	current, err := r.getClient(ctx, record.ClientID)
	if err != nil {
		return err
	}
	peers := ssoPeers(a, r.state.Platform(), settings)
	want := desiredClient(a, record, settings, peers)
	content := clientContent(a, record, settings, peers)
	meta := clientMetadata(a)

	var currentObj client.Object
	if current != nil {
		currentObj = current
	}
	_, err = r.writer.Ensure(ctx, currentObj, want, managedClient(record.ClientID), func(obj client.Object) {
		u, _ := controller.AsUnstructured(obj)
		meta.Apply(u)
		for _, key := range clientFields {
			if value, ok := content[key]; ok {
				u.Object[key] = value
			} else {
				delete(u.Object, key)
			}
		}
	})
	return err
}

func (r *Reconciler) applyService(ctx context.Context, a Authenticator, settings Settings) error {
	want, meta := desiredService(a, settings)

	// The cache holds the objects of the controller only: a Service of somebody else under this
	// name is found on the create conflict and left alone.
	var currentObj client.Object
	current := &corev1.Service{}
	switch err := r.client.Get(ctx, client.ObjectKeyFromObject(want), current); {
	case err == nil:
		currentObj = current
	case !apierrors.IsNotFound(err):
		return fmt.Errorf("get Service %s: %w", client.ObjectKeyFromObject(want), err)
	}

	_, err := r.writer.Ensure(ctx, currentObj, want, managed(a.Name), func(obj client.Object) {
		svc, _ := obj.(*corev1.Service)
		meta.Apply(svc)
		// Only what the controller sets: the API server fills the rest of the spec in.
		svc.Spec.Type = want.Spec.Type
		svc.Spec.ExternalName = want.Spec.ExternalName
		svc.Spec.Ports = want.Spec.Ports
		svc.Spec.Selector = nil
	})
	return err
}

// routeSet is the routes of a DexAuthenticator: the ones it has and the ones it should have.
type routeSet struct {
	ingresses      []networkingv1.Ingress
	httpRoutes     []unstructured.Unstructured
	wantIngresses  []route
	wantHTTPRoutes []httpRoute
}

// readRoutes lists the routes the controller made for a DexAuthenticator and builds the ones the
// object should have.
func (r *Reconciler) readRoutes(ctx context.Context, a Authenticator, entry Entry, settings Settings) (routeSet, error) {
	ingresses, httpRoutes, err := listRoutes(ctx, r.client, a.Key(), r.gatewayAPI)
	if err != nil {
		return routeSet{}, err
	}
	set := routeSet{ingresses: ingresses, httpRoutes: httpRoutes, wantIngresses: desiredIngresses(a, entry.Record, entry.ThroughService, settings)}
	if r.gatewayAPI {
		set.wantHTTPRoutes = desiredHTTPRoutes(a, entry.Record, settings)
	}
	return set, nil
}

// removeStaleRoutes removes the routes a DexAuthenticator has and should not have, and takes them
// out of set. A route none of whose hosts the object names any more goes at once, even when a
// route it should have has its name: the record has lost those hosts, and the route would keep
// them from another object until it is written anew. With replaced false the call leaves a route
// that leads a host and a path the object should still have a route of: it is removed, with
// replaced true, right before the route that replaces it is written.
func (r *Reconciler) removeStaleRoutes(ctx context.Context, set *routeSet, replaced bool) error {
	const reason = "the DexAuthenticator no longer needs it"

	wantedHosts := map[string]struct{}{}
	wantedPaths := map[hostPath]struct{}{}
	for _, want := range set.wantIngresses {
		for _, path := range ingressPaths(want.ingress) {
			wantedHosts[path.host] = struct{}{}
			wantedPaths[path] = struct{}{}
		}
	}
	for _, want := range set.wantHTTPRoutes {
		for _, path := range httpRoutePaths(want.object) {
			wantedHosts[path.host] = struct{}{}
			wantedPaths[path] = struct{}{}
		}
	}
	// stale reports whether a route with the name and the paths goes now.
	stale := func(nameWanted bool, paths []hostPath) bool {
		if !slices.ContainsFunc(paths, func(path hostPath) bool {
			_, wanted := wantedHosts[path.host]
			return wanted
		}) {
			return true
		}
		if nameWanted {
			return false
		}
		return replaced || !slices.ContainsFunc(paths, func(path hostPath) bool {
			_, wanted := wantedPaths[path]
			return wanted
		})
	}

	var errs []error
	ingresses := set.ingresses[:0:0]
	for i := range set.ingresses {
		name := set.ingresses[i].Name
		nameWanted := slices.ContainsFunc(set.wantIngresses, func(want route) bool { return want.ingress.Name == name })
		if !stale(nameWanted, ingressPaths(&set.ingresses[i])) {
			ingresses = append(ingresses, set.ingresses[i])
			continue
		}
		if err := r.writer.Delete(ctx, &set.ingresses[i], reason); err != nil {
			errs = append(errs, err)
			ingresses = append(ingresses, set.ingresses[i])
		}
	}
	httpRoutes := set.httpRoutes[:0:0]
	for i := range set.httpRoutes {
		name := set.httpRoutes[i].GetName()
		nameWanted := slices.ContainsFunc(set.wantHTTPRoutes, func(want httpRoute) bool { return want.object.GetName() == name })
		if !stale(nameWanted, httpRoutePaths(&set.httpRoutes[i])) {
			httpRoutes = append(httpRoutes, set.httpRoutes[i])
			continue
		}
		if err := r.writer.Delete(ctx, &set.httpRoutes[i], reason); err != nil {
			errs = append(errs, err)
			httpRoutes = append(httpRoutes, set.httpRoutes[i])
		}
	}
	// A route that is gone is made anew by writeRoutes, not updated.
	set.ingresses, set.httpRoutes = ingresses, httpRoutes
	return errors.Join(errs...)
}

// hostPath is a host and a path a route leads.
type hostPath struct {
	host, path string
}

// ingressPaths are the hosts and the paths of the rules of an Ingress. A rule without paths leads
// its host all the same.
func ingressPaths(ingress *networkingv1.Ingress) []hostPath {
	var paths []hostPath
	for _, rule := range ingress.Spec.Rules {
		host := strings.ToLower(rule.Host)
		if rule.HTTP == nil || len(rule.HTTP.Paths) == 0 {
			paths = append(paths, hostPath{host: host})
			continue
		}
		for _, path := range rule.HTTP.Paths {
			paths = append(paths, hostPath{host: host, path: path.Path})
		}
	}
	return paths
}

// httpRoutePaths are the hostnames of an HTTPRoute with the paths of its matches.
func httpRoutePaths(route *unstructured.Unstructured) []hostPath {
	hostnames, _, _ := unstructured.NestedStringSlice(route.Object, "spec", "hostnames")
	rules, _, _ := unstructured.NestedSlice(route.Object, "spec", "rules")
	var values []string
	for _, rule := range rules {
		matches, _, _ := unstructured.NestedSlice(asMap(rule), "matches")
		for _, match := range matches {
			value, _, _ := unstructured.NestedString(asMap(match), "path", "value")
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		values = []string{""}
	}
	paths := make([]hostPath, 0, len(hostnames)*len(values))
	for _, hostname := range hostnames {
		for _, value := range values {
			paths = append(paths, hostPath{host: strings.ToLower(hostname), path: value})
		}
	}
	return paths
}

// asMap is v as an object of JSON, or nil.
func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// writeRoutes writes the routes a DexAuthenticator should have.
func (r *Reconciler) writeRoutes(ctx context.Context, a Authenticator, set routeSet) error {
	var errs []error
	for _, want := range set.wantIngresses {
		var currentObj client.Object
		if i := slices.IndexFunc(set.ingresses, func(have networkingv1.Ingress) bool { return have.Name == want.ingress.Name }); i >= 0 {
			currentObj = &set.ingresses[i]
		}
		_, err := r.writer.Ensure(ctx, currentObj, want.ingress, managed(a.Name), func(obj client.Object) {
			ingress, _ := obj.(*networkingv1.Ingress)
			want.meta.Apply(ingress)
			for _, key := range hstsAnnotations {
				if _, wanted := want.ingress.Annotations[key]; !wanted {
					delete(ingress.Annotations, key)
				}
			}
			ingress.Spec = want.ingress.Spec
		})
		errs = append(errs, err)
	}
	for _, want := range set.wantHTTPRoutes {
		var currentObj client.Object
		if i := slices.IndexFunc(set.httpRoutes, func(have unstructured.Unstructured) bool { return have.GetName() == want.object.GetName() }); i >= 0 {
			currentObj = &set.httpRoutes[i]
		}
		_, err := r.writer.Ensure(ctx, currentObj, want.object, managed(a.Name), func(obj client.Object) {
			u, _ := controller.AsUnstructured(obj)
			want.meta.Apply(u)
			u.Object["spec"] = want.object.Object["spec"]
		})
		errs = append(errs, err)
	}
	return controller.JoinRetryable(errs)
}

// listRoutes lists the Ingress objects and, with the Gateway API, the HTTPRoute objects the
// controller made for the DexAuthenticator key.
func listRoutes(ctx context.Context, c client.Reader, key types.NamespacedName, gatewayAPI bool) ([]networkingv1.Ingress, []unstructured.Unstructured, error) {
	opts := []client.ListOption{
		client.InNamespace(key.Namespace),
		client.MatchingLabels{LabelFor: key.Name, adoption.ManagedByLabel: adoption.ManagedByValue},
	}
	ingresses := &networkingv1.IngressList{}
	if err := c.List(ctx, ingresses, opts...); err != nil {
		return nil, nil, fmt.Errorf("list the Ingresses of DexAuthenticator %s: %w", key, err)
	}
	if !gatewayAPI {
		return ingresses.Items, nil, nil
	}
	routes := controller.List(HTTPRouteGVK)
	if err := c.List(ctx, routes, opts...); err != nil {
		return nil, nil, fmt.Errorf("list the HTTPRoutes of DexAuthenticator %s: %w", key, err)
	}
	return ingresses.Items, routes.Items, nil
}

// cleanup removes the objects of a DexAuthenticator that is gone or has no record: the routes
// first, then the Service, then the Dex client. The record in the table goes after the routes,
// which the table reconciler sees to.
func (r *Reconciler) cleanup(ctx context.Context, key types.NamespacedName) error {
	const reason = "the DexAuthenticator is not served"
	var errs []error

	r.forgetLoaded(key)

	// None of its routes is wanted any more.
	ingresses, httpRoutes, err := listRoutes(ctx, r.client, key, r.gatewayAPI)
	if err != nil {
		return err
	}
	if err := r.removeStaleRoutes(ctx, &routeSet{ingresses: ingresses, httpRoutes: httpRoutes}, true); err != nil {
		return err
	}

	service := &corev1.Service{}
	switch err := r.client.Get(ctx, types.NamespacedName{Namespace: key.Namespace, Name: serviceName(key.Name).name}, service); {
	case err == nil:
		if managed(key.Name)(service) {
			errs = append(errs, r.writer.Delete(ctx, service, reason))
		}
	case !apierrors.IsNotFound(err):
		errs = append(errs, fmt.Errorf("get the Service of DexAuthenticator %s: %w", key, err))
	}

	id := naming.DexAuthenticatorClientID(key.Name, key.Namespace)
	dexClient, err := r.getClient(ctx, id)
	switch {
	case err != nil:
		errs = append(errs, err)
	case dexClient != nil && managedClient(id)(dexClient):
		errs = append(errs, r.writer.Delete(ctx, dexClient, reason))
	}
	return errors.Join(errs...)
}
