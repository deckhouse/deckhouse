// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checks

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
)

func prepareAuthHTTPClient(metaConfig *config.MetaConfig) (*http.Client, error) {
	registry := metaConfig.Registry.Settings.RemoteData

	client := &http.Client{}
	httpTransport := http.DefaultTransport.(*http.Transport).Clone()

	if strings.ToLower(string(registry.Scheme)) == "http" {
		httpTransport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	if len(registry.CA) == 0 {
		client.Transport = httpTransport
		return client, nil
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM([]byte(registry.CA)); !ok {
		return nil, fmt.Errorf("invalid cert in CA PEM")
	}

	httpTransport.TLSClientConfig = &tls.Config{
		RootCAs: certPool,
	}

	client.Transport = httpTransport
	return client, nil
}

type registryAuthCheck struct {
	MetaConfig *config.MetaConfig
}

const RegistryAuthCheckName preflightnew.CheckName = "registry-auth"

func (registryAuthCheck) Description() string {
	return "registry credentials are valid"
}

func (registryAuthCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePreInfra
}

func (registryAuthCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.DefaultRetryPolicy
}

var ErrAuthRegistryFailed = errors.New("authentication failed")

func (c registryAuthCheck) Run(ctx context.Context) error {
	if c.MetaConfig == nil {
		return fmt.Errorf("meta config is required")
	}

	client, err := prepareAuthHTTPClient(c.MetaConfig)
	if err != nil {
		return err
	}

	authData := c.MetaConfig.Registry.Settings.RemoteData.AuthBase64()

	if err := checkBasicRegistryAuth(ctx, c.MetaConfig, authData, client); err == nil {
		return nil
	} else if !errors.Is(err, ErrAuthRegistryFailed) {
		return err
	}

	return checkTokenRegistryAuth(ctx, c.MetaConfig, authData, client)
}

func prepareRegistryRequest(ctx context.Context, metaConfig *config.MetaConfig, authData string) (*http.Request, error) {
	registry := metaConfig.Registry.Settings.RemoteData
	registryAddress, _ := registry.AddressAndPath()

	registryURL := &url.URL{
		Scheme: strings.ToLower(string(registry.Scheme)),
		Host:   registryAddress,
		Path:   registryPath,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("prepare registry request: %w", err)
	}
	if authData != "" {
		req.Header.Add("Authorization", "Basic "+authData)
	}

	return req, nil
}

func prepareAuthRequest(ctx context.Context, authURL string, registryService string, authData string, metaConfig *config.MetaConfig) (*http.Request, error) {
	registry := metaConfig.Registry.Settings.RemoteData
	_, registryPath := registry.AddressAndPath()

	authURLValues := url.Values{}
	authURLValues.Add("service", registryService)
	authURLValues.Add("scope", fmt.Sprintf("repository:%s:pull", strings.TrimLeft(registryPath, "/")))

	authURL = fmt.Sprintf("%s?%s", authURL, authURLValues.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("prepare auth request: %w", err)
	}
	if authData != "" {
		req.Header.Add("Authorization", "Basic "+authData)
	}

	return req, nil
}

func getAuthRealmAndService(ctx context.Context, metaConfig *config.MetaConfig, client *http.Client) (string, string, error) {
	authURL := ""
	registryService := ""

	req, err := prepareRegistryRequest(ctx, metaConfig, "")
	if err != nil {
		return authURL, registryService, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return authURL, registryService, fmt.Errorf("cannot auth in registry. %w", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Docker-Distribution-API-Version") != "registry/2.0" {
		return authURL, registryService, fmt.Errorf("%w: expected Docker-Distribution-API-Version=registry/2.0 header in response from registry.\nCheck if container registry address is correct", ErrAuthRegistryFailed)
	}
	wwwAuthHeader := resp.Header.Get("WWW-Authenticate")

	if len(wwwAuthHeader) == 0 {
		return authURL, registryService, fmt.Errorf("WWW-Authenticate header not found. %w", ErrAuthRegistryFailed)
	}

	realmMatches := realmRe.FindStringSubmatch(wwwAuthHeader)
	if len(realmMatches) == 0 {
		return authURL, registryService, fmt.Errorf("couldn't find bearer realm parameter, consider enabling bearer token auth in your registry, returned header:%s. %w", wwwAuthHeader, ErrAuthRegistryFailed)
	}
	authURL = realmMatches[1]

	serviceMatches := serviceRe.FindStringSubmatch(wwwAuthHeader)
	if len(serviceMatches) > 0 {
		registryService = serviceMatches[1]
	}

	return authURL, registryService, nil
}

func checkResponseError(resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrAuthRegistryFailed
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status code %d, %w", resp.StatusCode, ErrAuthRegistryFailed)
	}

	return nil
}

func checkBasicRegistryAuth(ctx context.Context, metaConfig *config.MetaConfig, authData string, client *http.Client) error {
	req, err := prepareRegistryRequest(ctx, metaConfig, authData)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot request to registry. %w", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Docker-Distribution-API-Version") != "registry/2.0" {
		return fmt.Errorf("%w: expected Docker-Distribution-API-Version=registry/2.0 header in response from registry.\nCheck if container registry address is correct", ErrAuthRegistryFailed)
	}

	return checkResponseError(resp)
}

func checkTokenRegistryAuth(ctx context.Context, metaConfig *config.MetaConfig, authData string, client *http.Client) error {
	authURL, registryService, err := getAuthRealmAndService(ctx, metaConfig, client)
	if err != nil {
		return err
	}

	req, err := prepareAuthRequest(ctx, authURL, registryService, authData, metaConfig)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot auth in registry. %w", err)
	}
	defer resp.Body.Close()

	return checkResponseError(resp)
}

func RegistryProxyAuth(meta *config.MetaConfig) preflightnew.Check {
	check := registryAuthCheck{MetaConfig: meta}
	return preflightnew.Check{
		Name:        RegistryAuthCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
