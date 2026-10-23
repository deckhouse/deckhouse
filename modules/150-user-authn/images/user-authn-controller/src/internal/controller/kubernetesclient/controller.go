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

// Package kubernetesclient keeps the kubernetes OAuth2Client of Dex.
//
// The client is the audience kube-apiserver accepts tokens for, and its trustedPeers list the
// clients allowed to obtain such tokens: every DexAuthenticator and DexClient annotated with
// allow-access-to-kubernetes, and the static peers of the module configuration (publishAPI,
// kubeconfigGenerator), which the chart passes in the kubernetes-oauth2client ConfigMap. The
// secret is the one the chart keeps in the kubernetes-dex-client-app-secret Secret; it is read
// from the API, and the ConfigMap carries its checksum to trigger a reconcile when it changes.
//
// The chart rendered the client from hook values, so a DexClient or DexAuthenticator granted
// access got into trustedPeers only on some later module run. One list in one object cannot have
// two writers, so the controller owns all of it. The client exists while it has a peer.
package kubernetesclient

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
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
	"user-authn-controller/internal/controller/dexclient"
	"user-authn-controller/internal/naming"
)

const (
	ClientID = "kubernetes"
	// OAuth2ClientName is naming.ToFnvLikeDex(ClientID).
	OAuth2ClientName = "nn2wezlsnzsxizltzpzjzzeeeirsk"

	// SecretName holds the client secret under SecretKey; it is shared with the kubeconfig clients.
	SecretName = "kubernetes-dex-client-app-secret"
	SecretKey  = "secret"

	// ConfigMapName holds under PeersKey the JSON list of the peers the module configuration adds.
	ConfigMapName = "kubernetes-oauth2client"
	PeersKey      = "trustedPeers"

	appValue = "kubernetes-client"
)

// DexAuthenticatorClientID is the id the chart gives the OAuth2Client of a DexAuthenticator (see
// DexAuthenticatorClientID in the module hooks).
func DexAuthenticatorClientID(name, namespace string) string {
	return fmt.Sprintf("dex-authenticator-%s@%s", name, namespace)
}

var request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: naming.DexNamespace, Name: ClientID}}

// Reconciler keeps the kubernetes OAuth2Client.
type Reconciler struct {
	client client.Client
	reader client.Reader
	writer adoption.Writer
}

// New constructs a Reconciler.
func New(c client.Client, reader client.Reader, log logr.Logger) *Reconciler {
	return &Reconciler{client: c, reader: reader, writer: adoption.Writer{Client: c, Reader: reader, Log: log}}
}

// Register wires the reconciler onto the manager.
//
// Required RBAC: dexauthenticators and dexclients get, list, watch; configmaps get, list, watch
// and secrets get in d8-user-authn; oauth2clients list, watch, create in d8-user-authn, and get,
// update, delete of OAuth2ClientName.
func Register(mgr manager.Manager) error {
	r := New(mgr.GetClient(), mgr.GetAPIReader(), mgr.GetLogger().WithName("kubernetesclient"))
	if err := r.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup kubernetesclient controller: %w", err)
	}
	return nil
}

// InformerCacheByObject limits the ConfigMap informer to the one input. The Secret is read through
// the API and not cached: the controller may get Secrets of d8-user-authn but not list or watch
// them. The OAuth2Client informer is revocationfeed.ClientCache, shared with the feed.
func InformerCacheByObject() map[client.Object]cache.ByObject {
	byName := func(name string) cache.Config {
		return cache.Config{FieldSelector: fields.OneTermEqualSelector("metadata.name", name)}
	}
	return map[client.Object]cache.ByObject{
		&corev1.ConfigMap{}: {
			Namespaces: map[string]cache.Config{naming.DexNamespace: byName(ConfigMapName)},
		},
	}
}

// SetupWithManager reconciles the one client on a change of any input. DexAuthenticators are
// watched by metadata only: the annotation is all that is read from them.
func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	enqueue := handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return []reconcile.Request{request}
	})
	named := func(name string) builder.Predicates {
		return builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetNamespace() == naming.DexNamespace && obj.GetName() == name
		}))
	}
	authenticators := &metav1.PartialObjectMetadata{}
	authenticators.SetGroupVersionKind(controller.DexAuthenticatorGVK)

	return ctrl.NewControllerManagedBy(mgr).
		Named("kubernetesclient").
		Watches(&corev1.ConfigMap{}, enqueue, named(ConfigMapName)).
		Watches(controller.Object(controller.OAuth2ClientGVK), enqueue, named(OAuth2ClientName)).
		Watches(controller.Object(controller.DexClientGVK), enqueue).
		Watches(authenticators, enqueue, builder.OnlyMetadata).
		Complete(r)
}

