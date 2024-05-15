// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/frontend"
)

var (
	ErrBadProxyConfig      = errors.New("Bad proxy config")
	ErrRegistryUnreachable = errors.New("Could not reach registry over proxy")
	ErrAuthFailed          = errors.New("authentication failed")

	realmRe   = regexp.MustCompile(`realm="(http[s]{0,1}:\/\/[a-z0-9\.\:\/\-]+)"`)
	serviceRe = regexp.MustCompile(`service="(.*?)"`)
)

const (
	ProxyTunnelPort      = "22323"
	registryPath         = "/v2/"
	httpClientTimeoutSec = 20
)

func (pc *Checker) CheckRegistryAccessThroughProxy() error {
	if app.PreflightSkipRegistryThroughProxy {
		log.InfoLn("Checking if registry is accessible through proxy was skipped")
		return nil
	}

	log.DebugLn("Checking if registry is accessible through proxy")

	proxyUrl, noProxyAddresses, err := getProxyFromMetaConfig(pc.metaConfig)
	if err != nil {
		return fmt.Errorf("get proxy config: %w", err)
	}
	if proxyUrl == nil {
		log.DebugLn("No proxy is configured, skipping check")
		return nil
	}

	if tryToSkippingCheck(pc.metaConfig.Registry.Address, noProxyAddresses) {
		log.DebugLn("Registry address found in proxy.noProxy list, skipping check")
		return nil
	}

	tun, err := setupSSHTunnelToProxyAddr(pc.sshClient, proxyUrl)
	if err != nil {
		return fmt.Errorf(`Cannot setup tunnel to control-plane host: %w.
Please check connectivity to control-plane host and that the sshd config parameter 'AllowTcpForwarding' set to 'yes' on control-plane node.`, err)
	}
	defer tun.Stop()

	registryURL := &url.URL{Scheme: pc.metaConfig.Registry.Scheme, Host: pc.metaConfig.Registry.Address, Path: "/v2/"}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL.String(), nil)
	if err != nil {
		return fmt.Errorf("prepare request: %w", err)
	}

	httpCl := buildHTTPClientWithLocalhostProxy(proxyUrl)
	resp, err := httpCl.Do(req)
	if err != nil {
		return fmt.Errorf(`Container registry API connectivity check was failed with error: %w.
Please check connectivity from the control-plane node to the proxy and from the proxy to the container registry.`, err)
	}

	if err = checkResponseIsFromDockerRegistry(resp); err != nil {
		return err
	}

	return nil
}

func tryToSkippingCheck(registryAddress string, noProxyAddresses []string) bool {
	for _, noProxyAddress := range noProxyAddresses {
		if registryAddress == noProxyAddress {
			return true
		}

		registryIPAddr, _ := net.ResolveIPAddr("ip", registryAddress)
		if registryIPAddr == nil {
			continue
		}

		noProxyAddressIPAddr, _ := net.ResolveIPAddr("ip", noProxyAddress)
		if noProxyAddressIPAddr != nil {
			if noProxyAddressIPAddr.IP.Equal(registryIPAddr.IP) {
				return true
			}

			continue
		}

		_, noProxyIPNet, _ := net.ParseCIDR(noProxyAddress)
		if noProxyIPNet != nil && noProxyIPNet.Contains(registryIPAddr.IP) {
			return true
		}
	}

	return false
}

func buildHTTPClientWithLocalhostProxy(proxyUrl *url.URL) *http.Client {
	localhostProxy := proxyUrl
	localhostProxy.Host = net.JoinHostPort("localhost", ProxyTunnelPort)
	return &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyURL(localhostProxy),
			DisableKeepAlives: true,
		},
	}
}

