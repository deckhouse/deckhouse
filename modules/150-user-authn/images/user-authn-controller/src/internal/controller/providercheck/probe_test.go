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

package providercheck

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExecuteFailsWhenProviderIsMissing(t *testing.T) {
	t.Parallel()

	r := &Reconciler{http: newFakeHTTP(), ldap: &fakeLDAP{}, now: time.Now}
	status := r.execute(
		t.Context(),
		&DexProviderCheck{Spec: DexProviderCheckSpec{ProviderName: "missing"}},
		DexProviderForCheck{},
	)

	if status.Phase != DexProviderCheckPhaseFailed {
		t.Fatalf("expected failed phase, got %q", status.Phase)
	}
	if len(status.Checks) != 1 || status.Checks[0].Name != "providerExists" || status.Checks[0].Status != stepFailed {
		t.Fatalf("unexpected checks: %#v", status.Checks)
	}
}

func TestExecuteFailsWhenProviderIsDisabled(t *testing.T) {
	t.Parallel()

	enabled := false
	r := &Reconciler{http: newFakeHTTP(), ldap: &fakeLDAP{}, now: time.Now}
	status := r.execute(
		t.Context(),
		&DexProviderCheck{Spec: DexProviderCheckSpec{ProviderName: "github"}},
		DexProviderForCheck{
			ObjectMeta: metav1.ObjectMeta{Name: "github", Generation: 42},
			Spec: DexProviderForCheckSpec{
				Enabled: &enabled,
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
	if len(status.Checks) != 2 || status.Checks[1].Name != "providerEnabled" || status.Checks[1].Status != stepFailed {
		t.Fatalf("unexpected checks: %#v", status.Checks)
	}
}

func TestLDAPAddressDefaultsPortFromTLSMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  DexProviderLDAPForCheck
		want string
	}{
		{name: "ldaps default", cfg: DexProviderLDAPForCheck{Host: "ldap.example.com"}, want: "ldap.example.com:636"},
		{name: "plain ldap default", cfg: DexProviderLDAPForCheck{Host: "ldap.example.com", InsecureNoSSL: true}, want: "ldap.example.com:389"},
		{name: "starttls default", cfg: DexProviderLDAPForCheck{Host: "ldap.example.com", StartTLS: true}, want: "ldap.example.com:389"},
		{name: "explicit port", cfg: DexProviderLDAPForCheck{Host: "ldap.example.com:1636"}, want: "ldap.example.com:1636"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()

	ts := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(ts.Close)

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
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		notAfter time.Time
		want     string
	}{
		{name: "expired", notAfter: now.Add(-time.Hour), want: stepFailed},
		{name: "expires soon", notAfter: now.Add(24 * time.Hour), want: stepWarning},
		{name: "valid", notAfter: now.Add(365 * 24 * time.Hour), want: stepSucceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := &dexProviderCheckResult{now: now}
			reportExpiry(result, "cert", "test certificate", tt.notAfter)
			if len(result.checks) != 1 || result.checks[0].Status != tt.want {
				t.Fatalf("expected status %q, got %#v", tt.want, result.checks)
			}
		})
	}
}

func TestCheckTLSCertificate(t *testing.T) {
	t.Parallel()

	ts := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(ts.Close)

	t.Run("insecureSkipVerify reports a warning", func(t *testing.T) {
		t.Parallel()
		result := &dexProviderCheckResult{now: time.Now()}
		checkTLSCertificate(t.Context(), result, "tls", ts.URL, "", true)
		if len(result.checks) != 1 || result.checks[0].Status != stepWarning {
			t.Fatalf("expected warning, got %#v", result.checks)
		}
	})

	t.Run("non-https endpoint is skipped", func(t *testing.T) {
		t.Parallel()
		result := &dexProviderCheckResult{now: time.Now()}
		checkTLSCertificate(t.Context(), result, "tls", "http://example.com", "", false)
		if len(result.checks) != 1 || result.checks[0].Status != stepSkipped {
			t.Fatalf("expected skipped, got %#v", result.checks)
		}
	})
}

func TestOIDCDiscoveryMissingEndpoints(t *testing.T) {
	t.Parallel()

	full := oidcDiscoveryDocument{AuthorizationEndpoint: "https://idp/auth", TokenEndpoint: "https://idp/token"}
	if missing := full.missingEndpoints(); len(missing) != 0 {
		t.Fatalf("expected no missing endpoints, got %v", missing)
	}

	empty := oidcDiscoveryDocument{}
	if missing := empty.missingEndpoints(); len(missing) != 2 {
		t.Fatalf("expected 2 missing endpoints, got %v", missing)
	}
}

func TestProbeClientSecret(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		clientID, _, _ := r.BasicAuth()
		if clientID == "" {
			clientID = r.PostFormValue("client_id")
		}
		switch clientID {
		case "good-bad-code":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Code not valid"}`))
		case "good-bad-request":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_request"}`))
		case "good-token":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"token","token_type":"bearer"}`))
		case "bad-unauthorized-client":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unauthorized_client","error_description":"Invalid OAuth client credentials"}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"Invalid client credentials"}`))
		}
	}))
	t.Cleanup(srv.Close)

	client, err := NewDefaultHTTP().New(TLSOptions{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("http client: %v", err)
	}

	tests := []struct {
		name         string
		clientID     string
		wantAccepted bool
	}{
		{name: "valid secret, bogus code rejected", clientID: "good-bad-code", wantAccepted: true},
		{name: "valid secret, other grant error", clientID: "good-bad-request", wantAccepted: true},
		{name: "valid secret, token issued", clientID: "good-token", wantAccepted: true},
		{name: "wrong secret, invalid_client", clientID: "bad-invalid-client", wantAccepted: false},
		{name: "wrong secret, unauthorized_client (Bitbucket)", clientID: "bad-unauthorized-client", wantAccepted: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			accepted, detail, err := probeClientSecret(t.Context(), client, srv.URL, tt.clientID, "secret")
			if err != nil {
				t.Fatalf("probeClientSecret returned error: %v", err)
			}
			if accepted != tt.wantAccepted {
				t.Fatalf("expected accepted=%v, got %v (detail: %s)", tt.wantAccepted, accepted, detail)
			}
		})
	}
}

