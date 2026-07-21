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

package bashibleapiservercert

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

var apiServiceListGVK = schema.GroupVersionKind{
	Group:   apiServiceGVK.Group,
	Version: apiServiceGVK.Version,
	Kind:    "APIServiceList",
}

func newReconciler(t *testing.T, objs ...client.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	// The aggregated APIService is handled as unstructured, so register the GVK against
	// the unstructured types for the fake client's tracker.
	scheme.AddKnownTypeWithName(apiServiceGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(apiServiceListGVK, &unstructured.UnstructuredList{})

	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}
}

func serviceObj() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: bashibleServiceName, Namespace: namespace}}
}

func apiServiceObj() *unstructured.Unstructured {
	api := &unstructured.Unstructured{}
	api.SetGroupVersionKind(apiServiceGVK)
	api.SetName(apiServiceName)
	_ = unstructured.SetNestedField(api.Object, "", "spec", "caBundle")
	return api
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
	if err := r.Client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: tlsSecretName}, s); err != nil {
		t.Fatalf("get secret: %v", err)
	}
	return s
}

func getAPIServiceCABundle(t *testing.T, r *Reconciler) string {
	t.Helper()
	api := &unstructured.Unstructured{}
	api.SetGroupVersionKind(apiServiceGVK)
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: apiServiceName}, api); err != nil {
		t.Fatalf("get apiservice: %v", err)
	}
	v, _, err := unstructured.NestedString(api.Object, "spec", "caBundle")
	if err != nil {
		t.Fatalf("read caBundle: %v", err)
	}
	return v
}

// Without the bashible-api Service the reconcile is a no-op: no Secret is made.
func TestReconcile_NoService_Noop(t *testing.T) {
	r := newReconciler(t)
	doReconcile(t, r)
	s := &corev1.Secret{}
	err := r.Client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: tlsSecretName}, s)
	if err == nil {
		t.Fatal("expected no Secret to be created without the bashible-api Service")
	}
}

// With the Service present the reconcile issues an Opaque Secret and stamps the CA into the
// aggregated APIService.
func TestReconcile_GeneratesSecretAndInjects(t *testing.T) {
	r := newReconciler(t, serviceObj(), apiServiceObj())
	doReconcile(t, r)

	s := getSecret(t, r)
	if s.Type != corev1.SecretTypeOpaque {
		t.Fatalf("expected Opaque Secret, got %q", s.Type)
	}
	ca := s.Data["ca.crt"]
	if len(ca) == 0 || len(s.Data["apiserver.crt"]) == 0 || len(s.Data["apiserver.key"]) == 0 {
		t.Fatal("expected ca.crt/apiserver.crt/apiserver.key to be populated")
	}
	if _, err := parseCert(ca); err != nil {
		t.Fatalf("ca.crt is not a valid certificate: %v", err)
	}

	want := base64.StdEncoding.EncodeToString(ca)
	if got := getAPIServiceCABundle(t, r); got != want {
		t.Fatalf("APIService caBundle not injected: got %q want %q", got, want)
	}
}

// The generated leaf carries the loopback IP SAN and the service DNS SAN.
func TestReconcile_LeafSANs(t *testing.T) {
	r := newReconciler(t, serviceObj())
	doReconcile(t, r)
	leaf, err := parseCert(getSecret(t, r).Data["apiserver.crt"])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	if !stringsEqual(leaf.DNSNames, []string{sanDNS}) {
		t.Fatalf("unexpected DNS SANs: %v", leaf.DNSNames)
	}
	if len(leaf.IPAddresses) != 1 || leaf.IPAddresses[0].String() != sanIP {
		t.Fatalf("unexpected IP SANs: %v", leaf.IPAddresses)
	}
}

// A valid stored certificate is reused verbatim (zero-disruption): the private key must not
// change across a reconcile.
func TestReconcile_ReusesValidSecret(t *testing.T) {
	r := newReconciler(t, serviceObj())
	bundle, err := generateBundle(certCN, desiredSANs())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	pre := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: namespace},
		Type:       corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"ca.crt":        bundle.caPEM,
			"apiserver.crt": bundle.certPEM,
			"apiserver.key": bundle.keyPEM,
		},
	}
	if err := r.Client.Create(context.Background(), pre); err != nil {
		t.Fatalf("seed secret: %v", err)
	}

	doReconcile(t, r)

	s := getSecret(t, r)
	if !bytes.Equal(s.Data["apiserver.key"], bundle.keyPEM) {
		t.Fatal("valid certificate must be reused, key changed")
	}
}

// A Secret whose leaf does not match the desired SANs is regenerated.
func TestReconcile_RegeneratesOnSANsMismatch(t *testing.T) {
	r := newReconciler(t, serviceObj())
	stale, err := generateBundle(certCN, []string{"wrong.example.com"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	pre := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: namespace},
		Type:       corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"ca.crt":        stale.caPEM,
			"apiserver.crt": stale.certPEM,
			"apiserver.key": stale.keyPEM,
		},
	}
	if err := r.Client.Create(context.Background(), pre); err != nil {
		t.Fatalf("seed secret: %v", err)
	}

	doReconcile(t, r)

	s := getSecret(t, r)
	if bytes.Equal(s.Data["apiserver.key"], stale.keyPEM) {
		t.Fatal("certificate with mismatched SANs must be regenerated")
	}
	leaf, err := parseCert(s.Data["apiserver.crt"])
	if err != nil {
		t.Fatalf("parse regenerated leaf: %v", err)
	}
	if !stringsEqual(leaf.DNSNames, []string{sanDNS}) {
		t.Fatalf("regenerated leaf has wrong DNS SANs: %v", leaf.DNSNames)
	}
}

// A missing APIService is tolerated: the Secret is still issued.
func TestReconcile_MissingAPIService_NoError(t *testing.T) {
	r := newReconciler(t, serviceObj())
	doReconcile(t, r)
	getSecret(t, r) // fails the test if absent
}

func TestDesiredSANs(t *testing.T) {
	if !stringsEqual(desiredSANs(), []string{sanIP, sanDNS}) {
		t.Fatalf("unexpected SANs: %v", desiredSANs())
	}
}

// bundleValid accepts a fresh bundle and rejects mismatched SANs or an unparsable CA.
func TestBundleValid(t *testing.T) {
	sans := desiredSANs()
	good, err := generateBundle(certCN, sans)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !bundleValid(good.caPEM, good.certPEM, sans) {
		t.Fatal("freshly generated bundle must be valid")
	}
	if bundleValid(good.caPEM, good.certPEM, []string{sanDNS}) {
		t.Fatal("bundle missing the IP SAN must be invalid")
	}
	if bundleValid([]byte("garbage"), good.certPEM, sans) {
		t.Fatal("bundle with unparsable CA must be invalid")
	}
}

// The event filter passes only the three managed objects.
func TestSetupWatches_Predicate(t *testing.T) {
	r := &Reconciler{}
	w := &captureWatcher{}
	r.SetupWatches(w)

	if w.predicate == nil {
		t.Fatal("expected an event filter predicate")
	}
	for _, name := range []string{bashibleServiceName, tlsSecretName, apiServiceName} {
		obj := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name}}
		if !w.predicate.Create(createEvent(obj)) {
			t.Fatalf("predicate must pass %q", name)
		}
	}
	other := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "unrelated"}}
	if w.predicate.Create(createEvent(other)) {
		t.Fatal("predicate must drop unrelated objects")
	}
	if len(w.watched) != 2 {
		t.Fatalf("expected 2 secondary watches, got %d", len(w.watched))
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