func getProxyFromMetaConfig(metaConfig *config.MetaConfig) (*url.URL, []string, error) {
	proxyConfig, err := metaConfig.EnrichProxyData()
	switch {
	case err != nil:
		return nil, nil, err
	case proxyConfig == nil:
		return nil, nil, nil
	}

	var proxyAddrClause any
	if proxyAddr, hasHTTPSProxy := proxyConfig["httpsProxy"]; hasHTTPSProxy {
		proxyAddrClause = proxyAddr
	} else if proxyAddr, hasHTTPProxy := proxyConfig["httpProxy"]; hasHTTPProxy {
		proxyAddrClause = proxyAddr
	} else {
		return nil, nil, fmt.Errorf("%w: no proxy address was given", ErrBadProxyConfig)
	}

	noProxyClause, hasNoProxy := proxyConfig["noProxy"]
	var noProxyAddresses []string
	if hasNoProxy {
		addrs, isStringSlice := noProxyClause.([]string)
		if !isStringSlice {
			return nil, nil, fmt.Errorf("proxy.noProxy is not a set of addresses")
		}
		noProxyAddresses = addrs
	}

	proxyAddr, proxyAddrIsString := proxyAddrClause.(string)
	if !proxyAddrIsString {
		return nil, nil, fmt.Errorf(`%w: malformed proxy address: "%v"`, ErrBadProxyConfig, proxyAddr)
	}

	proxyUrl, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, nil, fmt.Errorf(`%s: %w`, ErrBadProxyConfig, err)
	}

	return proxyUrl, noProxyAddresses, nil
}

func checkResponseIsFromDockerRegistry(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf(
			"%w: got %d status code from the container registry API, this is not a valid registry API response.\n"+
				"Check if container registry address is correct and if there is any reverse proxies that might be misconfigured.",
			ErrRegistryUnreachable,
			resp.StatusCode,
		)
	}

	// https://docs.docker.com/registry/spec/api/#api-version-check
	if resp.Header.Get("Docker-Distribution-API-Version") != "registry/2.0" {
		return fmt.Errorf(
			"%w: expected Docker-Distribution-API-Version=registry/2.0 header in response from registry.\n"+
				"Check if container registry address is correct and if there is any reverse proxies that might be misconfigured",
			ErrRegistryUnreachable,
		)
	}

	return nil
}

func setupSSHTunnelToProxyAddr(sshCl *ssh.Client, proxyUrl *url.URL) (*frontend.Tunnel, error) {
	tunnel := strings.Join([]string{ProxyTunnelPort, proxyUrl.Hostname(), proxyUrl.Port()}, ":")
	tun := sshCl.Tunnel("L", tunnel)
	err := tun.Up()
	if err != nil {
		return nil, err
	}
	return tun, nil
}

func (pc *Checker) CheckRegistryCredentials() error {
	if app.PreflightSkipRegistryCredentials {
		log.InfoLn("Checking registry credentials was skipped")
		return nil
	}

	image := pc.installConfig.GetImage(false)
	log.DebugF("Image: %s\n", image)
	// skip for CE edition
	if image == "registry.deckhouse.ru/deckhouse/ce" {
		log.InfoLn("Checking registry credentials was skipped for CE edition")
		return nil
	}

	log.DebugLn("Checking registry credentials")
	ctx, cancel := context.WithTimeout(context.Background(), httpClientTimeoutSec*time.Second)
	defer cancel()

	authData, err := pc.metaConfig.Registry.Auth()
	if err != nil {
		return err
	}

	if authData == "" {
		return fmt.Errorf("%w, credentials are not specified. If you are using CE edition in a closed environment, this check can be skipped by specifying the --preflight-skip-registry-credential flag", ErrAuthFailed)
	}

	return checkRegistryAuth(ctx, pc.metaConfig, authData)
}

func prepareRegistryRequest(ctx context.Context, metaConfig *config.MetaConfig, authData string) (*http.Request, error) {
	registryURL := &url.URL{Scheme: metaConfig.Registry.Scheme, Host: metaConfig.Registry.Address, Path: registryPath}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("prepare registry request: %w", err)
	}
	if authData != "" {
		req.Header.Add("Authorization", "Basic "+authData)
	}

	return req, nil
}

