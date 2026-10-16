/*
Copyright 2024 Flant JSC

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
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/pkg/capnslog"
	"golang.org/x/oauth2"
)

// LDAPConfig groups NewLDAP parameters to avoid the boolean-parameter parade.
type LDAPConfig struct {
	// URL is the public address of Dex, its issuer.
	URL string
	// DexURL is where the provider sends its requests, the Dex Service inside the cluster. Empty
	// means URL.
	DexURL string
	// CAFile is the PEM bundle of the CA that issued the certificate of Dex. It is required: the
	// certificate of the Dex Service is internal, and the check of the credentials rests on it.
	CAFile               string
	ClientID             string
	ClientSecret         string
	Scopes               []string
	GetUserInfo          bool
	BasicAuthUnsupported bool
}

type LDAPProvider struct {
	httpClient  *http.Client
	oidc        *oidc.Provider
	oauth2      *oauth2.Config
	logger      *capnslog.PackageLogger
	getUserInfo bool
}

func NewLDAP(ctx context.Context, cfg LDAPConfig) (Provider, error) {
	if cfg.URL == "" {
		return nil, errors.New("the issuer URL of Dex is empty")
	}
	tlsConfig, err := dexTLSConfig(cfg.CAFile)
	if err != nil {
		return nil, err
	}

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
			TLSClientConfig:       tlsConfig,
		},
	}

	// The endpoints are not discovered: Dex serves them at fixed paths under the issuer, and the
	// discovery document would name them at the public address, which a pod cannot always reach.
	// Only the token and userinfo endpoints are used. The ID token is not verified: Dex answers the
	// password grant over a connection whose certificate is checked against CAFile, and that answer
	// is the check of the credentials.
	base := cfg.URL
	if cfg.DexURL != "" {
		base = cfg.DexURL
	}
	base = strings.TrimSuffix(base, "/")
	providerConfig := oidc.ProviderConfig{
		IssuerURL:   cfg.URL,
		TokenURL:    base + "/token",
		UserInfoURL: base + "/userinfo",
	}
	oidcProvider := providerConfig.NewProvider(oidc.ClientContext(ctx, httpClient))

	oauthCfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     oidcProvider.Endpoint(),
		Scopes:       cfg.Scopes,
	}
	if cfg.BasicAuthUnsupported {
		oauthCfg.Endpoint.AuthStyle = oauth2.AuthStyleInParams
	}

	return &LDAPProvider{
		logger:      capnslog.NewPackageLogger("basic-auth-proxy", "ldap provider"),
		httpClient:  httpClient,
		oidc:        oidcProvider,
		oauth2:      &oauthCfg,
		getUserInfo: cfg.GetUserInfo,
	}, nil
}

func (p *LDAPProvider) ValidateCredentials(ctx context.Context, login, password string) ([]string, error) {
	p.logger.Info("validate credentials via LDAP (password grant)")

	token, err := p.oauth2.PasswordCredentialsToken(
		oidc.ClientContext(ctx, p.httpClient),
		login,
		password,
	)
	if err != nil {
		return nil, err
	}

	if !p.getUserInfo {
		return nil, nil
	}

	p.logger.Info("get user info from Dex for LDAP user")

	info, err := p.oidc.UserInfo(
		oidc.ClientContext(ctx, p.httpClient),
		oauth2.StaticTokenSource(token),
	)
	if err != nil {
		return nil, err
	}

	claims := struct {
		Groups []string `json:"groups"`
	}{}
	if err = info.Claims(&claims); err != nil {
		return nil, err
	}

	return claims.Groups, nil
}

// dexTLSConfig verifies the certificate of Dex against the CA in caFile.
func dexTLSConfig(caFile string) (*tls.Config, error) {
	if caFile == "" {
		return nil, errors.New("the CA of Dex is not set")
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read the CA of Dex: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no certificate in the CA of Dex %q", caFile)
	}
	cfg.RootCAs = pool
	return cfg, nil
}
