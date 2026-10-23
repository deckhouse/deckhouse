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

// Package dexclient registers DexClients in Dex.
//
// For every DexClient the controller keeps three objects: the dex-client-<name> Secret with the
// client secret in the namespace of the DexClient, and two OAuth2Clients in d8-user-authn, one per
// client ID (the current one and the legacy one with a colon), carrying the same secret. The
// Secret is the source of truth for the secret: if it is missing, the value is recovered from an
// OAuth2Client, and only when neither exists a new one is generated, so that adopting the objects
// the module chart used to render keeps every application's credentials.
//
// The chart rendered these objects from values a hook computed, so a DexClient created while the
// module was running got its Secret and Dex client only on some later module run (addon-operator
// drops the rerun a hook requests when a kube config change is queued ahead of the running
// ModuleRun). The controller reacts to the DexClient itself instead.
//
// The reconciler runs alone, in the dexclient-controller Deployment (the dexclient command of the
// image), under a ServiceAccount of its own: writing the Secret of a DexClient in any namespace
// takes Secret rights in every namespace, and that pod serves nothing on the network but health
// and metrics. user-authn-controller, which serves the revocation feed, has no Secret rights
// beyond reading the client secret of the kubernetes OAuth2Client.
package dexclient

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/adoption"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const (
	// recorderName is the dexclient-controller Deployment the reconciler runs in.
	recorderName = "dexclient-controller"

	ReasonInvalidSpec       = "InvalidSpec"
	ReasonInvalidAnnotation = "InvalidAnnotation"
	ReasonForeignObject     = "ForeignObject"
	ReasonApplyError        = "ApplyError"
	actionReconcile         = "Reconcile"

	dexClientAPIVersion  = "deckhouse.io/v1"
	dexClientKind        = "DexClient"
	dexClientGroupPrefix = "deckhouse.io/"
)

// Reconciler keeps the Secret and the OAuth2Clients of a DexClient.
type Reconciler struct {
	client client.Client
	// reader bypasses the cache, to confirm a DexClient is gone before its objects are removed.
	reader   client.Reader
	writer   adoption.Writer
	recorder events.EventRecorder
}

// New constructs a Reconciler.
func New(c client.Client, reader client.Reader, recorder events.EventRecorder, log logr.Logger) *Reconciler {
	return &Reconciler{
		client:   c,
		reader:   reader,
		writer:   adoption.Writer{Client: c, Reader: reader, Log: log},
		recorder: recorder,
	}
}

// Register wires the DexClient reconciler onto the manager.
//
// Required RBAC: dexclients get, list, watch; secrets get, list, watch, create, update, delete
// cluster-wide; oauth2clients get, list, watch, create, update, delete in d8-user-authn; events
// create, patch.
func Register(mgr manager.Manager) error {
	r := New(mgr.GetClient(), mgr.GetAPIReader(), mgr.GetEventRecorder(recorderName), mgr.GetLogger().WithName("dexclient"))
	if err := r.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup dexclient controller: %w", err)
	}
	return nil
}

// InformerCacheByObject scopes the Secret informer to the Secrets the controller writes: a
// cluster-wide Secret informer would keep every Secret of the cluster in memory. Secrets are
// watched by metadata only and always read through the API: anyone can put the label on a Secret
// of their own, and its data must not end up in the memory of the controller. The OAuth2Client
// informer is revocationfeed.ClientCache, shared with the feed; the clients are read through the
// API too.
func InformerCacheByObject() map[client.Object]cache.ByObject {
	return map[client.Object]cache.ByObject{
		&corev1.Secret{}: {
			Namespaces: map[string]cache.Config{cache.AllNamespaces: {LabelSelector: adoption.ManagedSelector()}},
		},
	}
}

// SetupWithManager watches DexClients and, mapped back to them, their Secrets and OAuth2Clients,
// so that a changed or removed object is repaired and the objects of a DexClient deleted while
// the controller was down are found on its start: the OAuth2Client cache holds every client of
// d8-user-authn, the ones the chart rendered included, and its first list maps each to its DexClient.
func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("dexclient").
		For(controller.Object(controller.DexClientGVK)).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(mapSecret),
			builder.WithPredicates(predicate.NewPredicateFuncs(isCredentialsSecret)), builder.OnlyMetadata).
		Watches(controller.Object(controller.OAuth2ClientGVK), handler.EnqueueRequestsFromMapFunc(mapOAuth2Client),
			builder.WithPredicates(predicate.NewPredicateFuncs(isDexClientOAuth2Client))).
		Complete(r)
}