func TestCheckUpToDate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	freshFor := func(generation int64, completedAt time.Time) *DexProviderCheck {
		ct := metav1.NewTime(completedAt)
		return &DexProviderCheck{Status: DexProviderCheckStatus{
			ObservedDexProviderGeneration: generation,
			CompletedAt:                   &ct,
		}}
	}

	t.Run("never completed is not up to date", func(t *testing.T) {
		t.Parallel()
		if checkUpToDate(&DexProviderCheck{}, 3, now) {
			t.Fatal("expected a check without completedAt not to be up to date")
		}
	})

	t.Run("fresh result matching generation is up to date", func(t *testing.T) {
		t.Parallel()
		if !checkUpToDate(freshFor(3, now), 3, now) {
			t.Fatal("expected a fresh check for the current generation to be up to date")
		}
	})

	t.Run("stale result is not up to date", func(t *testing.T) {
		t.Parallel()
		if checkUpToDate(freshFor(3, now.Add(-2*recheckInterval)), 3, now) {
			t.Fatal("expected a stale check not to be up to date")
		}
	})

	t.Run("generation mismatch is not up to date", func(t *testing.T) {
		t.Parallel()
		if checkUpToDate(freshFor(2, now), 3, now) {
			t.Fatal("expected a check from a previous generation not to be up to date")
		}
	})
}

func TestCanonicalCheckName(t *testing.T) {
	t.Parallel()
	if got := canonicalCheckName("my-oidc"); got != "my-oidc" {
		t.Fatalf("expected canonical name to equal provider name, got %q", got)
	}
}

func TestParseAcknowledgedWarnings(t *testing.T) {
	t.Parallel()

	t.Run("absent annotation", func(t *testing.T) {
		t.Parallel()
		all, set := parseAcknowledgedWarnings(nil)
		if all || len(set) != 0 {
			t.Fatalf("expected no acknowledgements, got all=%v set=%#v", all, set)
		}
	})

	t.Run("list of steps", func(t *testing.T) {
		t.Parallel()
		all, set := parseAcknowledgedWarnings(map[string]string{
			acknowledgedWarningsAnnotation: "ldapCertificate, oidcCertificate",
		})
		if all {
			t.Fatal("did not expect acknowledge-all")
		}
		if !set["ldapCertificate"] || !set["oidcCertificate"] {
			t.Fatalf("expected both steps acknowledged, got %#v", set)
		}
	})

	t.Run("wildcard", func(t *testing.T) {
		t.Parallel()
		all, _ := parseAcknowledgedWarnings(map[string]string{
			acknowledgedWarningsAnnotation: "*",
		})
		if !all {
			t.Fatal("expected acknowledge-all")
		}
	})
}

func TestWarnAcknowledgement(t *testing.T) {
	t.Parallel()

	t.Run("unacknowledged warning stays Warning", func(t *testing.T) {
		t.Parallel()
		result := &dexProviderCheckResult{}
		result.warn("ldapCertificate", "verification disabled")
		if len(result.checks) != 1 || result.checks[0].Status != stepWarning {
			t.Fatalf("expected warning, got %#v", result.checks)
		}
	})

	t.Run("acknowledged step is downgraded to success", func(t *testing.T) {
		t.Parallel()
		result := &dexProviderCheckResult{acknowledgedWarnings: map[string]bool{"ldapCertificate": true}}
		result.warn("ldapCertificate", "verification disabled")
		if len(result.checks) != 1 || result.checks[0].Status != stepSucceeded {
			t.Fatalf("expected success, got %#v", result.checks)
		}
	})

	t.Run("acknowledge-all downgrades any warning", func(t *testing.T) {
		t.Parallel()
		result := &dexProviderCheckResult{acknowledgeAllWarnings: true}
		result.warn("oidcCertificate", "expires soon")
		if len(result.checks) != 1 || result.checks[0].Status != stepSucceeded {
			t.Fatalf("expected success, got %#v", result.checks)
		}
	})
}

