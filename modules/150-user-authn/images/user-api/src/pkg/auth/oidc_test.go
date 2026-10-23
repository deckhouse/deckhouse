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

package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const (
	testKeyID = "test-key"

	// The public issuer host is deliberately unreachable: any attempt to fetch
	// discovery or JWKS from it would fail, proving the code uses the connect host.
	testPublicIssuer = "https://dex.unreachable.invalid/"

	testConsoleClientID = "console-d8-console-dex-authenticator"
	testJaneSubject     = "CgRqYW5lEgVsb2NhbA"
)

// testDex serves the discovery document and the keys of a Dex whose tokens name testPublicIssuer.
type testDex struct {
	server   *httptest.Server
	key      *rsa.PrivateKey
	keysHits atomic.Int64
}

func newTestDex(t *testing.T) *testDex {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	dex := &testDex{key: key}

	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       key.Public(),
		KeyID:     testKeyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}}}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 testPublicIssuer,
			"authorization_endpoint": testPublicIssuer + "auth",
			"token_endpoint":         testPublicIssuer + "token",
			// Advertise the JWKS on the unreachable public host, as Dex does.
			"jwks_uri": testPublicIssuer + "keys",
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		dex.keysHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	dex.server = httptest.NewTLSServer(mux)
	t.Cleanup(dex.server.Close)

	return dex
}

// caFile writes the certificate of the test server, which is its own CA, to a file.
func (d *testDex) caFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "ca.crt")
	block := &pem.Block{Type: "CERTIFICATE", Bytes: d.server.Certificate().Raw}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write CA file: %v", err)
	}
	return path
}

func (d *testDex) verifier(t *testing.T) *OIDCVerifier {
	t.Helper()

	v, err := NewOIDCVerifier(context.Background(), VerifierConfig{
		ConnectURL: d.server.URL,
		IssuerURL:  testPublicIssuer,
		ClientID:   testConsoleClientID,
		CAFile:     d.caFile(t),
	})
	if err != nil {
		t.Fatalf("NewOIDCVerifier: %v", err)
	}
	return v
}

// sign issues a token for the console with the claims overridden by overrides; a nil value
// removes a claim.
func (d *testDex) sign(t *testing.T, overrides map[string]any) string {
	t.Helper()

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: d.key, KeyID: testKeyID}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	now := time.Now()
	claims := map[string]any{
		"iss":                testPublicIssuer,
		"aud":                testConsoleClientID,
		"sub":                testJaneSubject,
		"iat":                now.Unix(),
		"exp":                now.Add(time.Hour).Unix(),
		"preferred_username": "alice",
		"name":               "alice",
		"email":              "jane@example.com",
		"federated_claims": map[string]any{
			"connector_id": "local",
			"user_id":      "jane",
		},
	}
	for name, value := range overrides {
		if value == nil {
			delete(claims, name)
			continue
		}
		claims[name] = value
	}

	token, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

// TestNewOIDCVerifier_RoutesDiscoveryAndJWKSInternally verifies the closed-loop
// behaviour: discovery is fetched from the in-cluster connect URL, the public
// issuer (which differs from the connect URL) is accepted for "iss" validation,
// and JWKS keys are fetched from the connect host even though the discovery
// document advertises an unreachable public jwks_uri.
func TestNewOIDCVerifier_RoutesDiscoveryAndJWKSInternally(t *testing.T) {
	dex := newTestDex(t)
	v := dex.verifier(t)

	if _, err := v.Verify(context.Background(), dex.sign(t, nil)); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if dex.keysHits.Load() == 0 {
		t.Error("JWKS was not fetched from the connect host; keys endpoint never hit")
	}
}

