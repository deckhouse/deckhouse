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

package tokenexchangeprovider

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"maps"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
)

const (
	testName = "peer-moscow"
	testUID  = "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
	testID   = "tep-peer-moscow-1a2b3c4d"
)

var testRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: testName}}

func peeringSpec() map[string]any {
	return map[string]any{
		"issuer": "https://kubernetes.default.svc.cluster.local",
		"keys": map[string]any{"jwks": map[string]any{"keys": []any{
			map[string]any{"kty": "RSA", "kid": "moscow", "n": "AQAB", "e": "AQAB"},
		}}},
		"audiences": []any{map[string]any{"prefix": "peering.deckhouse.io/peer/b/"}},
		"identity": map[string]any{
			"username": map[string]any{"claim": "sub", "prefix": "peering:peer:moscow:"},
			"groups": []any{map[string]any{
				"claim": "aud", "trimPrefix": "peering.deckhouse.io/peer/b/cap/", "prefix": "peering:peer:moscow:cap:",
			}},
		},
		"clients": []any{map[string]any{"dexClient": map[string]any{"name": "peering-receiver", "namespace": "d8-peering"}}},
	}
}

func provider(spec map[string]any, finalizers ...string) *unstructured.Unstructured {
	u := controller.Object(controller.TokenExchangeProviderGVK)
	u.SetName(testName)
	u.SetUID(types.UID(testUID))
	u.SetGeneration(1)
	u.SetFinalizers(finalizers)
	u.Object["spec"] = spec
	return u
}

func newTestReconciler(t *testing.T, objs ...client.Object) (*Reconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "deckhouse.io", Version: "v1alpha1"}, {Group: "dex.coreos.com", Version: "v1"}})
	mapper.Add(controller.TokenExchangeProviderGVK, meta.RESTScopeRoot)
	mapper.Add(controller.ConnectorGVK, meta.RESTScopeNamespace)
	mapper.Add(controller.RevocationGVK, meta.RESTScopeNamespace)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(mapper).
		WithObjects(objs...).
		WithStatusSubresource(controller.Object(controller.TokenExchangeProviderGVK)).
		Build()
	r := New(c, c, nil)
	r.now = func() time.Time { return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC) }
	return r, c
}

func getConnector(t *testing.T, c client.Client, id string) (*unstructured.Unstructured, bool) {
	t.Helper()
	obj := controller.Object(controller.ConnectorGVK)
	err := c.Get(t.Context(), client.ObjectKey{Namespace: naming.DexNamespace, Name: id}, obj)
	if apierrors.IsNotFound(err) {
		return nil, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return obj, true
}

func connectorConfigOf(t *testing.T, obj *unstructured.Unstructured) map[string]any {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(obj.Object["config"].(string))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	return config
}

func TestConnectorID(t *testing.T) {
	if got := ConnectorID("gitlab-ci", testUID); got != "tep-gitlab-ci-1a2b3c4d" {
		t.Errorf("ConnectorID = %q", got)
	}

	// The longest name the schema admits, with dots, still makes a label value and the name of
	// the connector revocation.
	id := ConnectorID(strings.Repeat("a.b", 16)+"cd", testUID)
	if errs := validation.IsValidLabelValue(id); len(errs) != 0 {
		t.Errorf("ConnectorID %q is not a label value: %v", id, errs)
	}
	if errs := validation.IsDNS1123Subdomain(revocation.ConnectorName(id)); len(errs) != 0 {
		t.Errorf("revocation name of %q: %v", id, errs)
	}
}

func TestReconcileCreatesTheConnector(t *testing.T) {
	r, c := newTestReconciler(t, provider(peeringSpec()))

	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	connector, ok := getConnector(t, c, testID)
	if !ok {
		t.Fatal("no connector")
	}
	if connector.Object["id"] != testID || connector.Object["type"] != "tokenexchange" {
		t.Errorf("id, type = %v, %v", connector.Object["id"], connector.Object["type"])
	}
	if !reflect.DeepEqual(connector.Object["grantTypes"], []any{grantTypeTokenExchange}) {
		t.Errorf("grantTypes = %v", connector.Object["grantTypes"])
	}
	if connector.GetLabels()[ProviderLabel] != testName {
		t.Errorf("labels = %v", connector.GetLabels())
	}
	config := connectorConfigOf(t, connector)
	if !reflect.DeepEqual(config["clients"], []any{"dex-client-peering-receiver@d8-peering"}) {
		t.Errorf("clients = %v", config["clients"])
	}
	if !reflect.DeepEqual(config["reservedPrefixes"], []any{"peering:peer:moscow:", "peering:peer:moscow:cap:"}) {
		t.Errorf("reservedPrefixes = %v", config["reservedPrefixes"])
	}

	obj := controller.Object(controller.TokenExchangeProviderGVK)
	if err := c.Get(t.Context(), testRequest.NamespacedName, obj); err != nil {
		t.Fatal(err)
	}
	if got := obj.GetFinalizers(); !reflect.DeepEqual(got, []string{Finalizer}) {
		t.Errorf("finalizers = %v", got)
	}
	if id, _, _ := unstructured.NestedString(obj.Object, "status", "connectorID"); id != testID {
		t.Errorf("status.connectorID = %q", id)
	}
	if cond := meta.FindStatusCondition(readConditions(obj), conditionReady); cond == nil || cond.Status != metav1.ConditionTrue {
		t.Errorf("Ready = %+v", cond)
	}

	// An unchanged provider leaves the connector alone: every write reopens it in Dex.
	version := connector.GetResourceVersion()
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	again, _ := getConnector(t, c, testID)
	if again.GetResourceVersion() != version {
		t.Error("an unchanged provider rewrote its connector")
	}

	// Nor its status: the condition keeps its observedGeneration, so it reads back unchanged.
	providerVersion := obj.GetResourceVersion()
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("third Reconcile: %v", err)
	}
	if err := c.Get(t.Context(), testRequest.NamespacedName, obj); err != nil {
		t.Fatal(err)
	}
	if obj.GetResourceVersion() != providerVersion {
		t.Error("an unchanged provider rewrote its status")
	}
	if cond := meta.FindStatusCondition(readConditions(obj), conditionReady); cond == nil || cond.ObservedGeneration != 1 {
		t.Errorf("Ready = %+v", cond)
	}
}

