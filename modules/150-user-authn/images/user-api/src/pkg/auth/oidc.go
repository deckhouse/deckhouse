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
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

var (
	ErrNoAuthHeader    = errors.New("no authorization header")
	ErrInvalidAuthType = errors.New("invalid authorization type, expected Bearer")
	ErrTokenValidation = errors.New("token validation failed")
	ErrMissingUsername = errors.New("missing username claim in token")
)

type Claims struct {
	Username string
	Email    string
	Groups   []string
	// ConnectorID is the Dex connector that issued the token
	// (federated_claims.connector_id). The built-in local password DB uses
	// "local"; external providers use their own connector IDs.
	ConnectorID string
}

type Verifier interface {
	Verify(ctx context.Context, token string) (*Claims, error)
	ExtractToken(r *http.Request) (string, error)
}

type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
}

// NewOIDCVerifier creates a verifier for tokens issued by Dex.
func NewOIDCVerifier(ctx context.Context, connectURL, issuerURL string) (*OIDCVerifier, error) {
	// Skip TLS verification: this is in-cluster traffic to the Dex Service,
	// same pattern as dex-authenticator (--ssl-insecure-skip-verify=true).
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			// Never route in-cluster traffic through an egress proxy: a cluster-wide
			// HTTP(S)_PROXY injected into pods can't reach cluster IPs.
			Proxy: nil,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}

	clientCtx := oidc.ClientContext(ctx, httpClient)

	// Tokens carry the public issuer in "iss", so validation pins it. It must
	// match Dex's configured issuer exactly, including the trailing slash.
	expectedIssuer := issuerURL
	if expectedIssuer == "" {
		expectedIssuer = connectURL
	}

	// Discovery runs against the in-cluster connectURL; the discovery document
	// advertises the public issuer, so allow the mismatch.
	discoveryCtx := clientCtx
	if expectedIssuer != connectURL {
		discoveryCtx = oidc.InsecureIssuerURLContext(clientCtx, expectedIssuer)
	}

	provider, err := oidc.NewProvider(discoveryCtx, connectURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	// Fetch JWKS from the in-cluster host: the advertised jwks_uri points at the
	// public URL, which is unreachable in closed-loop clusters.
	keysURL, err := internalKeysURL(provider, connectURL)
	if err != nil {
		return nil, err
	}
	keySet := oidc.NewRemoteKeySet(clientCtx, keysURL)

	// SkipClientIDCheck: user-api has no client_id of its own; tokens are issued
	// to other clients and we only verify signature and extract claims.
	verifier := oidc.NewVerifier(expectedIssuer, keySet, &oidc.Config{
		SkipClientIDCheck: true,
	})

	return &OIDCVerifier{
		verifier: verifier,
	}, nil
}

// internalKeysURL rewrites the advertised jwks_uri scheme/host to the in-cluster
// connectURL (keeping its path), falling back to Dex's "/keys" path if absent.
func internalKeysURL(provider *oidc.Provider, connectURL string) (string, error) {
	conn, err := url.Parse(connectURL)
	if err != nil {
		return "", fmt.Errorf("invalid connect URL %q: %w", connectURL, err)
	}

	fallback := strings.TrimSuffix(connectURL, "/") + "/keys"

	var claims struct {
		JWKSURL string `json:"jwks_uri"`
	}
	if err := provider.Claims(&claims); err != nil || claims.JWKSURL == "" {
		return fallback, nil
	}

	jwks, err := url.Parse(claims.JWKSURL)
	if err != nil {
		return fallback, nil
	}

	jwks.Scheme = conn.Scheme
	jwks.Host = conn.Host
	return jwks.String(), nil
}

func (v *OIDCVerifier) ExtractToken(r *http.Request) (string, error) {
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

func (v *OIDCVerifier) Verify(ctx context.Context, token string) (*Claims, error) {
	idToken, err := v.verifier.Verify(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTokenValidation, err)
	}

	var claims struct {
		PreferredUsername string   `json:"preferred_username"`
		Email             string   `json:"email"`
		Groups            []string `json:"groups"`
		Name              string   `json:"name"`
		FederatedClaims   struct {
			ConnectorID string `json:"connector_id"`
		} `json:"federated_claims"`
	}

	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	// Priority: preferred_username > name > email
	// For local Dex users, 'name' matches the Password CRD username.
	// 'email' is a fallback for external providers that don't set name/preferred_username.
	username := claims.PreferredUsername
	if username == "" {
		username = claims.Name
	}
	if username == "" {
		username = claims.Email
	}
	if username == "" {
		return nil, ErrMissingUsername
	}

	return &Claims{
		Username:    username,
		Email:       claims.Email,
		Groups:      claims.Groups,
		ConnectorID: claims.FederatedClaims.ConnectorID,
	}, nil
}
