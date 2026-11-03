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
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"user-authn-controller/internal/controller"
)

// issuerServer serves a discovery document and a key set; issuer overrides the issuer it names.
func issuerServer(t *testing.T, status int, issuer string) (*httptest.Server, string) {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			http.Error(w, "down", status)
			return
		}
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			name := issuer
			if name == "" {
				name = srv.URL
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"issuer": name, "jwks_uri": srv.URL + "/keys"})
		case "/keys":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "n": "AQAB", "e": "AQAB"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))
}

func discoverySpec(issuer, ca string) map[string]any {
	spec := peeringSpec()
	spec["issuer"] = issuer
	spec["keys"] = map[string]any{"discovery": map[string]any{"caData": ca}}
	return spec
}

func TestDiscoveryProbe(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		issuer    string
		ready     metav1.ConditionStatus
		connector bool
	}{
		{name: "issuer serves its keys", status: http.StatusOK, ready: metav1.ConditionTrue, connector: true},
		// The connector is written all the same: Dex reads discovery at an exchange.
		{name: "issuer is down", status: http.StatusServiceUnavailable, ready: metav1.ConditionFalse, connector: true},
		{name: "discovery names another issuer", status: http.StatusOK, issuer: "https://other.example.com", ready: metav1.ConditionFalse, connector: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, ca := issuerServer(t, tt.status, tt.issuer)
			r, c := newTestReconciler(t, provider(discoverySpec(srv.URL, ca), Finalizer))

			result, err := r.Reconcile(t.Context(), testRequest)
			if err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			if _, ok := getConnector(t, c, testID); ok != tt.connector {
				t.Errorf("connector written = %v, want %v", ok, tt.connector)
			}
			obj := controller.Object(controller.TokenExchangeProviderGVK)
			if err := c.Get(t.Context(), testRequest.NamespacedName, obj); err != nil {
				t.Fatal(err)
			}
			cond := meta.FindStatusCondition(readConditions(obj), conditionReady)
			if cond == nil || cond.Status != tt.ready {
				t.Fatalf("Ready = %+v", cond)
			}
			if tt.ready == metav1.ConditionFalse && (cond.Reason != ReasonDiscoveryFailed || result.RequeueAfter == 0) {
				t.Errorf("reason %q, requeue after %v", cond.Reason, result.RequeueAfter)
			}
		})
	}
}

// A change that narrows the provider reaches Dex while the issuer is down.
func TestNarrowingWhileTheIssuerIsDown(t *testing.T) {
	srv, ca := issuerServer(t, http.StatusServiceUnavailable, "")
	spec := discoverySpec(srv.URL, ca)
	spec["clients"] = []any{
		map[string]any{"dexClient": map[string]any{"name": "a", "namespace": "ci"}},
		map[string]any{"dexClient": map[string]any{"name": "b", "namespace": "ci"}},
	}
	r, c := newTestReconciler(t, provider(spec, Finalizer))
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	obj := controller.Object(controller.TokenExchangeProviderGVK)
	if err := c.Get(t.Context(), testRequest.NamespacedName, obj); err != nil {
		t.Fatal(err)
	}
	spec["clients"] = []any{map[string]any{"dexClient": map[string]any{"name": "a", "namespace": "ci"}}}
	obj.Object["spec"] = spec
	if err := c.Update(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	connector, ok := getConnector(t, c, testID)
	if !ok {
		t.Fatal("no connector")
	}
	if got := connectorConfigOf(t, connector)["clients"]; !reflect.DeepEqual(got, []any{"dex-client-a@ci"}) {
		t.Errorf("clients = %v, want only a", got)
	}
}

// A discovery document that names a plain http key set fails the probe: Dex would refuse it.
func TestDiscoveryProbeRefusesAnHTTPKeySet(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": srv.URL, "jwks_uri": "http://keys.example.com/keys"})
	}))
	t.Cleanup(srv.Close)
	ca := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))
	spec, err := Decode(provider(discoverySpec(srv.URL, ca)))
	if err != nil {
		t.Fatal(err)
	}
	if err := probeDiscovery(t.Context(), spec); err == nil || !strings.Contains(err.Error(), "not https") {
		t.Fatalf("probeDiscovery = %v, want a refused http key set", err)
	}
}

// A discovery document that sends the probe to plain http is not followed.
func TestDiscoveryProbeRefusesARedirectToHTTP(t *testing.T) {
	plain := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(plain.Close)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL+r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	ca := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))

	spec, err := Decode(provider(discoverySpec(srv.URL, ca)))
	if err != nil {
		t.Fatal(err)
	}
	if err := probeDiscovery(t.Context(), spec); err == nil || !strings.Contains(err.Error(), "only https is followed") {
		t.Fatalf("probeDiscovery = %v, want a refused redirect", err)
	}
}
