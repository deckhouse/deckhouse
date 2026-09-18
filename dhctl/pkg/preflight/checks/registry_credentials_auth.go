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

// Basic-then-bearer authentication against a container registry, as run by the
// registry-credentials check in registry_credentials.go.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/registryutil"
)

func prepareAuthHTTPClient(ctx context.Context, metaConfig *config.MetaConfig) (*http.Client, error) {
	registry := metaConfig.Registry.Settings.RemoteData
	return registryutil.NewRegistryClient(ctx, string(registry.Scheme), registry.CA)
}

var ErrAuthRegistryFailed = errors.New("authentication failed")

// The two answers that mean something other than "wrong password", and which the operator has to
// be told apart: credentials that are accepted but carry no pull right on the repository, and a
// registry that offers basic authentication only.
var (
	ErrRegistryPullDenied        = errors.New("pull denied for the configured repository")
	ErrRegistryBearerUnsupported = errors.New("the registry offers basic authentication only")
)

// realmRe and serviceRe pull the bearer parameters out of a WWW-Authenticate header. They live
// here, next to their only caller, rather than in registry_proxy.go where they used to sit.
//
// The URL character class is deliberately wide: the previous one accepted lower-case letters and
// a handful of punctuation only, so a registry whose realm contained an upper-case letter or an
// underscore — both legal in a URL — matched nothing, and the operator was told to "consider
// enabling bearer token auth" on a registry that already had it.
var (
	realmRe   = regexp.MustCompile(`realm="(https?://[^"]+)"`)
	serviceRe = regexp.MustCompile(`service="(.*?)"`)
)

// registryV2URL is the /v2/ endpoint of the configured registry.
func registryV2URL(metaConfig *config.MetaConfig) *url.URL {
	registry := metaConfig.Registry.Settings.RemoteData
	registryAddress, _ := registry.AddressAndPath()

	return &url.URL{
		Scheme: strings.ToLower(string(registry.Scheme)),
		Host:   registryAddress,
		Path:   registryPath,
	}
}

func prepareRegistryRequest(ctx context.Context, metaConfig *config.MetaConfig, authData string) (*http.Request, error) {
	registryURL := registryV2URL(metaConfig)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("prepare registry request: %w", err)
	}
	if authData != "" {
		req.Header.Add("Authorization", "Basic "+authData)
	}

	return req, nil
}

func prepareAuthRequest(ctx context.Context, authURL, registryService, authData string, metaConfig *config.MetaConfig) (*http.Request, error) {
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
		return authURL, registryService, fmt.Errorf("cannot authenticate in registry. %w", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Docker-Distribution-API-Version") != "registry/2.0" {
		return authURL, registryService, fmt.Errorf("%w: expected Docker-Distribution-API-Version=registry/2.0 header in response from registry.\nCheck that the container registry address is correct", ErrAuthRegistryFailed)
	}
	wwwAuthHeader := resp.Header.Get("WWW-Authenticate")

	if len(wwwAuthHeader) == 0 {
		return authURL, registryService, fmt.Errorf("WWW-Authenticate header not found. %w", ErrAuthRegistryFailed)
	}

	realmMatches := realmRe.FindStringSubmatch(wwwAuthHeader)
	if len(realmMatches) == 0 {
		// No bearer realm: the registry wants Basic, and the Basic attempt has already failed —
		// so the credentials are wrong. Saying "enable bearer token auth" here, as the old text
		// did, sends the operator to change a registry that is working as designed.
		return authURL, registryService, fmt.Errorf("%w (WWW-Authenticate: %s). %w", ErrRegistryBearerUnsupported, wwwAuthHeader, ErrAuthRegistryFailed)
	}
	authURL = realmMatches[1]

	serviceMatches := serviceRe.FindStringSubmatch(wwwAuthHeader)
	if len(serviceMatches) > 0 {
		registryService = serviceMatches[1]
	}

	return authURL, registryService, nil
}

func checkResponseError(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return ErrAuthRegistryFailed
	case http.StatusForbidden:
		// The credentials were accepted and the repository was refused; that is a permission on
		// the registry side, not a wrong password.
		return fmt.Errorf("%w. %w", ErrRegistryPullDenied, ErrAuthRegistryFailed)
	default:
		return fmt.Errorf("unexpected response status code %d, %w", resp.StatusCode, ErrAuthRegistryFailed)
	}
}

func checkBasicRegistryAuth(ctx context.Context, metaConfig *config.MetaConfig, authData string, client *http.Client) error {
	req, err := prepareRegistryRequest(ctx, metaConfig, authData)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot send request to registry. %w", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Docker-Distribution-API-Version") != "registry/2.0" {
		return fmt.Errorf("%w: expected Docker-Distribution-API-Version=registry/2.0 header in response from registry.\nCheck that the container registry address is correct", ErrAuthRegistryFailed)
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
		return fmt.Errorf("cannot authenticate in registry. %w", err)
	}
	defer resp.Body.Close()

	return checkResponseError(resp)
}
