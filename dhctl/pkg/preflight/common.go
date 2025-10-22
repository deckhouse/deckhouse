// Copyright 2024 Flant JSC
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
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
)

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

func setupSSHTunnelToProxyAddr(sshCl node.SSHClient, proxyUrl *url.URL) (node.Tunnel, error) {
	port := proxyUrl.Port()
	if port == "" {
		switch proxyUrl.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	var tunnel string
	if sshclient.IsLegacyMode() {
		tunnel = strings.Join([]string{ProxyTunnelPort, proxyUrl.Hostname(), port}, ":")
	} else {
		tunnel = strings.Join([]string{proxyUrl.Hostname(), port, "127.0.0.1", ProxyTunnelPort}, ":")
	}
	log.DebugF("tunnel string: %s", tunnel)
	tun := sshCl.Tunnel(tunnel)
	err := tun.Up()
	if err != nil {
		return nil, err
	}
	return tun, nil
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
		return nil, nil, fmt.Errorf(
			`%w: malformed proxy address: "%v"`,
			ErrBadProxyConfig,
			proxyAddr,
		)
	}

	proxyUrl, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, nil, fmt.Errorf(`%s: %w`, ErrBadProxyConfig, err)
	}

	return proxyUrl, noProxyAddresses, nil
}

func shouldSkipProxyCheck(serviceAddress string, noProxyAddresses []string) bool {
	for _, noProxyAddress := range noProxyAddresses {
		if serviceAddress == noProxyAddress {
			return true
		}

		registryIPAddr, _ := net.ResolveIPAddr("ip", serviceAddress)
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
