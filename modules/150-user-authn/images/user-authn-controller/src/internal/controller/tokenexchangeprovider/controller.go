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

// Package tokenexchangeprovider keeps the Dex connector of every TokenExchangeProvider.
//
// The connector lives in the Dex storage (connectors.dex.coreos.com in d8-user-authn), which Dex
// reads on every request to it, so a change needs no restart of Dex. A provider is deleted through
// a finalizer: the controller first writes a Revocation of type connector, which the user-authz
// revocation check reads to refuse every token issued through the connector, then removes the
// connector and the finalizer.
package tokenexchangeprovider

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
)

const (
	recorderName    = "token-exchange-provider-controller"
	actionReconcile = "Reconcile"

	// ReasonInvalidSpec: the spec cannot become a connector; the previous connector is kept.
	ReasonInvalidSpec = "InvalidSpec"
	// ReasonApplyError: writing the connector, the revocation or the status failed.
	ReasonApplyError = "ApplyError"
	// ReasonPrefixConflict: an earlier provider gives the same user or group names; the provider
	// gets no connector.
	ReasonPrefixConflict = "PrefixConflict"
	// ReasonDiscoveryFailed: the discovery document or the key set of the issuer cannot be read;
	// the previous connector, if any, is kept.
	ReasonDiscoveryFailed = "DiscoveryFailed"

	// discoveryRetry is how soon a failed discovery probe is tried again.
	discoveryRetry = time.Minute

	conditionReady = "Ready"
)

// Reconciler keeps the connector of a TokenExchangeProvider.
type Reconciler struct {
	client client.Client
	// reader bypasses the cache: a provider missing from the cache is confirmed before its
	// connector is revoked, and the connector is read before it is written.
	reader   client.Reader
	recorder events.EventRecorder
	now      func() time.Time
}

// New constructs a Reconciler.
func New(c client.Client, reader client.Reader, recorder events.EventRecorder) *Reconciler {
	return &Reconciler{client: c, reader: reader, recorder: recorder, now: time.Now}
}

// Register wires the reconciler onto the manager.
//
// Required RBAC: tokenexchangeproviders get, list, watch, update, patch, and their status update;
// connectors get, list, watch, create, update, delete and revocations create in d8-user-authn;
// events create, patch.
func Register(mgr manager.Manager) error {
	r := New(mgr.GetClient(), mgr.GetAPIReader(), mgr.GetEventRecorder(recorderName))
	if err := r.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup tokenexchangeprovider controller: %w", err)
	}
	return nil
}

// InformerCacheByObject scopes the Connector informer to the connectors of providers: the
// connectors of the Dex config are not the controller's.
func InformerCacheByObject() map[client.Object]cache.ByObject {
	selector := labels.Everything()
	if req, err := labels.NewRequirement(ProviderLabel, selection.Exists, nil); err == nil {
		selector = labels.NewSelector().Add(*req)
	}
	return map[client.Object]cache.ByObject{
		controller.Object(controller.ConnectorGVK): {
			Namespaces: map[string]cache.Config{naming.DexNamespace: {LabelSelector: selector}},
		},
	}
}

// SetupWithManager watches the providers and, mapped back to them, their connectors, so that a
// connector changed or removed by hand is repaired and one left by a provider deleted past its
// finalizer is revoked on the start of the controller.
func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("tokenexchangeprovider").
		For(controller.Object(controller.TokenExchangeProviderGVK)).
		Watches(controller.Object(controller.TokenExchangeProviderGVK), handler.EnqueueRequestsFromMapFunc(r.mapAllProviders),
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(controller.Object(controller.ConnectorGVK), handler.EnqueueRequestsFromMapFunc(mapConnector),
			builder.WithPredicates(predicate.NewPredicateFuncs(isProviderConnector))).
		Complete(r)
}

func isProviderConnector(obj client.Object) bool {
	return obj.GetNamespace() == naming.DexNamespace && obj.GetLabels()[ProviderLabel] != ""
}

func mapConnector(_ context.Context, obj client.Object) []reconcile.Request {
	name := obj.GetLabels()[ProviderLabel]
	if name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: name}}}
}

