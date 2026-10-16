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

package provider

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeDex answers the password grant and userinfo the way Dex does and records the paths it got.
type fakeDex struct {
	*httptest.Server
	mu    sync.Mutex
	paths []string
}

func newFakeDex(t *testing.T) *fakeDex {
	t.Helper()
	d := &fakeDex{}
	d.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		d.paths = append(d.paths, r.URL.Path)
		d.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "password" || r.PostForm.Get("password") != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "bearer", "expires_in": 600})
		case "/userinfo":
			_ = json.NewEncoder(w).Encode(map[string]any{"sub": "jane", "groups": []string{"admins"}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(d.Close)
	return d
}

func (d *fakeDex) requested() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.paths)
}

// caFile writes the certificate of server as a PEM CA bundle.
func caFile(t *testing.T, server *httptest.Server) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ca.crt")
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// otherCAFile writes a self-signed CA that did not sign the certificate of any test server
// (httptest servers all share one built-in certificate).
func otherCAFile(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "another CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "other-ca.crt")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewLDAP_SendsRequestsToTheDexService(t *testing.T) {
	t.Parallel()

	dex := newFakeDex(t)
	p, err := NewLDAP(t.Context(), LDAPConfig{
		// The issuer is a public name the pod cannot reach; nothing may be sent there.
		URL:          "https://dex.unreachable.invalid/",
		DexURL:       dex.URL + "/",
		CAFile:       caFile(t, dex.Server),
		ClientID:     "kubernetes",
		ClientSecret: "client-secret",
		Scopes:       []string{"openid", "groups"},
		GetUserInfo:  true,
	})
	if err != nil {
		t.Fatalf("NewLDAP: %v", err)
	}

	groups, err := p.ValidateCredentials(t.Context(), "jane", "secret")
	if err != nil {
		t.Fatalf("ValidateCredentials: %v", err)
	}
	if !slices.Equal(groups, []string{"admins"}) {
		t.Errorf("groups = %v, want [admins]", groups)
	}
	if got := dex.requested(); !slices.Equal(got, []string{"/token", "/userinfo"}) {
		t.Errorf("Dex got %v, want [/token /userinfo]", got)
	}

	if _, err := p.ValidateCredentials(t.Context(), "jane", "wrong"); err == nil {
		t.Error("a wrong password was accepted")
	}
}

// The provider used to discover Dex at start and crash when the public address was out of reach.
func TestNewLDAP_DoesNotReachDexAtStart(t *testing.T) {
	t.Parallel()

	dex := newFakeDex(t)
	if _, err := NewLDAP(t.Context(), LDAPConfig{
		URL:    "https://dex.unreachable.invalid/",
		DexURL: "https://127.0.0.1:1/",
		CAFile: caFile(t, dex.Server),
	}); err != nil {
		t.Fatalf("NewLDAP: %v", err)
	}
	if got := dex.requested(); len(got) != 0 {
		t.Errorf("Dex got %v at start", got)
	}
}

func TestNewLDAP_VerifiesTheCertificateOfDex(t *testing.T) {
	t.Parallel()

	dex := newFakeDex(t)

	for _, tt := range []struct {
		name   string
		caFile string
	}{
		{name: "another CA", caFile: otherCAFile(t)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := NewLDAP(t.Context(), LDAPConfig{
				URL:    "https://dex.unreachable.invalid/",
				DexURL: dex.URL,
				CAFile: tt.caFile,
			})
			if err != nil {
				t.Fatalf("NewLDAP: %v", err)
			}
			_, err = p.ValidateCredentials(t.Context(), "jane", "secret")
			if err == nil || !strings.Contains(err.Error(), "certificate") {
				t.Errorf("ValidateCredentials error = %v, want a certificate error", err)
			}
		})
	}
	if got := dex.requested(); len(got) != 0 {
		t.Errorf("Dex answered requests over an unverified connection: %v", got)
	}
}

func TestNewLDAP_RefusesABadCAFile(t *testing.T) {
	t.Parallel()

	empty := filepath.Join(t.TempDir(), "empty.crt")
	if err := os.WriteFile(empty, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		caFile string
	}{
		{name: "not set", caFile: ""},
		{name: "missing", caFile: filepath.Join(t.TempDir(), "missing.crt")},
		{name: "no certificate", caFile: empty},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewLDAP(t.Context(), LDAPConfig{URL: "https://dex.example.com/", CAFile: tt.caFile}); err == nil {
				t.Error("NewLDAP accepted the CA file")
			}
		})
	}
}
