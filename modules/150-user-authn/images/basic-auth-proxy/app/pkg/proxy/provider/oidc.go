/*
Copyright 2021 Flant JSC

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

type OIDCProvider struct {
	apiURL   string
	login    string
	password string

	allowedGroups map[string]struct{}

	httpClient   *http.Client
	oidcProvider *oidc.Provider
	oauth2Config *oauth2.Config
}

func NewOIDCProvider(apiURL, login, password string, scopes, allowedGroups []string) *OIDCProvider {
	client := &http.Client{
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

	groups := make(map[string]struct{})
	for _, group := range allowedGroups {
		groups[group] = struct{}{}
	}

	ctx := context.Background()
	context.WithValue(ctx, oauth2.HTTPClient, client)

	provider, err := oidc.NewProvider(ctx, apiURL)
	if err != nil {
		panic(err) // TODO: handle error
	}

	config := oauth2.Config{
		ClientID:     login,
		ClientSecret: password,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	return &OIDCProvider{
		apiURL:        apiURL,
		login:         login,
		password:      password,
		allowedGroups: groups,
		httpClient:    client,
		oidcProvider:  provider,
		oauth2Config:  &config,
	}
}

func (c *OIDCProvider) ValidateCredentials(login, password string) ([]string, error) {
	ctx := context.Background()
	context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)

	// TODO: set authentication strategy to POST (endpoint.AuthStyle = oauth2.AuthStyleInParams)
	//   if the basicAuthUnsupported option is enabled
	token, err := c.oauth2Config.PasswordCredentialsToken(ctx, login, password)
	if err != nil {
		return nil, err
	}

	// TODO: request user info only if the getUserInfo option is enabled
	info, err := c.oidcProvider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return nil, err
	}

	// TODO: get the groups claim from the claimMappings settings of the provider
	claims := struct {
		Groups []string `json:"groups"`
	}{}
	if err := info.Claims(&claims); err != nil {
		return nil, err
	}

	return claims.Groups, nil
}
