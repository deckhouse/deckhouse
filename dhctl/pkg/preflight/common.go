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
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh/frontend"
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

func setupSSHTunnelToProxyAddr(sshCl *ssh.Client, proxyUrl *url.URL) (*frontend.Tunnel, error) {
	tunnel := strings.Join([]string{ProxyTunnelPort, proxyUrl.Hostname(), proxyUrl.Port()}, ":")
	tun := sshCl.Tunnel("L", tunnel)
	err := tun.Up()
	if err != nil {
		return nil, err
	}
	return tun, nil
}
