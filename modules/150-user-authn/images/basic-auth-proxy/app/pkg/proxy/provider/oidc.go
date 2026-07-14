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
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/pkg/capnslog"
	"golang.org/x/oauth2"
)

// OIDCConfig groups NewOIDC parameters.
type OIDCConfig struct {
	URL                  string
	ClientID             string
	ClientSecret         string
	Scopes               []string
	GetUserInfo          bool
	BasicAuthUnsupported bool
}

type OpenIDConnect struct {
	httpClient  *http.Client
	oidc        *oidc.Provider
	oauth2      *oauth2.Config
	logger      *capnslog.PackageLogger
	getUserInfo bool
}

func NewOIDC(ctx context.Context, cfg OIDCConfig) (Provider, error) {
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
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	oidcProvider, err := oidc.NewProvider(oidc.ClientContext(ctx, httpClient), cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("create OIDC provider %q: %w", cfg.URL, err)
	}

	oauthCfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     oidcProvider.Endpoint(),
		Scopes:       cfg.Scopes,
	}
	if cfg.BasicAuthUnsupported {
		oauthCfg.Endpoint.AuthStyle = oauth2.AuthStyleInParams
	}

	return &OpenIDConnect{
		logger:      capnslog.NewPackageLogger("basic-auth-proxy", "oidc provider"),
		httpClient:  httpClient,
		oidc:        oidcProvider,
		oauth2:      &oauthCfg,
		getUserInfo: cfg.GetUserInfo,
	}, nil
}

func (p *OpenIDConnect) ValidateCredentials(ctx context.Context, login, password string) ([]string, error) {
	p.logger.Info("validate credentials")
	token, err := p.oauth2.PasswordCredentialsToken(oidc.ClientContext(ctx, p.httpClient), login, password)
	if err != nil {
		return nil, err
	}
	p.logger.Info("validate credentials successful")

	if !p.getUserInfo {
		return nil, nil
	}

	p.logger.Info("get user info")
	info, err := p.oidc.UserInfo(oidc.ClientContext(ctx, p.httpClient), oauth2.StaticTokenSource(token))
	if err != nil {
		return nil, err
	}
	p.logger.Info("get user info successful")

	// TODO: get the groups claim from the claimMappings settings of the provider
	claims := struct {
		Groups []string `json:"groups"`
	}{}
	if err = info.Claims(&claims); err != nil {
		return nil, err
	}
	return claims.Groups, nil
}