func TestReconcileUpdatesTheConnector(t *testing.T) {
	r, c := newTestReconciler(t, provider(peeringSpec(), Finalizer))
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatal(err)
	}

	obj := controller.Object(controller.TokenExchangeProviderGVK)
	if err := c.Get(t.Context(), testRequest.NamespacedName, obj); err != nil {
		t.Fatal(err)
	}
	spec := peeringSpec()
	spec["allowedIdentities"] = map[string]any{"groups": []any{"peering:peer:moscow:cap:virtualization"}}
	obj.Object["spec"] = spec
	if err := c.Update(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatal(err)
	}

	connector, _ := getConnector(t, c, testID)
	if !reflect.DeepEqual(connector.Object["allowedGroups"], []any{"peering:peer:moscow:cap:virtualization"}) {
		t.Errorf("allowedGroups = %v", connector.Object["allowedGroups"])
	}
}

func TestReconcileRefusesAPrivateKey(t *testing.T) {
	spec := peeringSpec()
	spec["keys"] = map[string]any{"jwks": map[string]any{"keys": []any{
		map[string]any{"kty": "RSA", "n": "AQAB", "e": "AQAB", "d": "secret"},
	}}}
	r, c := newTestReconciler(t, provider(spec, Finalizer))

	_, err := r.Reconcile(t.Context(), testRequest)
	if !errors.Is(err, reconcile.TerminalError(nil)) {
		t.Fatalf("Reconcile = %v, want a terminal error", err)
	}
	if _, ok := getConnector(t, c, testID); ok {
		t.Error("a connector was written for an invalid spec")
	}
	obj := controller.Object(controller.TokenExchangeProviderGVK)
	if err := c.Get(t.Context(), testRequest.NamespacedName, obj); err != nil {
		t.Fatal(err)
	}
	if cond := meta.FindStatusCondition(readConditions(obj), conditionReady); cond == nil || cond.Reason != ReasonInvalidSpec {
		t.Errorf("Ready = %+v", cond)
	}
}

func TestDeletionRevokesTheConnector(t *testing.T) {
	r, c := newTestReconciler(t, provider(peeringSpec(), Finalizer))
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatal(err)
	}

	obj := controller.Object(controller.TokenExchangeProviderGVK)
	if err := c.Get(t.Context(), testRequest.NamespacedName, obj); err != nil {
		t.Fatal(err)
	}
	if err := c.Delete(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile after delete: %v", err)
	}

	if _, ok := getConnector(t, c, testID); ok {
		t.Error("the connector of a deleted provider is still there")
	}
	assertRevoked(t, c, testID)
	if err := c.Get(t.Context(), testRequest.NamespacedName, obj); !apierrors.IsNotFound(err) {
		t.Errorf("the provider is still there after its finalizer: %v", err)
	}
}

