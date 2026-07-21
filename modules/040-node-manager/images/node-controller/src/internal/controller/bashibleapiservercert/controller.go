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

// Package bashibleapiservercert issues the self-signed serving certificate for the
// bashible-apiserver and injects its CA into the aggregated APIService. It replaces the
// OnBeforeHelm hook gen_bashible_apiserver_certs.
//
// The hook generated a self-signed CA + leaf into helm values (nodeManager.internal.
// bashibleApiServer{CA,Crt,Key}) that helm then wrote into the Secret bashible-api-server-tls
// and stamped into the APIService v1alpha1.bashible.deckhouse.io spec.caBundle. nc cannot
// feed helm values, so instead it owns the Secret directly and patches the caBundle into the
// APIService. helm renders the APIService with an empty caBundle; because the rendered value
// never changes, helm's three-way merge leaves nc's patched CA in place across converges.
//
// bashible-apiserver is a core component of node-manager and is always deployed, so there is
// no enablement gate — the reconcile simply anchors on the always-present Service bashible-api
// (a no-op if it is somehow absent). nc does not depend on bashible-apiserver being up, so
// owning its serving Secret introduces no bootstrap deadlock; on a fresh cluster the
// bashible-apiserver pod mount-retries until nc has created the Secret.
package bashibleapiservercert

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/node-controller/internal/register"
)

const (
	namespace           = "d8-cloud-instance-manager"
	tlsSecretName       = "bashible-api-server-tls"
	bashibleServiceName = "bashible-api"
	apiServiceName      = "v1alpha1.bashible.deckhouse.io"

	certCN = "node-manager"

	// SANs are fixed (the hook passed them literally, without any clusterDomain variants):
	// the loopback address and the in-cluster service DNS name.
	sanIP  = "127.0.0.1"
	sanDNS = "bashible-api.d8-cloud-instance-manager.svc"

	// renewalCheckInterval bounds how long a cert can sit past its 6-month renewal
	// threshold without an event. The hook re-checked on every OnBeforeHelm run.
	renewalCheckInterval = 12 * time.Hour
)

// apiServiceGVK is the aggregated APIService. It is patched via unstructured because the
// kube-aggregator apiregistration types are not part of node-controller's scheme.
var apiServiceGVK = schema.GroupVersionKind{
	Group:   "apiregistration.k8s.io",
	Version: "v1",
	Kind:    "APIService",
}

func init() {
	register.RegisterController("bashible-apiserver-cert", &corev1.Service{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// React only to the three objects we manage. The APIService is cluster-scoped, so a
	// name check is enough for it as well.
	names := map[string]bool{
		bashibleServiceName: true,
		tlsSecretName:       true,
		apiServiceName:      true,
	}
	w.WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return names[obj.GetName()]
	}))

	enqueue := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: namespace, Name: tlsSecretName}}}
	})
	w.Watches(&corev1.Secret{}, enqueue)

	apiService := &unstructured.Unstructured{}
	apiService.SetGroupVersionKind(apiServiceGVK)
	w.Watches(apiService, enqueue)
}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Anchor on the bashible-apiserver Service. It is rendered unconditionally with the
	// component; if it is absent there is nothing to serve a cert for.
	svc := &corev1.Service{}
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: bashibleServiceName}, svc)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting Service %s/%s: %w", namespace, bashibleServiceName, err)
	}

	sans := desiredSANs()

	caPEM, err := r.ensureSecret(ctx, logger, sans)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.injectCABundle(ctx, logger, caPEM); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: renewalCheckInterval}, nil
}

// ensureSecret returns the CA PEM to inject, reusing the stored bundle while it is still
// valid and regenerating it otherwise.
func (r *Reconciler) ensureSecret(ctx context.Context, logger logr.Logger, sans []string) ([]byte, error) {
	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: tlsSecretName}, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("getting Secret %s/%s: %w", namespace, tlsSecretName, err)
	}
	if err == nil {
		ca := secret.Data["ca.crt"]
		crt := secret.Data["apiserver.crt"]
		if bundleValid(ca, crt, sans) {
			return ca, nil
		}
	}

	logger.Info("issuing bashible-apiserver serving certificate")
	bundle, err := generateBundle(certCN, sans)
	if err != nil {
		return nil, err
	}
	if err := r.writeSecret(ctx, bundle); err != nil {
		return nil, err
	}
	return bundle.caPEM, nil
}

func (r *Reconciler) writeSecret(ctx context.Context, bundle certBundle) error {
	data := map[string][]byte{
		"ca.crt":        bundle.caPEM,
		"apiserver.crt": bundle.certPEM,
		"apiserver.key": bundle.keyPEM,
	}
	labels := map[string]string{"heritage": "deckhouse", "module": "node-manager"}

	existing := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: tlsSecretName}, existing)
	if apierrors.IsNotFound(err) {
		desired := &corev1.Secret{}
		desired.SetName(tlsSecretName)
		desired.SetNamespace(namespace)
		desired.Type = corev1.SecretTypeOpaque
		desired.Labels = labels
		desired.Data = data
		if err := r.Client.Create(ctx, desired); err != nil {
			return fmt.Errorf("creating Secret %s/%s: %w", namespace, tlsSecretName, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting Secret %s/%s: %w", namespace, tlsSecretName, err)
	}

	patch := existing.DeepCopy()
	if patch.Labels == nil {
		patch.Labels = map[string]string{}
	}
	for k, v := range labels {
		patch.Labels[k] = v
	}
	patch.Type = corev1.SecretTypeOpaque
	patch.Data = data
	if err := r.Client.Update(ctx, patch); err != nil {
		return fmt.Errorf("updating Secret %s/%s: %w", namespace, tlsSecretName, err)
	}
	return nil
}

// injectCABundle stamps the CA into the aggregated APIService spec.caBundle. In the
// unstructured object caBundle is a base64-encoded string (the JSON form of the typed
// []byte field).
func (r *Reconciler) injectCABundle(ctx context.Context, logger logr.Logger, caPEM []byte) error {
	api := &unstructured.Unstructured{}
	api.SetGroupVersionKind(apiServiceGVK)
	if err := r.Client.Get(ctx, types.NamespacedName{Name: apiServiceName}, api); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("getting APIService %s: %w", apiServiceName, err)
	}

	want := base64.StdEncoding.EncodeToString(caPEM)
	current, _, err := unstructured.NestedString(api.Object, "spec", "caBundle")
	if err != nil {
		return fmt.Errorf("reading APIService %s caBundle: %w", apiServiceName, err)
	}
	if current == want {
		return nil
	}

	patch := api.DeepCopy()
	if err := unstructured.SetNestedField(patch.Object, want, "spec", "caBundle"); err != nil {
		return fmt.Errorf("setting APIService %s caBundle: %w", apiServiceName, err)
	}
	if err := r.Client.Patch(ctx, patch, client.MergeFrom(api)); err != nil {
		return fmt.Errorf("patching APIService %s: %w", apiServiceName, err)
	}
	logger.V(1).Info("injected CA bundle", "apiService", apiServiceName)
	return nil
}

// desiredSANs mirrors the hook's fixed SANs list.
func desiredSANs() []string {
	return []string{sanIP, sanDNS}
}