func isCredentialsSecret(obj client.Object) bool {
	objLabels := obj.GetLabels()
	return objLabels[LabelApp] == AppValue && objLabels[LabelName] == NameValue && strings.HasPrefix(obj.GetName(), SecretPrefix)
}

func isDexClientOAuth2Client(obj client.Object) bool {
	return obj.GetNamespace() == naming.DexNamespace && (obj.GetLabels()[LabelApp] == AppValue || adoption.HasManagedMetadata(obj))
}

func mapSecret(_ context.Context, obj client.Object) []reconcile.Request {
	if !isCredentialsSecret(obj) {
		return nil
	}
	name := strings.TrimPrefix(obj.GetName(), SecretPrefix)
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: obj.GetNamespace(), Name: name}}}
}

func mapOAuth2Client(_ context.Context, obj client.Object) []reconcile.Request {
	u, ok := controller.AsUnstructured(obj)
	if !ok || !isDexClientOAuth2Client(obj) {
		return nil
	}
	id, _, _ := unstructured.NestedString(u.Object, "id")
	key, ok := ParseClientID(id)
	if !ok {
		return nil
	}
	return []reconcile.Request{{NamespacedName: key}}
}

// Reconcile brings the Secret and the OAuth2Clients of the DexClient named in req to the desired
// state, or removes them when the DexClient is gone.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	obj := controller.Object(controller.DexClientGVK)
	err := r.client.Get(ctx, req.NamespacedName, obj)
	if apierrors.IsNotFound(err) {
		// The cache may lag behind a DexClient that was just created while an event of its Secret
		// arrived first; removing the objects on a stale miss would rotate the credentials.
		live := controller.Object(controller.DexClientGVK)
		switch err := r.reader.Get(ctx, req.NamespacedName, live); {
		case err == nil:
			return reconcile.Result{}, fmt.Errorf("DexClient %s exists but is not in the cache yet", req.NamespacedName)
		case !apierrors.IsNotFound(err):
			return reconcile.Result{}, fmt.Errorf("get DexClient %s: %w", req.NamespacedName, err)
		}
		return reconcile.Result{}, r.cleanup(ctx, req.Name, req.Namespace)
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get DexClient %s: %w", req.NamespacedName, err)
	}
	if obj.GetDeletionTimestamp() != nil {
		// The Secret goes with the DexClient by its owner reference; the OAuth2Clients are removed
		// on the event of the DexClient disappearing.
		return reconcile.Result{}, nil
	}

	c, err := Decode(obj)
	if err != nil {
		r.event(obj, ReasonInvalidSpec, err)
		return reconcile.Result{}, reconcile.TerminalError(err)
	}
	if _, err := controller.AllowsAccessToKubernetes(c.Annotations, AllowAccessToKubernetesAnnotation); err != nil {
		r.event(obj, ReasonInvalidAnnotation, err)
	}

	if err := r.apply(ctx, c); err != nil {
		reason := ReasonApplyError
		if errors.Is(err, adoption.ErrForeign) {
			reason = ReasonForeignObject
		}
		r.event(obj, reason, err)
		return r.writer.RequeueForeign(err)
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) event(obj runtime.Object, reason string, err error) {
	if r.recorder == nil {
		return
	}
	r.recorder.Eventf(obj, nil, corev1.EventTypeWarning, reason, actionReconcile, "%s", err.Error())
}

