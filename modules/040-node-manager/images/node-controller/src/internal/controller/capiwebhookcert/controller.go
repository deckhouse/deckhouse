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

// Package capiwebhookcert issues the self-signed serving certificate for the
// capi-controller-manager admission webhook and injects its CA into the CAPI webhook
// configurations. It replaces the OnBeforeHelm hook generate_capi_webhook_certs.
//
// The hook generated a self-signed CA + leaf into helm values (nodeManager.internal.
// capiControllerManagerWebhookCert) that helm then wrote into the Secret capi-webhook-tls
// and stamped into every webhook's clientConfig.caBundle. nc cannot feed helm values, so
// instead it owns the Secret directly and patches the caBundle into the two webhook
// configurations. helm renders them with an empty caBundle; because the rendered value
// never changes, helm's three-way merge leaves nc's patched CA in place across converges.
//
// CAPI enablement is detected by the presence of the capi-webhook-service Service: it is
// created early by helm (no dependency on this Secret), so gating on it avoids the deadlock
// where the capi-controller-manager Deployment waits for the Secret while the Secret waits
// for something the Deployment must create.
package capiwebhookcert

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/register"
)

const (
	capiNamespace      = "d8-cloud-instance-manager"
	kubeSystemNS       = "kube-system"
	tlsSecretName      = "capi-webhook-tls"
	webhookServiceName = "capi-webhook-service"

	mutatingWebhookName   = "capi-mutating-webhook-configuration"
	validatingWebhookName = "capi-validating-webhook-configuration"

	certCN = "capi-controller-manager-webhook"

	clusterConfigSecretName = "d8-cluster-configuration"
	clusterConfigKey        = "cluster-configuration.yaml"
	defaultClusterDomain    = "cluster.local"

	// renewalCheckInterval bounds how long a cert can sit past its 6-month renewal
	// threshold without an event. The hook re-checked on every OnBeforeHelm run.
	renewalCheckInterval = 12 * time.Hour
)

func init() {
	register.RegisterController("capi-webhook-cert", &corev1.Service{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// One predicate for every watched kind: react only to the four objects we manage.
	// The webhook configurations are cluster-scoped, so a name check is enough.
	names := map[string]bool{
		webhookServiceName:    true,
		tlsSecretName:         true,
		validatingWebhookName: true,
		mutatingWebhookName:   true,
	}
	w.WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return names[obj.GetName()]
	}))

	enqueue := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: capiNamespace, Name: tlsSecretName}}}
	})
	w.Watches(&corev1.Secret{}, enqueue)
	w.Watches(&admissionregistrationv1.ValidatingWebhookConfiguration{}, enqueue)
	w.Watches(&admissionregistrationv1.MutatingWebhookConfiguration{}, enqueue)
}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Gate on CAPI being enabled. The webhook Service exists only when helm rendered the
	// capi-controller-manager; without it there is nothing to serve a cert for.
	svc := &corev1.Service{}
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: capiNamespace, Name: webhookServiceName}, svc)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting Service %s/%s: %w", capiNamespace, webhookServiceName, err)
	}

	sans := r.desiredSANs(ctx)

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
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: capiNamespace, Name: tlsSecretName}, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("getting Secret %s/%s: %w", capiNamespace, tlsSecretName, err)
	}
	if err == nil {
		ca := secret.Data["ca.crt"]
		crt := secret.Data["tls.crt"]
		if bundleValid(ca, crt, sans) {
			return ca, nil
		}
	}

	logger.Info("issuing capi-controller-manager webhook certificate")
	bundle, err := generateBundle(sans)
	if err != nil {
		return nil, err
	}
	if err := r.writeSecret(ctx, bundle); err != nil {
		return nil, err
	}
	return bundle.caPEM, nil
}