func TestNewOIDCVerifier_VerifiesTheCertificateOfDex(t *testing.T) {
	dex := newTestDex(t)

	tests := []struct {
		name    string
		caFile  string
		wantErr bool
	}{
		{name: "the CA of Dex", caFile: dex.caFile(t)},
		// The test server certificate is not signed by any system root.
		{name: "system roots", caFile: "", wantErr: true},
		{name: "another CA", caFile: otherCAFile(t), wantErr: true},
		{name: "missing CA file", caFile: filepath.Join(t.TempDir(), "absent.crt"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOIDCVerifier(context.Background(), VerifierConfig{
				ConnectURL: dex.server.URL,
				IssuerURL:  testPublicIssuer,
				ClientID:   testConsoleClientID,
				CAFile:     tt.caFile,
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewOIDCVerifier() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// otherCAFile writes a CA that has signed nothing the test server presents. All httptest servers
// share one certificate, so a second server would not do.
func otherCAFile(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "other CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}

	path := filepath.Join(t.TempDir(), "other-ca.crt")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write CA file: %v", err)
	}
	return path
}

func TestNewOIDCVerifier_RequiresClientID(t *testing.T) {
	dex := newTestDex(t)

	_, err := NewOIDCVerifier(context.Background(), VerifierConfig{
		ConnectURL: dex.server.URL,
		IssuerURL:  testPublicIssuer,
		CAFile:     dex.caFile(t),
	})
	if err == nil {
		t.Fatal("NewOIDCVerifier() without a client ID succeeded, want an error")
	}
}

func TestOIDCVerifier_Verify(t *testing.T) {
	dex := newTestDex(t)
	v := dex.verifier(t)

	tests := []struct {
		name      string
		overrides map[string]any
		want      *Claims
		wantErr   error
	}{
		{
			name: "token of the console",
			want: &Claims{Subject: testJaneSubject, UserID: "jane", ConnectorID: "local", Email: "jane@example.com"},
		},
		{
			// The console asks for the kubernetes audience; Dex adds the console and names it in azp.
			name: "token of the console for kubernetes",
			overrides: map[string]any{
				"aud": []string{"kubernetes", testConsoleClientID},
				"azp": testConsoleClientID,
			},
			want: &Claims{Subject: testJaneSubject, UserID: "jane", ConnectorID: "local", Email: "jane@example.com"},
		},
		{
			name: "external user is taken from sub, not from the profile claims",
			overrides: map[string]any{
				"sub":                "CiNjbj1qZG9lLG91PXBlb3BsZSxkYz1leGFtcGxlLGRjPW9yZxIEbGRhcA",
				"email":              "jdoe@example.org",
				"preferred_username": "jane",
				"federated_claims":   map[string]any{"connector_id": "local", "user_id": "jane"},
			},
			want: &Claims{
				Subject:     "CiNjbj1qZG9lLG91PXBlb3BsZSxkYz1leGFtcGxlLGRjPW9yZxIEbGRhcA",
				UserID:      "cn=jdoe,ou=people,dc=example,dc=org",
				ConnectorID: "ldap",
				Email:       "jdoe@example.org",
			},
		},
		{
			name:      "token naming its device",
			overrides: map[string]any{"sid": "fehzwdh5qy4oyqcn7qhkcgzbm", "d8_gid": "qx3w2ttiyq5ovzvbuehdcbbse"},
			want: &Claims{
				Subject: testJaneSubject, UserID: "jane", ConnectorID: "local", Email: "jane@example.com",
				SessionID: "fehzwdh5qy4oyqcn7qhkcgzbm", GrantID: "qx3w2ttiyq5ovzvbuehdcbbse",
			},
		},
		{
			name:      "token of another client",
			overrides: map[string]any{"aud": "kubeconfig-generator"},
			wantErr:   ErrTokenValidation,
		},
		{
			name:      "token of kubernetes only",
			overrides: map[string]any{"aud": "kubernetes"},
			wantErr:   ErrTokenValidation,
		},
		{
			// Another client that lists the console as a trusted peer gets the console among the
			// audiences, but it is the authorized party.
			name: "token requested by another client for the console",
			overrides: map[string]any{
				"aud": []string{testConsoleClientID, "other-dex-authenticator"},
				"azp": "other-dex-authenticator",
			},
			wantErr: ErrTokenValidation,
		},
		{
			name:      "several audiences without azp",
			overrides: map[string]any{"aud": []string{"kubernetes", testConsoleClientID}},
			wantErr:   ErrTokenValidation,
		},
		{
			name:      "single audience with a foreign azp",
			overrides: map[string]any{"azp": "other-dex-authenticator"},
			wantErr:   ErrTokenValidation,
		},
		{
			name:      "foreign issuer",
			overrides: map[string]any{"iss": "https://evil.example.com/"},
			wantErr:   ErrTokenValidation,
		},
		{
			name:      "expired",
			overrides: map[string]any{"exp": time.Now().Add(-time.Minute).Unix()},
			wantErr:   ErrTokenValidation,
		},
		{
			name:      "subject that is not a Dex subject",
			overrides: map[string]any{"sub": "jane"},
			wantErr:   ErrInvalidSubject,
		},
		{
			name:      "no subject",
			overrides: map[string]any{"sub": nil},
			wantErr:   ErrInvalidSubject,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := v.Verify(context.Background(), dex.sign(t, tt.overrides))
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Verify() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Verify() error = %v", err)
			}
			if *claims != *tt.want {
				t.Errorf("Verify() = %+v, want %+v", *claims, *tt.want)
			}
		})
	}
}

func TestOIDCVerifier_VerifyRefusesAForeignSignature(t *testing.T) {
	dex := newTestDex(t)
	v := dex.verifier(t)

	// Same key ID, another key: the signature does not verify.
	impostor := newTestDex(t)
	if _, err := v.Verify(context.Background(), impostor.sign(t, nil)); !errors.Is(err, ErrTokenValidation) {
		t.Fatalf("Verify() error = %v, want %v", err, ErrTokenValidation)
	}
}

func TestOIDCVerifier_ExtractToken(t *testing.T) {
	// Test the real OIDCVerifier.ExtractToken method
	verifier := &OIDCVerifier{}

	tests := []struct {
		name       string
		authHeader string
		wantToken  string
		wantErr    error
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer mytoken123",
			wantToken:  "mytoken123",
			wantErr:    nil,
		},
		{
			name:       "no header",
			authHeader: "",
			wantToken:  "",
			wantErr:    ErrNoAuthHeader,
		},
		{
			name:       "invalid type",
			authHeader: "Basic dXNlcjpwYXNz",
			wantToken:  "",
			wantErr:    ErrInvalidAuthType,
		},
		{
			name:       "bearer lowercase - should work (case insensitive)",
			authHeader: "bearer mytoken",
			wantToken:  "mytoken",
			wantErr:    nil,
		},
		{
			name:       "BEARER uppercase - should work (case insensitive)",
			authHeader: "BEARER mytoken",
			wantToken:  "mytoken",
			wantErr:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			token, err := verifier.ExtractToken(req)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ExtractToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if token != tt.wantToken {
				t.Errorf("ExtractToken() token = %v, want %v", token, tt.wantToken)
			}
		})
	}
}
