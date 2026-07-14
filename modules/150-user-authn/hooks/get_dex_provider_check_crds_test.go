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
		{name: "expires soon", notAfter: time.Now().Add(24 * time.Hour), want: dexProviderCheckStepWarning},
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

	t.Run("insecureSkipVerify reports a warning", func(t *testing.T) {
		result := &dexProviderCheckResult{}
		checkTLSCertificate(result, "tls", ts.URL, "", true)
		if len(result.checks) != 1 || result.checks[0].Status != dexProviderCheckStepWarning {
			t.Fatalf("expected warning, got %#v", result.checks)
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

// TestProbeClientSecret exercises the single, unified client-secret probe used
// by the OIDC, GitLab and Bitbucket credential checks. The test server selects a
// canned token-endpoint response based on the client_id, mirroring the real
// behaviour observed across providers (including Bitbucket, which answers
// unauthorized_client for invalid consumer credentials).
func TestProbeClientSecret(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		// The client id is sent via HTTP Basic auth, with a form-body fallback.
		clientID, _, _ := r.BasicAuth()
		if clientID == "" {
			clientID = r.PostFormValue("client_id")
		}
		switch clientID {
		case "good-bad-code":
			// Valid secret; only the bogus authorization code is rejected.
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Code not valid"}`))
		case "good-bad-request":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_request"}`))
		case "good-token":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"token","token_type":"bearer"}`))
		case "bad-unauthorized-client":
			// Bitbucket-style: invalid consumer credentials.
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unauthorized_client","error_description":"Invalid OAuth client credentials"}`))
		default:
			// invalid_client / HTTP 401: standard client authentication failure.
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"Invalid client credentials"}`))
		}
	}))
	defer srv.Close()

	client := d8http.NewClient(d8http.WithInsecureSkipVerify(), d8http.WithTimeout(5*time.Second))

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
			accepted, detail, err := probeClientSecret(context.Background(), client, srv.URL, tt.clientID, "secret")
			if err != nil {
				t.Fatalf("probeClientSecret returned error: %v", err)
			}
			if accepted != tt.wantAccepted {
				t.Fatalf("expected accepted=%v, got %v (detail: %s)", tt.wantAccepted, accepted, detail)
			}
		})
	}
}

func TestDexProviderCheckUpToDate(t *testing.T) {
	provider := DexProviderForCheck{ObjectMeta: metav1.ObjectMeta{Generation: 3}}
	freshFor := func(generation int64, completedAt metav1.Time) DexProviderCheck {
		return DexProviderCheck{Status: DexProviderCheckStatus{
			ObservedDexProviderGeneration: generation,
			CompletedAt:                   ptr.To(completedAt),
		}}
	}

	t.Run("never completed is not up to date", func(t *testing.T) {
		if dexProviderCheckUpToDate(DexProviderCheck{}, provider) {
			t.Fatal("expected a check without completedAt not to be up to date")
		}
	})

	t.Run("fresh result matching generation is up to date", func(t *testing.T) {
		if !dexProviderCheckUpToDate(freshFor(provider.Generation, metav1.Now()), provider) {
			t.Fatal("expected a fresh check for the current generation to be up to date")
		}
	})

	t.Run("stale result is not up to date", func(t *testing.T) {
		stale := metav1.NewTime(time.Now().Add(-2 * dexProviderCheckRecheckInterval))
		if dexProviderCheckUpToDate(freshFor(provider.Generation, stale), provider) {
			t.Fatal("expected a stale check not to be up to date")
		}
	})

	t.Run("generation mismatch is not up to date", func(t *testing.T) {
		if dexProviderCheckUpToDate(freshFor(provider.Generation-1, metav1.Now()), provider) {
			t.Fatal("expected a check from a previous generation not to be up to date")
		}
	})
}

func TestDecideDexProviderCheckAction(t *testing.T) {
	provider := DexProviderForCheck{ObjectMeta: metav1.ObjectMeta{Name: "p", Generation: 5}}

	completed := func(phase DexProviderCheckPhase, generation int64, completedAt metav1.Time) DexProviderCheck {
		return DexProviderCheck{Status: DexProviderCheckStatus{
			Phase:                         phase,
			ObservedDexProviderGeneration: generation,
			CompletedAt:                   ptr.To(completedAt),
		}}
	}
	pending := func(generation int64) DexProviderCheck {
		return DexProviderCheck{Status: DexProviderCheckStatus{
			Phase:                         DexProviderCheckPhasePending,
			ObservedDexProviderGeneration: generation,
		}}
	}

	tests := []struct {
		name  string
		check DexProviderCheck
		want  dexProviderCheckAction
	}{
		{name: "brand-new check is acknowledged as Pending", check: DexProviderCheck{}, want: dexProviderCheckActionMarkPending},
		{name: "Pending for the current generation is executed", check: pending(provider.Generation), want: dexProviderCheckActionExecute},
		{name: "Pending from an older generation is re-acknowledged", check: pending(provider.Generation - 1), want: dexProviderCheckActionMarkPending},
		{name: "fresh Succeeded for the current generation is kept", check: completed(DexProviderCheckPhaseSucceeded, provider.Generation, metav1.Now()), want: dexProviderCheckActionKeep},
		{name: "fresh Failed for the current generation is kept", check: completed(DexProviderCheckPhaseFailed, provider.Generation, metav1.Now()), want: dexProviderCheckActionKeep},
		{name: "result from a previous generation is re-run via Pending", check: completed(DexProviderCheckPhaseSucceeded, provider.Generation-1, metav1.Now()), want: dexProviderCheckActionMarkPending},
		{name: "expired result is re-run via Pending", check: completed(DexProviderCheckPhaseSucceeded, provider.Generation, metav1.NewTime(time.Now().Add(-2*dexProviderCheckRecheckInterval))), want: dexProviderCheckActionMarkPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decideDexProviderCheckAction(tt.check, provider); got != tt.want {
				t.Fatalf("decideDexProviderCheckAction = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestDexProviderCheckLifecycleTerminates walks a check through the two-phase
// pickup to prove it converges and never re-marks an identical Pending (which
// would make the status write self-trigger the hook forever).
func TestDexProviderCheckLifecycleTerminates(t *testing.T) {
	provider := DexProviderForCheck{ObjectMeta: metav1.ObjectMeta{Name: "p", Generation: 7}}
	check := DexProviderCheck{}

	// 1. Brand-new → acknowledge as Pending.
	if got := decideDexProviderCheckAction(check, provider); got != dexProviderCheckActionMarkPending {
		t.Fatalf("new check: got %d, want MarkPending", got)
	}

	// 2. After the Pending write the next pass must Execute (not MarkPending
	//    again), so the status write triggers the hook exactly once more.
	pendingStatus := pendingDexProviderCheckStatus(provider.Generation)
	check.Status.Phase = DexProviderCheckPhase(pendingStatus["phase"].(string))
	check.Status.ObservedDexProviderGeneration = pendingStatus["observedDexProviderGeneration"].(int64)
	check.Status.Checks = nil
	check.Status.CompletedAt = nil
	if got := decideDexProviderCheckAction(check, provider); got != dexProviderCheckActionExecute {
		t.Fatalf("after Pending write: got %d, want Execute", got)
	}

	// 3. After the terminal write the next pass must Keep, so the result is not
	//    re-run and the loop terminates.
	check.Status = DexProviderCheckStatus{
		Phase:                         DexProviderCheckPhaseSucceeded,
		ObservedDexProviderGeneration: provider.Generation,
		CompletedAt:                   ptr.To(metav1.Now()),
	}
	if got := decideDexProviderCheckAction(check, provider); got != dexProviderCheckActionKeep {
		t.Fatalf("after terminal write: got %d, want Keep", got)
	}
}

func TestPendingDexProviderCheckStatus(t *testing.T) {
	status := pendingDexProviderCheckStatus(9)

	if status["phase"] != string(DexProviderCheckPhasePending) {
		t.Fatalf("phase = %v, want Pending", status["phase"])
	}
	if status["observedDexProviderGeneration"] != int64(9) {
		t.Fatalf("observedDexProviderGeneration = %v, want 9", status["observedDexProviderGeneration"])
	}
	// Explicit nils clear a previous run's results through the JSON merge patch,
	// so a Pending status never shows stale step results or completedAt.
	if v, ok := status["checks"]; !ok || v != nil {
		t.Fatalf("checks = %v (present=%v), want explicit nil", v, ok)
	}
	if v, ok := status["completedAt"]; !ok || v != nil {
		t.Fatalf("completedAt = %v (present=%v), want explicit nil", v, ok)
	}
}

func TestCanonicalDexProviderCheckName(t *testing.T) {
	if got := canonicalDexProviderCheckName("my-oidc"); got != "my-oidc" {
		t.Fatalf("expected canonical name to equal provider name, got %q", got)
	}
}

func TestParseAcknowledgedWarnings(t *testing.T) {
	t.Run("absent annotation", func(t *testing.T) {
		all, set := parseAcknowledgedWarnings(nil)
		if all || len(set) != 0 {
			t.Fatalf("expected no acknowledgements, got all=%v set=%#v", all, set)
		}
	})

	t.Run("list of steps", func(t *testing.T) {
		all, set := parseAcknowledgedWarnings(map[string]string{
			dexProviderAcknowledgedWarningsAnnotation: "ldapCertificate, oidcCertificate",
		})
		if all {
			t.Fatal("did not expect acknowledge-all")
		}
		if !set["ldapCertificate"] || !set["oidcCertificate"] {
			t.Fatalf("expected both steps acknowledged, got %#v", set)
		}
	})

	t.Run("wildcard", func(t *testing.T) {
		all, _ := parseAcknowledgedWarnings(map[string]string{
			dexProviderAcknowledgedWarningsAnnotation: "*",
		})
		if !all {
			t.Fatal("expected acknowledge-all")
		}
	})
}

func TestWarnAcknowledgement(t *testing.T) {
	t.Run("unacknowledged warning stays Warning", func(t *testing.T) {
		result := &dexProviderCheckResult{}
		result.warn("ldapCertificate", "verification disabled")
		if len(result.checks) != 1 || result.checks[0].Status != dexProviderCheckStepWarning {
			t.Fatalf("expected warning, got %#v", result.checks)
		}
	})

	t.Run("acknowledged step is downgraded to success", func(t *testing.T) {
		result := &dexProviderCheckResult{acknowledgedWarnings: map[string]bool{"ldapCertificate": true}}
		result.warn("ldapCertificate", "verification disabled")
		if len(result.checks) != 1 || result.checks[0].Status != dexProviderCheckStepSucceeded {
			t.Fatalf("expected success, got %#v", result.checks)
		}
	})

	t.Run("acknowledge-all downgrades any warning", func(t *testing.T) {
		result := &dexProviderCheckResult{acknowledgeAllWarnings: true}
		result.warn("oidcCertificate", "expires soon")
		if len(result.checks) != 1 || result.checks[0].Status != dexProviderCheckStepSucceeded {
			t.Fatalf("expected success, got %#v", result.checks)
		}
	})
}

// TestProbeIntrospection exercises the RFC 7662 token-introspection fast path
// used for OIDC client-secret validation. A correctly authenticated client
// receives HTTP 200 for a bogus token; invalid credentials get HTTP 401; and an
// ambiguous response (e.g. the client cannot introspect) leaves the decision to
// the caller.
func TestProbeIntrospection(t *testing.T) {
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
	defer srv.Close()

	client := d8http.NewClient(d8http.WithInsecureSkipVerify(), d8http.WithTimeout(5*time.Second))

	tests := []struct {
		name           string
		clientID       string
		wantConclusive bool
		wantStatus     string
	}{
		{name: "valid secret", clientID: "good", wantConclusive: true, wantStatus: dexProviderCheckStepSucceeded},
		{name: "wrong secret", clientID: "bad", wantConclusive: true, wantStatus: dexProviderCheckStepFailed},
		{name: "ambiguous response falls back", clientID: "no-introspect", wantConclusive: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &dexProviderCheckResult{}
			conclusive := probeIntrospection(context.Background(), client, result, srv.URL, tt.clientID, "secret")
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
	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{name: "internal svc fails", rawURL: "https://keycloak.keycloak1.svc:8443/realms/d8test", want: dexProviderCheckStepFailed},
		{name: "single label fails", rawURL: "https://keycloak:8443/realms/d8test", want: dexProviderCheckStepFailed},
		{name: "public domain succeeds", rawURL: "https://keycloak.example.com/realms/d8test", want: dexProviderCheckStepSucceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &dexProviderCheckResult{}
			checkPublicBrowserURL(result, "public", tt.rawURL)
			if len(result.checks) != 1 || result.checks[0].Status != tt.want {
				t.Fatalf("expected status %q, got %#v", tt.want, result.checks)
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