// Reconcile brings the connector of the provider named in req to its spec, revokes and removes it
// when the provider is deleted, and removes connectors left without a provider.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	obj := controller.Object(controller.TokenExchangeProviderGVK)
	err := r.client.Get(ctx, req.NamespacedName, obj)
	if apierrors.IsNotFound(err) {
		// The cache may lag behind a provider that was just created; revoking its connector on a
		// stale miss would cut off a working provider.
		live := controller.Object(controller.TokenExchangeProviderGVK)
		switch err := r.reader.Get(ctx, req.NamespacedName, live); {
		case err == nil:
			return reconcile.Result{}, fmt.Errorf("TokenExchangeProvider %s exists but is not in the cache yet", req.Name)
		case !apierrors.IsNotFound(err):
			return reconcile.Result{}, fmt.Errorf("get TokenExchangeProvider %s: %w", req.Name, err)
		}
		return reconcile.Result{}, r.removeOrphans(ctx, req.Name, "")
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get TokenExchangeProvider %s: %w", req.Name, err)
	}

	if obj.GetDeletionTimestamp() != nil {
		return reconcile.Result{}, r.finalize(ctx, obj)
	}
	id, err := r.connectorID(ctx, obj)
	if err != nil {
		return reconcile.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(obj, Finalizer) {
		controllerutil.AddFinalizer(obj, Finalizer)
		if err := r.client.Update(ctx, obj); err != nil {
			return reconcile.Result{}, fmt.Errorf("add finalizer to %s: %w", obj.GetName(), err)
		}
	}

	spec, invalid := Decode(obj)
	if invalid == nil {
		invalid = validate(spec)
	}

	// The conflict comes before the validity of the spec: a provider whose new spec is invalid
	// keeps its connector, and that connector must still give way to an earlier provider. Its
	// names are those of the connector Dex serves as well as those of the spec.
	own, err := r.connectorNames(ctx, obj.GetName())
	if err != nil {
		return reconcile.Result{}, err
	}
	if decoded, err := Decode(obj); err == nil {
		own = append(own, namesOf(decoded))
	}
	other, where, err := r.conflictingProvider(ctx, obj, own)
	if err != nil {
		return reconcile.Result{}, err
	}
	if other != "" {
		// The names stay with the earlier provider; a connector this one had is revoked.
		message := fmt.Sprintf("the %s overlaps the names of TokenExchangeProvider %s, created earlier: two issuers would give the same names", where, other)
		r.event(obj, ReasonPrefixConflict, errors.New(message))
		return reconcile.Result{}, errors.Join(r.removeOrphans(ctx, obj.GetName(), ""),
			r.setStatus(ctx, obj, id, metav1.ConditionFalse, ReasonPrefixConflict, message))
	}

	if invalid != nil {
		r.event(obj, ReasonInvalidSpec, invalid)
		// A failed status write is retried; the spec itself waits for a change.
		if statusErr := r.setStatus(ctx, obj, id, metav1.ConditionFalse, ReasonInvalidSpec, invalid.Error()); statusErr != nil {
			return reconcile.Result{}, statusErr
		}
		return reconcile.Result{}, reconcile.TerminalError(invalid)
	}

	// The connector is written before the probe: a change that narrows the provider (a client
	// removed, a stricter claimValidation) must reach Dex while the issuer is down too, and Dex
	// reads the discovery document only at an exchange anyway.
	if err := r.applyConnector(ctx, obj.GetName(), id, spec); err != nil {
		r.event(obj, ReasonApplyError, err)
		return reconcile.Result{}, errors.Join(err, r.setStatus(ctx, obj, id, metav1.ConditionFalse, ReasonApplyError, err.Error()))
	}
	// A provider recreated under the same name has a new UID and so a new connector; the old one
	// is revoked like the connector of any deleted provider.
	if err := r.removeOrphans(ctx, obj.GetName(), id); err != nil {
		return reconcile.Result{}, err
	}

	// A provider ready at this generation is not probed again: every provider is reconciled when
	// any other changes, and a hung issuer would hold the worker each time.
	if !readyAt(obj) {
		if err := probeDiscovery(ctx, spec); err != nil {
			r.event(obj, ReasonDiscoveryFailed, err)
			return reconcile.Result{RequeueAfter: discoveryRetry},
				r.setStatus(ctx, obj, id, metav1.ConditionFalse, ReasonDiscoveryFailed, err.Error())
		}
	}
	return reconcile.Result{}, r.setStatus(ctx, obj, id, metav1.ConditionTrue, "ConnectorReady", "")
}

// readyAt reports whether the provider is ready at its current generation.
func readyAt(obj *unstructured.Unstructured) bool {
	cond := meta.FindStatusCondition(readConditions(obj), conditionReady)
	return cond != nil && cond.Status == metav1.ConditionTrue && cond.ObservedGeneration == obj.GetGeneration()
}

