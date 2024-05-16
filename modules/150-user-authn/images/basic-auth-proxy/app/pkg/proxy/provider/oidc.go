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
	"net"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/pkg/capnslog"
	"golang.org/x/oauth2"
)

type OpenIDConnect struct {
	httpClient  *http.Client
	oidc        *oidc.Provider
	oauth2      *oauth2.Config
	logger      *capnslog.PackageLogger
	getUserInfo bool
}

func NewOIDC(oidcURL, clientID, clientSecret string, basicAuthUnsupported bool, scopes []string) Provider {
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
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		},
	}

	provider, err := oidc.NewProvider(oidc.ClientContext(context.Background(), httpClient), oidcURL)
	if err != nil {
		panic(err) // TODO: handle error
	}

	config := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	if basicAuthUnsupported {
		config.Endpoint.AuthStyle = oauth2.AuthStyleInParams
	}

	return &OpenIDConnect{
		logger:     capnslog.NewPackageLogger("basic-auth-proxy", "provider"),
		httpClient: httpClient,
		oidc:       provider,
		oauth2:     &config,
	}
}

func (p *OpenIDConnect) ValidateCredentials(login, password string) ([]string, error) {
	p.logger.Info("oidc provider validates credentials")
	token, err := p.oauth2.PasswordCredentialsToken(oidc.ClientContext(context.Background(), p.httpClient), login, password)
	if err != nil {
		return nil, err
	}
	p.logger.Info("oidc provider validates credentials successful")
	p.logger.Info("oidc provider gets user info")
	// TODO: request user info only if the getUserInfo option is enabled
	info, err := p.oidc.UserInfo(oidc.ClientContext(context.Background(), p.httpClient), oauth2.StaticTokenSource(token))
	if err != nil {
		return nil, err
	}
	p.logger.Info("oidc provider gets user info successful")
	// TODO: get the groups claim from the claimMappings settings of the provider
	claims := struct {
		Groups []string `json:"groups"`
	}{}
	if err = info.Claims(&claims); err != nil {
		return nil, err
	}
	return claims.Groups, nil
}
