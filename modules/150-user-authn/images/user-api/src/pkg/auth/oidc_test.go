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
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

type MockVerifier struct {
	claims *Claims
	err    error
}

func (m *MockVerifier) Verify(_ context.Context, _ string) (*Claims, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.claims, nil
}

func (m *MockVerifier) ExtractToken(r *http.Request) (string, error) {
	// Use the same case-insensitive logic as OIDCVerifier
	return extractBearerToken(r)
}

// extractBearerToken extracts Bearer token from Authorization header (shared logic)
func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrNoAuthHeader
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrInvalidAuthType
	}
	return parts[1], nil
}

// TestNewOIDCVerifier_RoutesDiscoveryAndJWKSInternally verifies the closed-loop
// behaviour: discovery is fetched from the in-cluster connect URL, the public
// issuer (which differs from the connect URL) is accepted for "iss" validation,
// and JWKS keys are fetched from the connect host even though the discovery
// document advertises an unreachable public jwks_uri.
func TestNewOIDCVerifier_RoutesDiscoveryAndJWKSInternally(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	const keyID = "test-key"

	// The public issuer host is deliberately unreachable: any attempt to fetch
	// discovery or JWKS from it would fail, proving the code uses the connect host.
	const publicIssuer = "https://dex.unreachable.invalid/"

	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       priv.Public(),
		KeyID:     keyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}}}

	var keysHits int
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 publicIssuer,
			"authorization_endpoint": publicIssuer + "auth",
			"token_endpoint":         publicIssuer + "token",
			// Advertise the JWKS on the unreachable public host, as Dex does.
			"jwks_uri": publicIssuer + "keys",
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		keysHits++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	ts := httptest.NewTLSServer(mux)
	defer ts.Close()

	ctx := context.Background()
	v, err := NewOIDCVerifier(ctx, ts.URL, publicIssuer)
	if err != nil {
		t.Fatalf("NewOIDCVerifier: %v", err)
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: priv, KeyID: keyID}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	now := time.Now()
	token, err := jwt.Signed(signer).Claims(map[string]any{
		"iss":                publicIssuer,
		"aud":                "some-other-client",
		"sub":                "CgR0ZXN0",
		"iat":                now.Unix(),
		"exp":                now.Add(time.Hour).Unix(),
		"preferred_username": "alice",
		"email":              "alice@example.com",
		"groups":             []string{"admins"},
		"federated_claims": map[string]any{
			"connector_id": "local",
			"user_id":      "alice",
		},
	}).Serialize()
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	claims, err := v.Verify(ctx, token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Username != "alice" {
		t.Errorf("username = %q, want %q", claims.Username, "alice")
	}
	if claims.ConnectorID != "local" {
		t.Errorf("connector ID = %q, want %q", claims.ConnectorID, "local")
	}
	if keysHits == 0 {
		t.Error("JWKS was not fetched from the connect host; keys endpoint never hit")
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