// Reconcile brings the kubernetes OAuth2Client to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: ConfigMapName}, cm)
	if apierrors.IsNotFound(err) {
		// Not rendered: a chart that still renders the client itself, or the module going away.
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get ConfigMap %s: %w", ConfigMapName, err)
	}
	var static []string
	if raw := cm.Data[PeersKey]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &static); err != nil {
			return reconcile.Result{}, reconcile.TerminalError(fmt.Errorf("ConfigMap %s: %s: %w", ConfigMapName, PeersKey, err))
		}
	}

	peers, err := r.peers(ctx, static)
	if err != nil {
		return reconcile.Result{}, err
	}

	current, err := r.getClient(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}
	if len(peers) == 0 {
		if current == nil || !owned(current) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, r.writer.Delete(ctx, current, "no trusted peers")
	}

	secret := &corev1.Secret{}
	if err := r.reader.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: SecretName}, secret); err != nil {
		return reconcile.Result{}, fmt.Errorf("get Secret %s: %w", SecretName, err)
	}
	value := string(secret.Data[SecretKey])
	if value == "" {
		return reconcile.Result{}, fmt.Errorf("secret %s has no %s", SecretName, SecretKey)
	}

	return r.writer.RequeueForeign(r.apply(ctx, current, peers, value))
}

// peers lists the trusted peers: DexAuthenticators and DexClients with access, each sorted, then
// the static ones in their order.
func (r *Reconciler) peers(ctx context.Context, static []string) ([]string, error) {
	authenticators := &metav1.PartialObjectMetadataList{}
	authenticators.SetGroupVersionKind(controller.DexAuthenticatorGVK.GroupVersion().WithKind(controller.DexAuthenticatorGVK.Kind + "List"))
	if err := r.client.List(ctx, authenticators); err != nil {
		return nil, fmt.Errorf("list DexAuthenticators: %w", err)
	}
	var fromAuthenticators []string
	for _, a := range authenticators.Items {
		if allowed(&a, controller.DexAuthenticatorAllowAccessToKubernetesAnnotation) {
			fromAuthenticators = append(fromAuthenticators, DexAuthenticatorClientID(a.Name, a.Namespace))
		}
	}

	clients := controller.List(controller.DexClientGVK)
	if err := r.client.List(ctx, clients); err != nil {
		return nil, fmt.Errorf("list DexClients: %w", err)
	}
	var fromClients []string
	for i := range clients.Items {
		if c := &clients.Items[i]; allowed(c, controller.DexClientAllowAccessToKubernetesAnnotation) {
			fromClients = append(fromClients, dexclient.ClientID(c.GetName(), c.GetNamespace()))
		}
	}

	slices.Sort(fromAuthenticators)
	slices.Sort(fromClients)
	var out []string
	for _, peer := range slices.Concat(fromAuthenticators, fromClients, static) {
		if !slices.Contains(out, peer) {
			out = append(out, peer)
		}
	}
	return out, nil
}

func allowed(obj client.Object, annotation string) bool {
	if obj.GetDeletionTimestamp() != nil {
		return false
	}
	ok, err := controller.AllowsAccessToKubernetes(obj.GetAnnotations(), annotation)
	return ok && err == nil
}

// getClient returns the client from the API.
func (r *Reconciler) getClient(ctx context.Context) (*unstructured.Unstructured, error) {
	obj := controller.Object(controller.OAuth2ClientGVK)
	found, err := r.writer.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: OAuth2ClientName}, obj)
	if !found {
		return nil, err
	}
	return obj, nil
}

// owned reports whether obj is the kubernetes client of the chart or of the controller.
func owned(obj client.Object) bool {
	u, ok := controller.AsUnstructured(obj)
	if !ok {
		return false
	}
	if id, _, _ := unstructured.NestedString(u.Object, "id"); id != ClientID {
		return false
	}
	return adoption.RenderedByRelease(obj) || adoption.HasManagedMetadata(obj)
}

func (r *Reconciler) apply(ctx context.Context, current *unstructured.Unstructured, peers []string, value string) error {
	meta := adoption.Metadata{Fixed: map[string]string{
		dexclient.LabelApp:      appValue,
		dexclient.LabelHeritage: dexclient.HeritageValue,
		dexclient.LabelModule:   dexclient.ModuleValue,
	}, Track: true}
	content := map[string]any{
		"id":           ClientID,
		"name":         ClientID,
		"secret":       value,
		"trustedPeers": toAny(peers),
		// Every peer redirects to its own client, so this one needs no redirect URI.
		"redirectURIs": []any{},
	}

	want := controller.Object(controller.OAuth2ClientGVK)
	want.SetNamespace(naming.DexNamespace)
	want.SetName(OAuth2ClientName)
	meta.New(want)
	for key, v := range content {
		want.Object[key] = v
	}

	var currentObj client.Object
	if current != nil {
		currentObj = current
	}
	_, err := r.writer.Ensure(ctx, currentObj, want, owned, func(obj client.Object) {
		u, _ := controller.AsUnstructured(obj)
		meta.Apply(u)
		for key := range u.Object {
			if _, desired := content[key]; !desired && key != "apiVersion" && key != "kind" && key != "metadata" {
				delete(u.Object, key)
			}
		}
		for key, v := range content {
			u.Object[key] = v
		}
	})
	return err
}

func toAny(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}