func (r *Reconciler) apply(ctx context.Context, c Client) error {
	secret, err := r.getSecret(ctx, c.Namespace, c.Name)
	if err != nil {
		return err
	}

	ids := []string{ClientID(c.Name, c.Namespace), LegacyClientID(c.Name, c.Namespace)}
	clients := make([]*unstructured.Unstructured, len(ids))
	for i, id := range ids {
		if clients[i], err = r.getOAuth2Client(ctx, id); err != nil {
			return err
		}
	}

	candidate := pickSecret(secret, clients)
	if candidate == "" {
		if candidate, err = GenerateSecret(); err != nil {
			return err
		}
	}

	// The Secret first: once it is written the value survives a failure of the clients, which are
	// repaired from it on the retry. The value comes back from the Secret actually in the cluster:
	// one the label-scoped cache did not hold (its labels overridden through secretMetadata by the
	// chart) is found on the create conflict and its value wins.
	value, err := r.applySecret(ctx, c, secret, candidate)
	if err != nil {
		return err
	}
	var errs []error
	for i, id := range ids {
		errs = append(errs, r.applyOAuth2Client(ctx, c, id, clients[i], value))
	}
	return joinRetryable(errs)
}

// joinRetryable joins errs so that one terminal error does not stop the retry of another:
// errors.Join of a terminal error is terminal as a whole.
func joinRetryable(errs []error) error {
	var retry, terminal []error
	for _, err := range errs {
		switch {
		case err == nil:
		case errors.Is(err, reconcile.TerminalError(nil)):
			terminal = append(terminal, err)
		default:
			retry = append(retry, err)
		}
	}
	if len(retry) == 0 {
		return errors.Join(terminal...)
	}
	for _, err := range terminal {
		retry = append(retry, errors.New(err.Error()))
	}
	return errors.Join(retry...)
}

// pickSecret is the value of the Secret, or of the first OAuth2Client that has one, or empty.
func pickSecret(secret *corev1.Secret, clients []*unstructured.Unstructured) string {
	if secret != nil {
		if value := string(secret.Data[SecretKey]); value != "" {
			return value
		}
	}
	for _, obj := range clients {
		if obj == nil {
			continue
		}
		if value, _, _ := unstructured.NestedString(obj.Object, "secret"); value != "" {
			return value
		}
	}
	return ""
}

// getSecret and getOAuth2Client read through the API. A Secret the chart rendered is missing from
// the label-scoped Secret cache until it is adopted, and taking it for absent would replace its
// value or leave it behind a deleted DexClient; the OAuth2Client cache keeps no secret.
func (r *Reconciler) getSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: namespace, Name: SecretName(name)}
	err := r.reader.Get(ctx, key, secret)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Secret %s: %w", key, err)
	}
	return secret, nil
}

func (r *Reconciler) getOAuth2Client(ctx context.Context, id string) (*unstructured.Unstructured, error) {
	obj := controller.Object(controller.OAuth2ClientGVK)
	found, err := r.writer.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: OAuth2ClientName(id)}, obj)
	if !found {
		return nil, err
	}
	return obj, nil
}

func ownerReference(c Client) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: dexClientAPIVersion,
		Kind:       dexClientKind,
		Name:       c.Name,
		UID:        c.UID,
		Controller: ptr.To(true),
	}
}

func isDexClientRef(ref metav1.OwnerReference) bool {
	return ref.Kind == dexClientKind && strings.HasPrefix(ref.APIVersion, dexClientGroupPrefix)
}

// withOwner replaces the DexClient owner references (a stale one of a DexClient recreated under
// the same name included) with the one of c and keeps the others.
func withOwner(refs []metav1.OwnerReference, owner metav1.OwnerReference) []metav1.OwnerReference {
	out := make([]metav1.OwnerReference, 0, len(refs)+1)
	for _, ref := range refs {
		if !isDexClientRef(ref) {
			out = append(out, ref)
		}
	}
	return append(out, owner)
}

// secretOwnedBy reports whether the Secret was written for this DexClient by the controller or by
// the module chart: anything else with the name of the Secret is not taken over.
func secretOwnedBy(c Client) func(client.Object) bool {
	return func(obj client.Object) bool {
		for _, ref := range obj.GetOwnerReferences() {
			if ref.UID == c.UID {
				return true
			}
		}
		return adoption.RenderedByRelease(obj) || (adoption.HasManagedMetadata(obj) && isCredentialsSecret(obj))
	}
}