func (r *Reconciler) writeSecret(ctx context.Context, bundle certBundle) error {
	desired := &corev1.Secret{}
	desired.SetName(tlsSecretName)
	desired.SetNamespace(capiNamespace)
	desired.Type = corev1.SecretTypeTLS
	desired.Labels = map[string]string{"heritage": "deckhouse", "module": "node-manager"}
	desired.Data = map[string][]byte{
		"ca.crt":  bundle.caPEM,
		"tls.crt": bundle.certPEM,
		"tls.key": bundle.keyPEM,
	}

	existing := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: capiNamespace, Name: tlsSecretName}, existing)
	if apierrors.IsNotFound(err) {
		if err := r.Client.Create(ctx, desired); err != nil {
			return fmt.Errorf("creating Secret %s/%s: %w", capiNamespace, tlsSecretName, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting Secret %s/%s: %w", capiNamespace, tlsSecretName, err)
	}

	patch := existing.DeepCopy()
	if patch.Labels == nil {
		patch.Labels = map[string]string{}
	}
	for k, v := range desired.Labels {
		patch.Labels[k] = v
	}
	patch.Type = corev1.SecretTypeTLS
	patch.Data = desired.Data
	if err := r.Client.Update(ctx, patch); err != nil {
		return fmt.Errorf("updating Secret %s/%s: %w", capiNamespace, tlsSecretName, err)
	}
	return nil
}

// injectCABundle stamps the CA into every webhook of both CAPI webhook configurations.
func (r *Reconciler) injectCABundle(ctx context.Context, logger logr.Logger, caPEM []byte) error {
	validating := &admissionregistrationv1.ValidatingWebhookConfiguration{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: validatingWebhookName}, validating); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting ValidatingWebhookConfiguration %s: %w", validatingWebhookName, err)
		}
	} else {
		changed := false
		patch := validating.DeepCopy()
		for i := range patch.Webhooks {
			if !bytesEqual(patch.Webhooks[i].ClientConfig.CABundle, caPEM) {
				patch.Webhooks[i].ClientConfig.CABundle = caPEM
				changed = true
			}
		}
		if changed {
			if err := r.Client.Patch(ctx, patch, client.MergeFrom(validating)); err != nil {
				return fmt.Errorf("patching ValidatingWebhookConfiguration %s: %w", validatingWebhookName, err)
			}
			logger.V(1).Info("injected CA bundle", "config", validatingWebhookName)
		}
	}

	mutating := &admissionregistrationv1.MutatingWebhookConfiguration{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: mutatingWebhookName}, mutating); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("getting MutatingWebhookConfiguration %s: %w", mutatingWebhookName, err)
		}
		return nil
	}
	changed := false
	patch := mutating.DeepCopy()
	for i := range patch.Webhooks {
		if !bytesEqual(patch.Webhooks[i].ClientConfig.CABundle, caPEM) {
			patch.Webhooks[i].ClientConfig.CABundle = caPEM
			changed = true
		}
	}
	if changed {
		if err := r.Client.Patch(ctx, patch, client.MergeFrom(mutating)); err != nil {
			return fmt.Errorf("patching MutatingWebhookConfiguration %s: %w", mutatingWebhookName, err)
		}
		logger.V(1).Info("injected CA bundle", "config", mutatingWebhookName)
	}
	return nil
}

// desiredSANs mirrors the hook's SANs list, including the clusterDomain-suffixed variants.
func (r *Reconciler) desiredSANs(ctx context.Context) []string {
	base := webhookServiceName + "." + capiNamespace
	clusterDomain := r.readClusterDomain(ctx)
	return []string{
		base,
		base + ".svc",
		base + "." + clusterDomain,
		base + ".svc." + clusterDomain,
	}
}

type clusterConfiguration struct {
	ClusterDomain string `json:"clusterDomain"`
}

func (r *Reconciler) readClusterDomain(ctx context.Context) string {
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: kubeSystemNS, Name: clusterConfigSecretName}, secret); err != nil {
		return defaultClusterDomain
	}
	raw, ok := secret.Data[clusterConfigKey]
	if !ok {
		return defaultClusterDomain
	}
	if decoded, err := base64.StdEncoding.DecodeString(string(raw)); err == nil {
		raw = decoded
	}
	cfg := &clusterConfiguration{}
	if err := sigsyaml.Unmarshal(raw, cfg); err != nil || cfg.ClusterDomain == "" {
		return defaultClusterDomain
	}
	return cfg.ClusterDomain
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
