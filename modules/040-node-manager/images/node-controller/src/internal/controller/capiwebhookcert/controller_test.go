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

package capiwebhookcert

import (
	"bytes"
	"context"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/deckhouse/node-controller/internal/register"
)

func newReconciler(t *testing.T, objs ...client.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := admissionregistrationv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add admissionregistration scheme: %v", err)
	}
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}
}

func service() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: webhookServiceName, Namespace: capiNamespace}}
}

func validatingConfig() *admissionregistrationv1.ValidatingWebhookConfiguration {
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: validatingWebhookName},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{Name: "a.example.com"},
			{Name: "b.example.com"},
		},
	}
}

func mutatingConfig() *admissionregistrationv1.MutatingWebhookConfiguration {
	return &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: mutatingWebhookName},
		Webhooks:   []admissionregistrationv1.MutatingWebhook{{Name: "c.example.com"}},
	}
}

func doReconcile(t *testing.T, r *Reconciler) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func getSecret(t *testing.T, r *Reconciler) *corev1.Secret {
	t.Helper()
	s := &corev1.Secret{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Namespace: capiNamespace, Name: tlsSecretName}, s); err != nil {
		t.Fatalf("get secret: %v", err)
	}
	return s
}

// Without the webhook Service (CAPI disabled) the reconcile is a no-op: no Secret is made.
func TestReconcile_NoService_Noop(t *testing.T) {
	r := newReconciler(t)
	doReconcile(t, r)
	s := &corev1.Secret{}
	err := r.Client.Get(context.Background(), types.NamespacedName{Namespace: capiNamespace, Name: tlsSecretName}, s)
	if err == nil {
		t.Fatal("expected no Secret to be created without the webhook Service")
	}
}

// With the Service present the reconcile issues a TLS Secret and stamps the CA into both
// webhook configurations.
func TestReconcile_GeneratesSecretAndInjects(t *testing.T) {
	r := newReconciler(t, service(), validatingConfig(), mutatingConfig())
	doReconcile(t, r)

	s := getSecret(t, r)
	if s.Type != corev1.SecretTypeTLS {
		t.Fatalf("expected kubernetes.io/tls Secret, got %q", s.Type)
	}
	ca := s.Data["ca.crt"]
	if len(ca) == 0 || len(s.Data["tls.crt"]) == 0 || len(s.Data["tls.key"]) == 0 {
		t.Fatal("expected ca.crt/tls.crt/tls.key to be populated")
	}
	if _, err := parseCert(ca); err != nil {
		t.Fatalf("ca.crt is not a valid certificate: %v", err)
	}

	vc := &admissionregistrationv1.ValidatingWebhookConfiguration{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: validatingWebhookName}, vc); err != nil {
		t.Fatalf("get validating config: %v", err)
	}
	for _, wh := range vc.Webhooks {
		if !bytes.Equal(wh.ClientConfig.CABundle, ca) {
			t.Fatalf("validating webhook %q caBundle not injected", wh.Name)
		}
	}

	mc := &admissionregistrationv1.MutatingWebhookConfiguration{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: mutatingWebhookName}, mc); err != nil {
		t.Fatalf("get mutating config: %v", err)
	}
	for _, wh := range mc.Webhooks {
		if !bytes.Equal(wh.ClientConfig.CABundle, ca) {
			t.Fatalf("mutating webhook %q caBundle not injected", wh.Name)
		}
	}
}

// A valid stored certificate is reused verbatim (zero-disruption): the private key must not
// change across a reconcile.
func TestReconcile_ReusesValidSecret(t *testing.T) {
	r := newReconciler(t, service())
	sans := r.desiredSANs(context.Background())
	bundle, err := generateBundle(certCN, sans)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	pre := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: capiNamespace},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"ca.crt":  bundle.caPEM,
			"tls.crt": bundle.certPEM,
			"tls.key": bundle.keyPEM,
		},
	}
	if err := r.Client.Create(context.Background(), pre); err != nil {
		t.Fatalf("seed secret: %v", err)
	}

	doReconcile(t, r)

	s := getSecret(t, r)
	if !bytes.Equal(s.Data["tls.key"], bundle.keyPEM) {
		t.Fatal("valid certificate must be reused, key changed")
	}
}