// applySecret writes the Secret and returns the client secret it holds.
func (r *Reconciler) applySecret(ctx context.Context, c Client, current *corev1.Secret, value string) (string, error) {
	meta := secretMetadata(c)

	want := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            SecretName(c.Name),
			Namespace:       c.Namespace,
			OwnerReferences: []metav1.OwnerReference{ownerReference(c)},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{SecretKey: []byte(value)},
	}
	meta.New(want)

	var currentObj client.Object
	if current != nil {
		currentObj = current
	}
	live, err := r.writer.Ensure(ctx, currentObj, want, secretOwnedBy(c), func(obj client.Object) {
		s, _ := obj.(*corev1.Secret)
		meta.Apply(s)
		s.OwnerReferences = withOwner(s.OwnerReferences, ownerReference(c))
		if s.Data == nil {
			s.Data = make(map[string][]byte, 1)
		}
		// A value already in the Secret is kept: it is what the application uses.
		if len(s.Data[SecretKey]) == 0 {
			s.Data[SecretKey] = []byte(value)
		}
	})
	if err != nil {
		return "", err
	}
	s, _ := live.(*corev1.Secret)
	return string(s.Data[SecretKey]), nil
}

// oauth2ClientOwnedBy reports whether obj is the OAuth2Client of the client id.
func oauth2ClientOwnedBy(id string) func(client.Object) bool {
	return func(obj client.Object) bool {
		u, ok := controller.AsUnstructured(obj)
		if !ok {
			return false
		}
		if have, _, _ := unstructured.NestedString(u.Object, "id"); have != id {
			return false
		}
		// The release annotation or the managed-metadata marker is enough, as for the Secret: the
		// labels are what the reconcile puts back.
		objLabels := u.GetLabels()
		return objLabels[LabelApp] == AppValue && objLabels[LabelModule] == ModuleValue ||
			adoption.RenderedByRelease(obj) || adoption.HasManagedMetadata(obj)
	}
}

func (r *Reconciler) applyOAuth2Client(ctx context.Context, c Client, id string, current *unstructured.Unstructured, value string) error {
	meta := oauth2ClientMetadata()
	content := oauth2ClientContent(c, id, value)

	want := controller.Object(controller.OAuth2ClientGVK)
	want.SetNamespace(naming.DexNamespace)
	want.SetName(OAuth2ClientName(id))
	meta.New(want)
	for key, v := range content {
		want.Object[key] = v
	}

	var currentObj client.Object
	if current != nil {
		currentObj = current
	}
	_, err := r.writer.Ensure(ctx, currentObj, want, oauth2ClientOwnedBy(id), func(obj client.Object) {
		u, _ := controller.AsUnstructured(obj)
		meta.Apply(u)
		for _, key := range oauth2ClientFields {
			if v, ok := content[key]; ok {
				u.Object[key] = v
			} else {
				delete(u.Object, key)
			}
		}
	})
	return err
}

// cleanup removes the objects of a DexClient that is gone. The Secret normally goes by its owner
// reference; this covers a Secret the chart rendered that the controller never adopted, and the
// OAuth2Clients, which cannot have an owner in another namespace.
func (r *Reconciler) cleanup(ctx context.Context, name, namespace string) error {
	var errs []error
	for _, id := range []string{ClientID(name, namespace), LegacyClientID(name, namespace)} {
		obj, err := r.getOAuth2Client(ctx, id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if obj == nil || !oauth2ClientOwnedBy(id)(obj) {
			continue
		}
		errs = append(errs, r.writer.Delete(ctx, obj, "DexClient deleted"))
	}

	secret, err := r.getSecret(ctx, namespace, name)
	switch {
	case err != nil:
		errs = append(errs, err)
	case secret != nil && secretOfDeletedClient(secret, name):
		errs = append(errs, r.writer.Delete(ctx, secret, "DexClient deleted"))
	}
	return errors.Join(errs...)
}

// secretOfDeletedClient reports whether secret belongs to the gone DexClient name: it is owned by
// a DexClient of that name, or rendered by the chart, or written by the controller.
func secretOfDeletedClient(secret *corev1.Secret, name string) bool {
	for _, ref := range secret.OwnerReferences {
		if isDexClientRef(ref) {
			return ref.Name == name
		}
	}
	return adoption.RenderedByRelease(secret) || adoption.HasManagedMetadata(secret)
}