// connectorID returns the ID of the connector the provider has to have: the one in its status
// while that is not revoked, else the first of its IDs that is not. A connector removed because
// the provider lost its names to an earlier one is revoked, and a revocation refuses every token
// issued through the connector until it expires, so the connector the provider gets back once the
// conflict is gone needs another ID.
//
// An ID in the status is taken only if it is one of the IDs of this provider: anyone who can
// write the status could otherwise point the provider at the connector of another one, which the
// provider would then overwrite and, once deleted, revoke.
func (r *Reconciler) connectorID(ctx context.Context, obj *unstructured.Unstructured) (string, error) {
	current, _, _ := unstructured.NestedString(obj.Object, "status", "connectorID")
	if current != "" && ownsConnectorID(obj, current) {
		revoked, err := r.revoked(ctx, current)
		if err != nil || !revoked {
			return current, err
		}
	}
	for n := range maxConnectorIDs {
		id := connectorIDAt(obj.GetName(), string(obj.GetUID()), n)
		revoked, err := r.revoked(ctx, id)
		if err != nil {
			return "", err
		}
		if !revoked {
			return id, nil
		}
	}
	return "", fmt.Errorf("all %d connector IDs of %s are revoked", maxConnectorIDs, obj.GetName())
}

// ownsConnectorID reports whether id is one of the IDs of the provider.
func ownsConnectorID(obj *unstructured.Unstructured, id string) bool {
	for n := range maxConnectorIDs {
		if connectorIDAt(obj.GetName(), string(obj.GetUID()), n) == id {
			return true
		}
	}
	return false
}

// maxConnectorIDs bounds the IDs connectorID tries; revocations expire, so a provider never needs
// more than a few.
const maxConnectorIDs = 100

// revoked reports whether a revocation of the connector id exists. It reads past the cache: a
// revocation that was just written must not be missed.
func (r *Reconciler) revoked(ctx context.Context, id string) (bool, error) {
	rev := controller.Object(controller.RevocationGVK)
	err := r.reader.Get(ctx, client.ObjectKey{Namespace: naming.DexNamespace, Name: revocation.ConnectorName(id)}, rev)
	switch {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err):
		return false, nil
	}
	return false, fmt.Errorf("get revocation of connector %s: %w", id, err)
}

func (r *Reconciler) event(obj *unstructured.Unstructured, reason string, err error) {
	if r.recorder == nil {
		return
	}
	r.recorder.Eventf(obj, nil, corev1.EventTypeWarning, reason, actionReconcile, "%s", err.Error())
}

