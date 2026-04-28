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
}

type Verifier interface {
	Verify(ctx context.Context, token string) (*Claims, error)
	ExtractToken(r *http.Request) (string, error)
}

type OIDCVerifier struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
}

func NewOIDCVerifier(ctx context.Context, issuerURL string) (*OIDCVerifier, error) {
	// Skip TLS verification for Dex connection. This follows the same pattern
	// as dex-authenticator (--ssl-insecure-skip-verify=true).
	// The public Dex URL goes through Ingress with a certificate that may use
	// different CAs depending on the HTTPS mode (CertManager, CustomCertificate, etc).
	// Since this is internal cluster communication and the URL is resolved via DNS,
	// skipping verification is acceptable and more robust than trying to handle
	// all possible CA configurations.
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
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
	provider, err := oidc.NewProvider(clientCtx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	// user-api validates tokens issued by Dex but doesn't have its own
	// client_id registered in Dex. Tokens are issued to other clients
	// (e.g., kubeconfig-generator, dex-authenticator) and we only need
	// to verify the signature and extract user claims.
	verifier := provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
	})

	return &OIDCVerifier{
		provider: provider,
		verifier: verifier,
	}, nil
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
		Username: username,
		Email:    claims.Email,
		Groups:   claims.Groups,
	}, nil
}