func TestProbeIntrospection(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		clientID, _, _ := r.BasicAuth()
		if clientID == "" {
			clientID = r.PostFormValue("client_id")
		}
		w.Header().Set("Content-Type", "application/json")
		switch clientID {
		case "good":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"active":false}`))
		case "bad":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
		default:
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"access_denied"}`))
		}
	}))
	t.Cleanup(srv.Close)

	client, err := NewDefaultHTTP().New(TLSOptions{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("http client: %v", err)
	}

	tests := []struct {
		name           string
		clientID       string
		wantConclusive bool
		wantStatus     string
	}{
		{name: "valid secret", clientID: "good", wantConclusive: true, wantStatus: stepSucceeded},
		{name: "wrong secret", clientID: "bad", wantConclusive: true, wantStatus: stepFailed},
		{name: "ambiguous response falls back", clientID: "no-introspect", wantConclusive: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := &dexProviderCheckResult{}
			conclusive := probeIntrospection(t.Context(), client, result, srv.URL, tt.clientID, "secret")
			if conclusive != tt.wantConclusive {
				t.Fatalf("expected conclusive=%v, got %v (checks: %#v)", tt.wantConclusive, conclusive, result.checks)
			}
			if !tt.wantConclusive {
				if len(result.checks) != 0 {
					t.Fatalf("expected no recorded step, got %#v", result.checks)
				}
				return
			}
			if len(result.checks) != 1 || result.checks[0].Status != tt.wantStatus {
				t.Fatalf("expected status %q, got %#v", tt.wantStatus, result.checks)
			}
		})
	}
}

func TestClusterInternalHostReason(t *testing.T) {
	t.Parallel()

	internal := []string{
		"keycloak.keycloak1.svc",
		"keycloak.keycloak1.svc.cluster.local",
		"keycloak",
		"localhost",
		"idp.local",
		"127.0.0.1",
		"10.0.0.1",
		"192.168.1.10",
		"172.16.5.4",
		"169.254.1.1",
	}
	for _, host := range internal {
		if reason := clusterInternalHostReason(host); reason == "" {
			t.Errorf("expected %q to be detected as cluster-internal", host)
		}
	}

	public := []string{
		"keycloak.example.com",
		"accounts.google.com",
		"idp.185.11.73.222.sslip.io",
		"8.8.8.8",
	}
	for _, host := range public {
		if reason := clusterInternalHostReason(host); reason != "" {
			t.Errorf("expected %q to be public, got reason %q", host, reason)
		}
	}
}

func TestCheckPublicBrowserURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{name: "internal svc fails", rawURL: "https://keycloak.keycloak1.svc:8443/realms/d8test", want: stepFailed},
		{name: "single label fails", rawURL: "https://keycloak:8443/realms/d8test", want: stepFailed},
		{name: "public domain succeeds", rawURL: "https://keycloak.example.com/realms/d8test", want: stepSucceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := &dexProviderCheckResult{}
			checkPublicBrowserURL(result, "public", tt.rawURL)
			if len(result.checks) != 1 || result.checks[0].Status != tt.want {
				t.Fatalf("expected status %q, got %#v", tt.want, result.checks)
			}
		})
	}
}

func TestCheckCABundle(t *testing.T) {
	t.Parallel()

	t.Run("empty is skipped", func(t *testing.T) {
		t.Parallel()
		result := &dexProviderCheckResult{}
		checkCABundle(result, "ca", "")
		if len(result.checks) != 1 || result.checks[0].Status != stepSkipped {
			t.Fatalf("expected skipped, got %#v", result.checks)
		}
	})

	t.Run("invalid bundle fails", func(t *testing.T) {
		t.Parallel()
		result := &dexProviderCheckResult{}
		checkCABundle(result, "ca", "-----BEGIN CERTIFICATE-----\nbroken\n-----END CERTIFICATE-----")
		if len(result.checks) != 1 || result.checks[0].Status != stepFailed {
			t.Fatalf("expected failure, got %#v", result.checks)
		}
	})
}

func TestExecuteHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	r := &Reconciler{http: newFakeHTTP(), ldap: &fakeLDAP{}, now: time.Now}
	status := r.execute(ctx, &DexProviderCheck{Spec: DexProviderCheckSpec{ProviderName: "github"}}, DexProviderForCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "github", Generation: 1},
		Spec:       DexProviderForCheckSpec{Type: "Github", Github: &DexProviderGithubForCheck{ClientID: "id"}},
	})
	if status.Phase != DexProviderCheckPhaseFailed {
		t.Fatalf("phase = %q, want Failed", status.Phase)
	}
	assertNamedStatus(t, status, "dexReady", stepFailed)
}

func assertNamedStatus(t *testing.T, status DexProviderCheckStatus, name, want string) {
	t.Helper()
	for _, step := range status.Checks {
		if step.Name == name {
			if step.Status != want {
				t.Fatalf("step %q status = %q, want %q", name, step.Status, want)
			}
			return
		}
	}
	t.Fatalf("missing step %q in %#v", name, status.Checks)
}
