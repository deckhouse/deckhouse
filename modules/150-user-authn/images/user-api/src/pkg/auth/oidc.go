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
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

var (
	ErrNoAuthHeader    = errors.New("no authorization header")
	ErrInvalidAuthType = errors.New("invalid authorization type, expected Bearer")
	ErrTokenValidation = errors.New("token validation failed")
	ErrInvalidSubject  = errors.New("invalid subject claim in token")
)

// Claims identifies the user a token was issued to. The user is only ever taken from the subject:
// the name, preferred_username and email claims come from the provider and may collide between
// users of different providers.
type Claims struct {
	// Subject is the sub claim as Dex issued it.
	Subject string
	// UserID and ConnectorID are the pair the subject encodes. The built-in local password
	// database has the connector ID "local"; external providers use their own connector IDs.
	UserID      string
	ConnectorID string
	// Email only names the account of an external provider in a UserOperation target.
	Email string
	// SessionID (the sid claim) is the browser session of Dex the token was issued in, and GrantID
	// (the d8_gid claim) the refresh token it was issued with: together they name the device of the
	// request. Either is empty when the token has none.
	SessionID string
	GrantID   string
}

type Verifier interface {
	Verify(ctx context.Context, token string) (*Claims, error)
	ExtractToken(r *http.Request) (string, error)
}

// VerifierConfig describes where Dex is and which tokens it issues are accepted.
type VerifierConfig struct {
	// ConnectURL is the in-cluster Dex address used for discovery and keys.
	ConnectURL string
	// IssuerURL is the public issuer in the "iss" claim. ConnectURL is used when it is empty.
	IssuerURL string
	// ClientID is the only client whose ID tokens are accepted.
	ClientID string
	// CAFile holds the CA that signed the certificate of Dex. The system roots are used when it is
	// empty.
	CAFile string
}

type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
	clientID string
}

// NewOIDCVerifier creates a verifier for the ID tokens Dex issues to cfg.ClientID.
func NewOIDCVerifier(ctx context.Context, cfg VerifierConfig) (*OIDCVerifier, error) {
	if cfg.ClientID == "" {
		return nil, errors.New("client ID is required")
	}

	tlsConfig, err := dexTLSConfig(cfg.CAFile)
	if err != nil {
		return nil, err
	}

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
			TLSClientConfig:       tlsConfig,
		},
	}

	clientCtx := oidc.ClientContext(ctx, httpClient)

	// Tokens carry the public issuer in "iss", so validation pins it. It must
	// match Dex's configured issuer exactly, including the trailing slash.
	expectedIssuer := cfg.IssuerURL
	if expectedIssuer == "" {
		expectedIssuer = cfg.ConnectURL
	}

	// Discovery runs against the in-cluster connectURL; the discovery document
	// advertises the public issuer, so allow the mismatch.
	discoveryCtx := clientCtx
	if expectedIssuer != cfg.ConnectURL {
		discoveryCtx = oidc.InsecureIssuerURLContext(clientCtx, expectedIssuer)
	}

	provider, err := oidc.NewProvider(discoveryCtx, cfg.ConnectURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	// Fetch JWKS from the in-cluster host: the advertised jwks_uri points at the
	// public URL, which is unreachable in closed-loop clusters.
	keysURL, err := internalKeysURL(provider, cfg.ConnectURL)
	if err != nil {
		return nil, err
	}
	keySet := oidc.NewRemoteKeySet(clientCtx, keysURL)

	// Only the ID tokens of one client are accepted: the console signs its users in and forwards
	// their tokens. A token issued to any other client, kubeconfig included, is refused even though
	// Dex signed it.
	verifier := oidc.NewVerifier(expectedIssuer, keySet, &oidc.Config{
		ClientID: cfg.ClientID,
	})

	return &OIDCVerifier{
		verifier: verifier,
		clientID: cfg.ClientID,
	}, nil
}

// dexTLSConfig verifies the certificate of Dex against the CA in caFile, or against the system
// roots when caFile is empty. user-api trusts every token signed by the keys it fetches from Dex,
// so whoever could answer in place of Dex inside the cluster could mint tokens for any user.
func dexTLSConfig(caFile string) (*tls.Config, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile == "" {
		return tlsConfig, nil
	}

	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read Dex CA %q: %w", caFile, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("no certificate found in Dex CA %q", caFile)
	}
	tlsConfig.RootCAs = pool

	return tlsConfig, nil
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
		AuthorizedParty string `json:"azp"`
		Email           string `json:"email"`
		SessionID       string `json:"sid"`
		GrantID         string `json:"d8_gid"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("%w: parse claims: %v", ErrTokenValidation, err)
	}

	// The verifier has found the console among the audiences. A token for several audiences is
	// requested by the client named in azp, and Dex adds the requesting client to the audiences, so
	// a client that only names the console as a peer gets a token the console is an audience of.
	// azp tells them apart (OpenID Connect Core 1.0, section 3.1.3.7).
	if claims.AuthorizedParty != "" && claims.AuthorizedParty != v.clientID {
		return nil, fmt.Errorf("%w: the token was requested by another client", ErrTokenValidation)
	}
	if len(idToken.Audience) > 1 && claims.AuthorizedParty == "" {
		return nil, fmt.Errorf("%w: a token for several audiences has no azp claim", ErrTokenValidation)
	}

	userID, connectorID, err := ParseSubject(idToken.Subject)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSubject, err)
	}

	return &Claims{
		Subject:     idToken.Subject,
		UserID:      userID,
		ConnectorID: connectorID,
		Email:       claims.Email,
		SessionID:   claims.SessionID,
		GrantID:     claims.GrantID,
	}, nil
}