// A connector whose provider is gone without its finalizer, as after a forced removal, is revoked
// and removed when the controller meets it.
func TestOrphanConnectorIsRevoked(t *testing.T) {
	orphan, err := desiredConnector(testName, testID, Spec{})
	if err != nil {
		t.Fatal(err)
	}
	r, c := newTestReconciler(t, orphan)

	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if _, ok := getConnector(t, c, testID); ok {
		t.Error("the orphan connector is still there")
	}
	assertRevoked(t, c, testID)
}

func assertRevoked(t *testing.T, c client.Client, id string) {
	t.Helper()
	obj := controller.Object(controller.RevocationGVK)
	if err := c.Get(t.Context(), client.ObjectKey{Namespace: naming.DexNamespace, Name: revocation.ConnectorName(id)}, obj); err != nil {
		t.Fatalf("revocation of %s: %v", id, err)
	}
	rev, err := revocation.FromObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	if rev.Type != revocation.TypeConnector || rev.ConnectorID != id || rev.Reason != revocation.ReasonProviderDeleted {
		t.Errorf("revocation = %+v", rev)
	}
}

func TestOwnedPrefixes(t *testing.T) {
	tests := []struct {
		name     string
		identity Identity
		expected []string
	}{
		{name: "no reserved prefix", identity: Identity{Username: ClaimMapping{Claim: "sub", Prefix: "gitlab-ci:"}}},
		{
			name:     "username prefix",
			identity: Identity{Username: ClaimMapping{Claim: "sub", Prefix: "peering:peer:a:"}},
			expected: []string{"peering:peer:a:"},
		},
		{
			name: "group prefix only",
			identity: Identity{
				Username: ClaimMapping{Claim: "sub"},
				Groups:   []GroupMapping{{Claim: "aud", Prefix: "peering:peer:a:cap:"}, {Claim: "groups", Prefix: "ci:"}},
			},
			expected: []string{"peering:peer:a:cap:"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ownedPrefixes(tt.identity); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ownedPrefixes = %v, want %v", got, tt.expected)
			}
		})
	}
}

// jwk is the public JWK of a key generated for the test.
func jwk(t *testing.T, kty string) map[string]any {
	t.Helper()
	b64 := base64.RawURLEncoding.EncodeToString
	switch kty {
	case "RSA":
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		return map[string]any{"kty": "RSA", "n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes())}
	case "EC":
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := key.PublicKey.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		return map[string]any{"kty": "EC", "crv": "P-256", "x": b64(raw[1:33]), "y": b64(raw[33:])}
	default:
		public, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return map[string]any{"kty": "OKP", "crv": "Ed25519", "x": b64(public)}
	}
}

func TestValidateKeys(t *testing.T) {
	with := func(key map[string]any, field string, value any) map[string]any {
		key = maps.Clone(key)
		key[field] = value
		return key
	}
	rsaKey, ecKey, okpKey := jwk(t, "RSA"), jwk(t, "EC"), jwk(t, "OKP")
	offCurve := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))

	tests := []struct {
		name    string
		key     map[string]any
		errText string
	}{
		{name: "RSA public key", key: rsaKey},
		{name: "EC public key", key: ecKey},
		{name: "Ed25519 public key", key: okpKey},
		{name: "RSA without the modulus", key: map[string]any{"kty": "RSA", "e": "AQAB"}, errText: `needs "n"`},
		{name: "modulus not in base64url", key: with(rsaKey, "n", "not base64!"), errText: "base64url"},
		{name: "RSA exponent of one", key: with(rsaKey, "e", "AQ"), errText: "not a valid RSA"},
		{name: "unknown curve", key: with(ecKey, "crv", "P-224"), errText: "is not P-256"},
		{name: "short coordinate", key: with(ecKey, "x", "AQAB"), errText: "32 bytes"},
		{name: "point off the curve", key: with(with(ecKey, "x", offCurve), "y", offCurve), errText: "not a point"},
		{name: "X25519 key", key: with(okpKey, "crv", "X25519"), errText: "not Ed25519"},
		{name: "short Ed25519 key", key: with(okpKey, "x", "AQAB"), errText: "32 bytes"},
		{name: "kid that is not a string", key: with(rsaKey, "kid", 7), errText: `spec.keys.jwks.keys[0]: "kid" is not a string`},
		{name: "use that is not a string", key: with(rsaKey, "use", true), errText: `spec.keys.jwks.keys[0]: "use" is not a string`},
		{name: "alg that is not a string", key: with(rsaKey, "alg", []any{"RS256"}), errText: `spec.keys.jwks.keys[0]: "alg" is not a string`},
		{name: "broken x5c", key: with(rsaKey, "x5c", []any{"not a certificate"}), errText: "Dex cannot read the key"},
		{name: "symmetric key", key: map[string]any{"kty": "oct", "k": "c2VjcmV0"}, errText: "symmetric"},
		{name: "unknown type", key: map[string]any{"kty": "XYZ"}, errText: "is not RSA"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(Spec{Keys: Keys{JWKS: &JWKS{Keys: []map[string]any{tt.key}}}})
			if tt.errText == "" {
				if err != nil {
					t.Fatalf("validate: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.errText) {
				t.Fatalf("validate = %v, want an error with %q", err, tt.errText)
			}
		})
	}
}

