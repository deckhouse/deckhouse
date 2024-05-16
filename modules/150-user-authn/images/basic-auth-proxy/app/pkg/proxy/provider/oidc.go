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
	"golang.org/x/oauth2"
)

type OpenIDConnect struct {
	httpClient  *http.Client
	oidc        *oidc.Provider
	oauth2      *oauth2.Config
	getUserInfo bool
}

func NewOIDC(oidcURL, clientID, clientSecret string, scopes []string) Provider {
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

	provider, err := oidc.NewProvider(context.WithValue(context.Background(), oauth2.HTTPClient, httpClient), oidcURL)
	if err != nil {
		panic(err) // TODO: handle error
	}

	config := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	return &OpenIDConnect{
		httpClient: httpClient,
		oidc:       provider,
		oauth2:     &config,
	}
}

func (p *OpenIDConnect) ValidateCredentials(login, password string) ([]string, error) {
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, p.httpClient)

	// TODO: set authentication strategy to POST (endpoint.AuthStyle = oauth2.AuthStyleInParams)
	//   if the basicAuthUnsupported option is enabled
	token, err := p.oauth2.PasswordCredentialsToken(ctx, login, password)
	if err != nil {
		return nil, err
	}

	// TODO: request user info only if the getUserInfo option is enabled
	info, err := p.oidc.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return nil, err
	}

	// TODO: get the groups claim from the claimMappings settings of the provider
	claims := struct {
		Groups []string `json:"groups"`
	}{}
	if err = info.Claims(&claims); err != nil {
		return nil, err
	}

	return claims.Groups, nil
}