func prepareAuthRequest(
	ctx context.Context,
	authURL string,
	registryService string,
	authData string,
	metaConfig *config.MetaConfig,
) (*http.Request, error) {
	authURLValues := url.Values{}
	authURLValues.Add("service", registryService)
	authURLValues.Add("scope", fmt.Sprintf("repository:%s:pull", strings.TrimLeft(metaConfig.Registry.Path, "/")))

	authURL = fmt.Sprintf("%s?%s", authURL, authURLValues.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("prepare auth request: %w", err)
	}

	req.Header.Add("Authorization", "Basic "+authData)

	return req, nil
}

func prepareAuthHTTPClient(metaConfig *config.MetaConfig) (*http.Client, error) {
	client := &http.Client{}
	httpTransport := http.DefaultTransport.(*http.Transport).Clone()

	if strings.ToLower(metaConfig.Registry.Scheme) == "http" {
		httpTransport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	if len(metaConfig.Registry.CA) == 0 {
		client.Transport = httpTransport
		return client, nil
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM([]byte(metaConfig.Registry.CA)); !ok {
		return nil, fmt.Errorf("invalid cert in CA PEM")
	}

	httpTransport.TLSClientConfig = &tls.Config{
		RootCAs: certPool,
	}

	client.Transport = httpTransport

	return client, nil
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
		return authURL, registryService, fmt.Errorf(
			"%w: expected Docker-Distribution-API-Version=registry/2.0 header in response from registry.\n"+
				"Check if container registry address is correct and if there is any reverse proxies that might be misconfigured",
			ErrAuthFailed,
		)
	}
	wwwAuthHeader := resp.Header.Get("WWW-Authenticate")

	if len(wwwAuthHeader) == 0 {
		return authURL, registryService, fmt.Errorf("WWW-Authenticate header not found. %w", ErrAuthFailed)
	}
	// Bearer realm="https://registry.local:5001/auth",service="Docker registry"
	log.DebugF("WWW-Authenticate: %s\n", wwwAuthHeader)

	// realm="(http[s]{0,1}:\/\/[a-z0-9\.\:\/\-]+)"
	realmMatches := realmRe.FindStringSubmatch(wwwAuthHeader)
	if len(realmMatches) == 0 {
		return authURL, registryService, fmt.Errorf("couldn't find bearer realm parameter, consider enabling bearer token auth in your registry, returned header:%s. %w", wwwAuthHeader, ErrAuthFailed)
	}
	authURL = realmMatches[1]

	// service="(.*?)"
	serviceMatches := serviceRe.FindStringSubmatch(wwwAuthHeader)
	if len(serviceMatches) > 0 {
		registryService = serviceMatches[1]
	}

	return authURL, registryService, nil
}

func checkResponseError(resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrAuthFailed
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status code %d, %w", resp.StatusCode, ErrAuthFailed)
	}

	return nil
}

func checkBasicRegistryAuth(
	ctx context.Context,
	metaConfig *config.MetaConfig,
	authData string,
	client *http.Client,
) error {
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
		return fmt.Errorf(
			"%w: expected Docker-Distribution-API-Version=registry/2.0 header in response from registry.\n"+
				"Check if container registry address is correct and if there is any reverse proxies that might be misconfigured",
			ErrAuthFailed,
		)
	}

	return checkResponseError(resp)
}

func checkTokenRegistryAuth(
	ctx context.Context,
	metaConfig *config.MetaConfig,
	authData string,
	client *http.Client,
) error {
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

	log.DebugF("Status Code: %d\n", resp.StatusCode)

	return checkResponseError(resp)
}

func checkRegistryAuth(ctx context.Context, metaConfig *config.MetaConfig, authData string) error {
	client, err := prepareAuthHTTPClient(metaConfig)
	if err != nil {
		return err
	}

	err = checkBasicRegistryAuth(ctx, metaConfig, authData, client)
	if err == nil {
		return nil
	}

	if !errors.Is(err, ErrAuthFailed) {
		return err
	}

	return checkTokenRegistryAuth(ctx, metaConfig, authData, client)
}