// A Secret whose leaf does not match the desired SANs is regenerated.
func TestReconcile_RegeneratesOnSANsMismatch(t *testing.T) {
	r := newReconciler(t, service())
	stale, err := generateBundle(certCN, []string{"wrong.example.com"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	pre := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: capiNamespace},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"ca.crt":  stale.caPEM,
			"tls.crt": stale.certPEM,
			"tls.key": stale.keyPEM,
		},
	}
	if err := r.Client.Create(context.Background(), pre); err != nil {
		t.Fatalf("seed secret: %v", err)
	}

	doReconcile(t, r)

	s := getSecret(t, r)
	if bytes.Equal(s.Data["tls.key"], stale.keyPEM) {
		t.Fatal("certificate with mismatched SANs must be regenerated")
	}
	leaf, err := parseCert(s.Data["tls.crt"])
	if err != nil {
		t.Fatalf("parse regenerated leaf: %v", err)
	}
	if !sansEqual(leaf.DNSNames, r.desiredSANs(context.Background())) {
		t.Fatalf("regenerated leaf has wrong SANs: %v", leaf.DNSNames)
	}
}

// Missing webhook configurations are tolerated: the Secret is still issued.
func TestReconcile_MissingWebhookConfigs_NoError(t *testing.T) {
	r := newReconciler(t, service())
	doReconcile(t, r)
	getSecret(t, r) // fails the test if absent
}

// desiredSANs falls back to cluster.local without the cluster-configuration Secret.
func TestDesiredSANs_DefaultDomain(t *testing.T) {
	r := newReconciler(t, service())
	sans := r.desiredSANs(context.Background())
	want := []string{
		"capi-webhook-service.d8-cloud-instance-manager",
		"capi-webhook-service.d8-cloud-instance-manager.svc",
		"capi-webhook-service.d8-cloud-instance-manager.cluster.local",
		"capi-webhook-service.d8-cloud-instance-manager.svc.cluster.local",
	}
	if !sansEqual(sans, want) {
		t.Fatalf("unexpected SANs: %v", sans)
	}
}

// desiredSANs honours the clusterDomain from d8-cluster-configuration.
func TestDesiredSANs_CustomDomain(t *testing.T) {
	cfg := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: clusterConfigSecretName, Namespace: kubeSystemNS},
		Data:       map[string][]byte{clusterConfigKey: []byte("clusterDomain: corp.internal\n")},
	}
	r := newReconciler(t, service(), cfg)
	sans := r.desiredSANs(context.Background())
	found := false
	for _, s := range sans {
		if s == "capi-webhook-service.d8-cloud-instance-manager.svc.corp.internal" {
			found = true
		}
	}
	if !found {
		t.Fatalf("clusterDomain not honoured: %v", sans)
	}
}

// bundleValid rejects a certificate issued for a different SAN set.
func TestBundleValid(t *testing.T) {
	sans := []string{"a", "b"}
	good, err := generateBundle(certCN, sans)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !bundleValid(good.caPEM, good.certPEM, sans) {
		t.Fatal("freshly generated bundle must be valid")
	}
	if bundleValid(good.caPEM, good.certPEM, []string{"a", "c"}) {
		t.Fatal("bundle with mismatched SANs must be invalid")
	}
	if bundleValid([]byte("garbage"), good.certPEM, sans) {
		t.Fatal("bundle with unparsable CA must be invalid")
	}
}

// The event filter passes only the four managed objects.
func TestSetupWatches_Predicate(t *testing.T) {
	r := &Reconciler{}
	w := &captureWatcher{}
	r.SetupWatches(w)

	if w.predicate == nil {
		t.Fatal("expected an event filter predicate")
	}
	for _, name := range []string{webhookServiceName, tlsSecretName, validatingWebhookName, mutatingWebhookName} {
		obj := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name}}
		if !w.predicate.Create(createEvent(obj)) {
			t.Fatalf("predicate must pass %q", name)
		}
	}
	other := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "unrelated"}}
	if w.predicate.Create(createEvent(other)) {
		t.Fatal("predicate must drop unrelated objects")
	}
	if len(w.watched) != 3 {
		t.Fatalf("expected 3 secondary watches, got %d", len(w.watched))
	}
}

func createEvent(obj client.Object) event.CreateEvent { return event.CreateEvent{Object: obj} }

type captureWatcher struct {
	predicate predicate.Predicate
	watched   []client.Object
}

func (w *captureWatcher) Owns(_ client.Object, _ ...builder.OwnsOption) {}
func (w *captureWatcher) Watches(obj client.Object, _ handler.EventHandler, _ ...builder.WatchesOption) {
	w.watched = append(w.watched, obj)
}
func (w *captureWatcher) WatchesRawSource(_ source.Source)      {}
func (w *captureWatcher) WithEventFilter(p predicate.Predicate) { w.predicate = p }
