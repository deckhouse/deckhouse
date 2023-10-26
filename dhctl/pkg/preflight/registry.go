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
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"k8s.io/utils/strings/slices"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/frontend"
)

var (
	ErrBadProxyConfig      = errors.New("Bad proxy config")
	ErrRegistryUnreachable = errors.New("Could not reach registry over proxy")
)

const ProxyTunnelPort = "22323"

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
	if slices.Contains(noProxyAddresses, pc.metaConfig.Registry.Address) {
		log.DebugLn("Registry address found in proxy.noProxy list, skipping check")
		return nil
	}

	tun, err := setupSSHTunnelToProxyAddr(pc.sshClient, proxyUrl)
	if err != nil {
		return fmt.Errorf(`Cannot setup tunnel to control-plane host: %w.
Please check connectivity to control-plane host or
check that sshd config 'AllowTcpForwarding' set to 'yes' on control-plane node`, err)
	}
	defer tun.Stop()

	registryURL := &url.URL{Scheme: pc.metaConfig.Registry.Scheme, Host: pc.metaConfig.Registry.Address, Path: "/v2"}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL.String(), nil)
	if err != nil {
		return fmt.Errorf("prepare request: %w", err)
	}

	httpCl := buildHTTPClientWithLocalhostProxy(proxyUrl)
	resp, err := httpCl.Do(req)
	if err != nil {
		return fmt.Errorf(`Container registry api connectivity check was failed with error: %w.
Please chech connectivity from control-plane node to proxy an from proxy to container registry.`, err)
	}

	if err = checkResponseIsFromDockerRegistry(resp); err != nil {
		return err
	}

	return nil
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
		return fmt.Errorf("%w: got %d status code", ErrRegistryUnreachable, resp.StatusCode)
	}

	// https://docs.docker.com/registry/spec/api/#api-version-check
	if resp.Header.Get("Docker-Distribution-API-Version") != "registry/2.0" {
		return fmt.Errorf("%w: expected Docker-Distribution-API-Version=registry/2.0 header", ErrRegistryUnreachable)
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