func TestValidateCAData(t *testing.T) {
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	ca := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))

	if err := validate(Spec{Keys: Keys{Discovery: &Discovery{CAData: ca}}}); err != nil {
		t.Errorf("validate a PEM bundle: %v", err)
	}
	err := validate(Spec{Keys: Keys{Discovery: &Discovery{CAData: "not a certificate"}}})
	if err == nil || !strings.Contains(err.Error(), "caData") {
		t.Errorf("validate = %v, want a caData error", err)
	}
}

func TestConnectorCarriesTheLifetimeLimit(t *testing.T) {
	spec := peeringSpec()
	spec["subjectTokenMaxLifetime"] = "1h"
	r, c := newTestReconciler(t, provider(spec))
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	connector, ok := getConnector(t, c, testID)
	if !ok {
		t.Fatal("no connector")
	}
	if got := connectorConfigOf(t, connector)["subjectTokenMaxLifetime"]; got != "1h" {
		t.Errorf("subjectTokenMaxLifetime = %v", got)
	}
}

func TestValidateLifetime(t *testing.T) {
	for value, ok := range map[string]bool{"": true, "24h": true, "1h30m": true, "0s": false, "1d": false} {
		err := validate(Spec{SubjectTokenMaxLifetime: value})
		if (err == nil) != ok {
			t.Errorf("validate(%q) = %v", value, err)
		}
	}
}

// A connector the API server refuses leaves the provider not ready, with the reason in its status.
func TestReconcileReportsAWriteFailure(t *testing.T) {
	r, c := newTestReconciler(t, provider(peeringSpec(), Finalizer))
	r.client = failingCreate{Client: c}

	if _, err := r.Reconcile(t.Context(), testRequest); err == nil {
		t.Fatal("Reconcile succeeded though the connector could not be created")
	}
	obj := controller.Object(controller.TokenExchangeProviderGVK)
	if err := c.Get(t.Context(), testRequest.NamespacedName, obj); err != nil {
		t.Fatal(err)
	}
	cond := meta.FindStatusCondition(readConditions(obj), conditionReady)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != ReasonApplyError || !strings.Contains(cond.Message, "refused") {
		t.Errorf("Ready = %+v", cond)
	}
}

// failingCreate refuses to create connectors.
type failingCreate struct {
	client.Client
}

func (f failingCreate) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if obj.GetObjectKind().GroupVersionKind() == controller.ConnectorGVK {
		return errors.New("refused by the API server")
	}
	return f.Client.Create(ctx, obj, opts...)
}

// A connector whose provider label was removed is invisible to the label-scoped cache. The
// controller reads it past the cache and writes it back with the label and the spec.
func TestReconcileRepairsAConnectorWithoutItsLabel(t *testing.T) {
	r, c := newTestReconciler(t, provider(peeringSpec(), Finalizer))
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatal(err)
	}

	tampered, _ := getConnector(t, c, testID)
	tampered.SetLabels(nil)
	tampered.Object["config"] = base64.StdEncoding.EncodeToString([]byte(`{"issuer":"https://evil.test"}`))
	if err := c.Update(t.Context(), tampered); err != nil {
		t.Fatal(err)
	}

	r.client = labelScopedCache{Client: c}
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	repaired, _ := getConnector(t, c, testID)
	if repaired.GetLabels()[ProviderLabel] != testName {
		t.Errorf("labels = %v", repaired.GetLabels())
	}
	if issuer := connectorConfigOf(t, repaired)["issuer"]; issuer != "https://kubernetes.default.svc.cluster.local" {
		t.Errorf("issuer = %v", issuer)
	}
}

// labelScopedCache hides the connectors without the provider label, as the informer cache does.
type labelScopedCache struct {
	client.Client
}

func (l labelScopedCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if err := l.Client.Get(ctx, key, obj, opts...); err != nil {
		return err
	}
	if obj.GetObjectKind().GroupVersionKind() == controller.ConnectorGVK && obj.GetLabels()[ProviderLabel] == "" {
		return apierrors.NewNotFound(schema.GroupResource{Group: "dex.coreos.com", Resource: "connectors"}, key.Name)
	}
	return nil
}