// applyConnector writes the connector when it differs from the spec. Every write makes Dex open
// the connector again on every replica, so an unchanged connector is left alone.
func (r *Reconciler) applyConnector(ctx context.Context, name, id string, spec Spec) error {
	want, err := desiredConnector(name, id, spec)
	if err != nil {
		return err
	}

	// Read past the cache: it holds only the connectors with the provider label, and one whose
	// label was removed would look missing, so every create would fail with AlreadyExists and the
	// connector would never be repaired.
	current := controller.Object(controller.ConnectorGVK)
	err = r.reader.Get(ctx, client.ObjectKey{Namespace: naming.DexNamespace, Name: id}, current)
	if apierrors.IsNotFound(err) {
		if err := r.client.Create(ctx, want); err != nil {
			return fmt.Errorf("create connector %s: %w", id, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get connector %s: %w", id, err)
	}

	changed := !reflect.DeepEqual(current.GetLabels(), want.GetLabels())
	for _, field := range connectorFields {
		if !reflect.DeepEqual(current.Object[field], want.Object[field]) {
			changed = true
			break
		}
	}
	if !changed {
		return nil
	}
	current.SetLabels(want.GetLabels())
	for _, field := range connectorFields {
		if v, ok := want.Object[field]; ok {
			current.Object[field] = v
		} else {
			delete(current.Object, field)
		}
	}
	if err := r.client.Update(ctx, current); err != nil {
		return fmt.Errorf("update connector %s: %w", id, err)
	}
	return nil
}

// finalize revokes the connectors of a deleted provider, removes them, and releases the provider.
// The connector of its status is revoked even when the label that lists it is gone.
func (r *Reconciler) finalize(ctx context.Context, obj *unstructured.Unstructured) error {
	if !controllerutil.ContainsFinalizer(obj, Finalizer) {
		return nil
	}
	id, _, _ := unstructured.NestedString(obj.Object, "status", "connectorID")
	if id == "" || !ownsConnectorID(obj, id) {
		id = ConnectorID(obj.GetName(), string(obj.GetUID()))
	}
	if err := r.revokeAndDelete(ctx, id); err != nil {
		r.event(obj, ReasonApplyError, err)
		return err
	}
	if err := r.removeOrphans(ctx, obj.GetName(), ""); err != nil {
		return err
	}
	controllerutil.RemoveFinalizer(obj, Finalizer)
	if err := r.client.Update(ctx, obj); err != nil {
		return fmt.Errorf("remove finalizer from %s: %w", obj.GetName(), err)
	}
	return nil
}

// removeOrphans revokes and removes the connectors of the provider name other than keep: the
// connector of an earlier object of the same name, or every one when the provider is gone.
func (r *Reconciler) removeOrphans(ctx context.Context, name, keep string) error {
	list := controller.List(controller.ConnectorGVK)
	if err := r.client.List(ctx, list, client.InNamespace(naming.DexNamespace), client.MatchingLabels{ProviderLabel: name}); err != nil {
		return fmt.Errorf("list connectors of %s: %w", name, err)
	}
	var errs []error
	for i := range list.Items {
		id := list.Items[i].GetName()
		if id == keep {
			continue
		}
		errs = append(errs, r.revokeAndDelete(ctx, id))
	}
	return errors.Join(errs...)
}

// revokeAndDelete writes the revocation of the connector, then deletes it. The revocation comes
// first: a token issued through the connector must be refused even if the deletion fails.
func (r *Reconciler) revokeAndDelete(ctx context.Context, id string) error {
	rev := revocation.Revocation{
		Name:        revocation.ConnectorName(id),
		Type:        revocation.TypeConnector,
		ConnectorID: id,
		Clients:     []string{},
		Reason:      revocation.ReasonProviderDeleted,
		Source:      revocation.SourceController,
		CreatedAt:   r.now(),
	}
	obj, err := rev.Object()
	if err != nil {
		return err
	}
	if err := r.client.Create(ctx, obj); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create revocation of connector %s: %w", id, err)
	}

	connector := controller.Object(controller.ConnectorGVK)
	connector.SetNamespace(naming.DexNamespace)
	connector.SetName(id)
	if err := r.client.Delete(ctx, connector); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("delete connector %s: %w", id, err)
	}
	return nil
}

// setStatus writes the connector ID and the Ready condition when they changed.
func (r *Reconciler) setStatus(ctx context.Context, obj *unstructured.Unstructured, id string, status metav1.ConditionStatus, reason, message string) error {
	conditions := readConditions(obj)
	changed := meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: obj.GetGeneration(),
	})
	current, _, _ := unstructured.NestedString(obj.Object, "status", "connectorID")
	observed, _, _ := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")
	if !changed && current == id && observed == obj.GetGeneration() {
		return nil
	}

	items := make([]any, 0, len(conditions))
	for _, c := range conditions {
		item := map[string]any{
			"type":               c.Type,
			"status":             string(c.Status),
			"reason":             c.Reason,
			"lastTransitionTime": c.LastTransitionTime.UTC().Format(time.RFC3339),
			"observedGeneration": c.ObservedGeneration,
		}
		if c.Message != "" {
			item["message"] = c.Message
		}
		items = append(items, item)
	}
	obj.Object["status"] = map[string]any{
		"connectorID":        id,
		"observedGeneration": obj.GetGeneration(),
		"conditions":         items,
	}
	if err := r.client.Status().Update(ctx, obj); err != nil {
		return fmt.Errorf("update status of %s: %w", obj.GetName(), err)
	}
	return nil
}

func readConditions(obj *unstructured.Unstructured) []metav1.Condition {
	raw, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	conditions := make([]metav1.Condition, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		c := metav1.Condition{}
		c.Type, _ = m["type"].(string)
		status, _ := m["status"].(string)
		c.Status = metav1.ConditionStatus(status)
		c.Reason, _ = m["reason"].(string)
		c.Message, _ = m["message"].(string)
		// A typed client decodes the integer as int64, a JSON one as float64.
		switch g := m["observedGeneration"].(type) {
		case int64:
			c.ObservedGeneration = g
		case float64:
			c.ObservedGeneration = int64(g)
		}
		if t, ok := m["lastTransitionTime"].(string); ok {
			if parsed, err := time.Parse(time.RFC3339, t); err == nil {
				c.LastTransitionTime = metav1.NewTime(parsed)
			}
		}
		conditions = append(conditions, c)
	}
	return conditions
}
