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

package hooks

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
)

func TestExecuteDexProviderCheckFailsWhenProviderIsMissing(t *testing.T) {
	status := executeDexProviderCheck(
		context.Background(),
		nil,
		nil,
		DexProviderCheck{Spec: DexProviderCheckSpec{ProviderName: "missing"}},
		DexProviderForCheck{},
	)

	if status.Phase != DexProviderCheckPhaseFailed {
		t.Fatalf("expected failed phase, got %q", status.Phase)
	}
	if len(status.Checks) != 1 || status.Checks[0].Name != "providerExists" || status.Checks[0].Status != dexProviderCheckStepFailed {
		t.Fatalf("unexpected checks: %#v", status.Checks)
	}
}

func TestExecuteDexProviderCheckFailsWhenProviderIsDisabled(t *testing.T) {
	status := executeDexProviderCheck(
		context.Background(),
		nil,
		nil,
		DexProviderCheck{Spec: DexProviderCheckSpec{ProviderName: "github"}},
		DexProviderForCheck{
			ObjectMeta: metav1.ObjectMeta{Name: "github", Generation: 42},
			Spec: DexProviderForCheckSpec{
				Enabled: ptr.To(false),
				Type:    "Github",
			},
		},
	)

	if status.Phase != DexProviderCheckPhaseFailed {
		t.Fatalf("expected failed phase, got %q", status.Phase)
	}
	if status.ObservedDexProviderGeneration != 42 {
		t.Fatalf("expected observed generation 42, got %d", status.ObservedDexProviderGeneration)
	}
	if len(status.Checks) != 2 || status.Checks[1].Name != "providerEnabled" || status.Checks[1].Status != dexProviderCheckStepFailed {
		t.Fatalf("unexpected checks: %#v", status.Checks)
	}
}

func TestLDAPAddressDefaultsPortFromTLSMode(t *testing.T) {
	tests := []struct {
		name string
		cfg  DexProviderLDAPForCheck
		want string
	}{
		{
			name: "ldaps default",
			cfg:  DexProviderLDAPForCheck{Host: "ldap.example.com"},
			want: "ldap.example.com:636",
		},
		{
			name: "plain ldap default",
			cfg:  DexProviderLDAPForCheck{Host: "ldap.example.com", InsecureNoSSL: true},
			want: "ldap.example.com:389",
		},
		{
			name: "starttls default",
			cfg:  DexProviderLDAPForCheck{Host: "ldap.example.com", StartTLS: true},
			want: "ldap.example.com:389",
		},
		{
			name: "explicit port",
			cfg:  DexProviderLDAPForCheck{Host: "ldap.example.com:1636"},
			want: "ldap.example.com:1636",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := ldapAddress(&tt.cfg)
			if err != nil {
				t.Fatalf("ldapAddress returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestEarliestCertExpiry(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer ts.Close()

	cert := ts.Certificate()
	pemBytes := pem.EncodeToMemory(&pem.Block{Bytes: cert.Raw, Type: "CERTIFICATE"})

	got, err := earliestCertExpiry(pemBytes)
	if err != nil {
		t.Fatalf("earliestCertExpiry returned error: %v", err)
	}
	if !got.Equal(cert.NotAfter) {
		t.Fatalf("expected %s, got %s", cert.NotAfter, got)
	}

	if _, err := earliestCertExpiry([]byte("not a pem")); err == nil {
		t.Fatal("expected error for input without certificates")
	}
}

func TestReportExpiry(t *testing.T) {
	tests := []struct {
		name     string
		notAfter time.Time
		want     string
	}{
		{name: "expired", notAfter: time.Now().Add(-time.Hour), want: dexProviderCheckStepFailed},
		{name: "expires soon", notAfter: time.Now().Add(24 * time.Hour), want: dexProviderCheckStepSucceeded},
		{name: "valid", notAfter: time.Now().Add(365 * 24 * time.Hour), want: dexProviderCheckStepSucceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &dexProviderCheckResult{}
			reportExpiry(result, "cert", "test certificate", tt.notAfter)
			if len(result.checks) != 1 || result.checks[0].Status != tt.want {
				t.Fatalf("expected status %q, got %#v", tt.want, result.checks)
			}
		})
	}
}

func TestCheckTLSCertificate(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer ts.Close()

	t.Run("reachable https reports certificate validity", func(t *testing.T) {
		result := &dexProviderCheckResult{}
		checkTLSCertificate(result, "tls", ts.URL, "", true)
		if len(result.checks) != 1 || result.checks[0].Status != dexProviderCheckStepSucceeded {
			t.Fatalf("expected success, got %#v", result.checks)
		}
	})

	t.Run("non-https endpoint is skipped", func(t *testing.T) {
		result := &dexProviderCheckResult{}
		checkTLSCertificate(result, "tls", "http://example.com", "", false)
		if len(result.checks) != 1 || result.checks[0].Status != dexProviderCheckStepSkipped {
			t.Fatalf("expected skipped, got %#v", result.checks)
		}
	})
}

func TestOIDCDiscoveryMissingEndpoints(t *testing.T) {
	full := oidcDiscoveryDocument{AuthorizationEndpoint: "https://idp/auth", TokenEndpoint: "https://idp/token"}
	if missing := full.missingEndpoints(); len(missing) != 0 {
		t.Fatalf("expected no missing endpoints, got %v", missing)
	}

	empty := oidcDiscoveryDocument{}
	if missing := empty.missingEndpoints(); len(missing) != 2 {
		t.Fatalf("expected 2 missing endpoints, got %v", missing)
	}
}

func TestProbeOAuthClientCredentials(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		switch r.PostFormValue("client_id") {
		case "valid":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"token","token_type":"bearer"}`))
		case "grant-disabled":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unauthorized_client"}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
		}
	}))
	defer srv.Close()

	client := d8http.NewClient(d8http.WithInsecureSkipVerify(), d8http.WithTimeout(5*time.Second))

	tests := []struct {
		name         string
		clientID     string
		wantAccepted bool
	}{
		{name: "valid credentials issue a token", clientID: "valid", wantAccepted: true},
		{name: "valid client but grant disabled is accepted", clientID: "grant-disabled", wantAccepted: true},
		{name: "wrong secret is rejected", clientID: "wrong", wantAccepted: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accepted, detail, err := probeOAuthClientCredentials(context.Background(), client, srv.URL, tt.clientID, "secret")
			if err != nil {
				t.Fatalf("probeOAuthClientCredentials returned error: %v", err)
			}
			if accepted != tt.wantAccepted {
				t.Fatalf("expected accepted=%v, got %v (detail: %s)", tt.wantAccepted, accepted, detail)
			}
		})
	}
}

func TestCheckCABundle(t *testing.T) {
	t.Run("empty is skipped", func(t *testing.T) {
		result := &dexProviderCheckResult{}
		checkCABundle(result, "ca", "")
		if len(result.checks) != 1 || result.checks[0].Status != dexProviderCheckStepSkipped {
			t.Fatalf("expected skipped, got %#v", result.checks)
		}
	})

	t.Run("invalid bundle fails", func(t *testing.T) {
		result := &dexProviderCheckResult{}
		checkCABundle(result, "ca", "-----BEGIN CERTIFICATE-----\nbroken\n-----END CERTIFICATE-----")
		if len(result.checks) != 1 || result.checks[0].Status != dexProviderCheckStepFailed {
			t.Fatalf("expected failure, got %#v", result.checks)
		}
	})
}
