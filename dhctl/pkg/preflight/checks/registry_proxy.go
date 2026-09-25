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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/deckhouse/lib-connection/pkg/ssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/utils"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

type RegistryProxyCheck struct {
	MetaConfig *config.MetaConfig
	// NodeInterface resolves the connection at the moment the check runs — see NodeInterfaceFunc.
	// It is a function rather than the initializer itself so the tunnel this check opens can be
	// stood up locally in tests; production passes nodeInterfaceResolverFor(initializer).
	NodeInterface NodeInterfaceFunc
	// LegacyMode reflects whether the SSH client used here runs the legacy
	// clissh backend (true) or modern gossh (false). Set at suite
	// construction from sshclient.Config.IsLegacyMode().
	LegacyMode bool
}

var ErrRegistryUnreachable = errors.New("cannot reach the registry through the proxy")

const (
	registryPath         = "/v2/"
	httpClientTimeoutSec = 20
)

const RegistryProxyCheckName preflight.CheckName = "registry-access-through-proxy"

func (RegistryProxyCheck) Description() string {
	return "the container registry is reachable from the node through the proxy"
}

func (RegistryProxyCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (RegistryProxyCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NetworkRetry
}

func (c RegistryProxyCheck) Run(ctx context.Context) (string, error) {
	if c.MetaConfig == nil {
		return "", errors.New("dhctl was given no cluster configuration")
	}

	proxyURL, noProxy, err := utils.GetProxyFromMetaConfig(c.MetaConfig)
	if err != nil {
		return "", fmt.Errorf("reading ClusterConfiguration.proxy: %w", err)
	}
	if proxyURL == nil {
		return "", preflight.NotApplicable("no proxy is configured in ClusterConfiguration.proxy")
	}

	registry := c.MetaConfig.Registry.Settings.RemoteData
	registryAddress, _ := registry.AddressAndPath()
	registryURL := &url.URL{
		Scheme: strings.ToLower(string(registry.Scheme)),
		Host:   registryAddress,
		Path:   registryPath,
	}

	if utils.ShouldSkipProxyCheck(registryURL, noProxy) {
		return "", preflight.NotApplicable("%s is exempt from the proxy by ClusterConfiguration.proxy.noProxy", registryAddress)
	}

	if c.NodeInterface == nil {
		return "", errors.New("dhctl was given no SSH connection to the node")
	}

	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	wrapper, ok := nodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return "", preflight.NotApplicable("dhctl was given no SSH host to make the request from")
	}

	tun, err := utils.SetupSSHTunnelToProxyAddr(ctx, wrapper.Client(), proxyURL, c.LegacyMode)
	if err != nil {
		return "", tunnelFailure(wrapper.Client(), proxyURL, err)
	}
	defer tun.Stop()

	ctx, cancel := context.WithTimeout(ctx, httpClientTimeoutSec*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("building the request to %s: %w", registryURL, err)
	}

	httpCl, err := utils.BuildHTTPClientWithLocalhostProxy(proxyURL, utils.TLSOptions{
		ServerName: registryURL.Hostname(),
		CACert:     registry.CA,
	})
	if err != nil {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  registryCAField(c.registryMode()),
			Observed: err.Error(),
			Expected: "a valid PEM certificate bundle",
			Fix:      "correct " + registryCAField(c.registryMode()),
		})
	}

	resp, err := httpCl.Do(req)
	if err != nil {
		// A proxy that refuses the CONNECT answers with a status, not with a connection, and
		// that status is the whole diagnosis — it must not be reported as "the registry did
		// not answer".
		if refusal := proxyRefusal(proxyConnectStatus(err), proxyURL, registryURL); refusal != nil {
			return "", refusal
		}

		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("GET %s from %s via proxy %s", registryURL, hostLabelOfClient(wrapper.Client()), proxyURL.Redacted()),
			Observed: classifyNetworkError(err),
			Expected: "an answer from the registry API",
			Fix: fmt.Sprintf(
				"check that the node reaches %s and that the proxy reaches %s, "+
					"or add %s to ClusterConfiguration.proxy.noProxy",
				proxyURL.Redacted(), registryAddress, registryURL.Hostname(),
			),
			Err: err,
		}
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp, registryURL, proxyURL); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s answers from %s via proxy %s",
		registryAddress, hostLabelOfClient(wrapper.Client()), proxyURL.Redacted()), nil
}

// checkResponse tells apart the three things that can come back on this path: the proxy refusing
// the request, something that is not a registry answering at the address, and the registry.
func (c RegistryProxyCheck) checkResponse(resp *http.Response, registryURL, proxyURL *url.URL) error {
	switch {
	case resp.StatusCode == http.StatusProxyAuthRequired, resp.StatusCode == http.StatusForbidden:
		// The same two answers the CONNECT path produces, on the plain-http path where they
		// arrive as a response instead. One wording for both.
		return proxyRefusal(resp.StatusCode, proxyURL, registryURL)
	case resp.StatusCode >= 500:
		return &preflight.Failure{
			Checked:  fmt.Sprintf("GET %s via proxy %s", registryURL, proxyURL.Redacted()),
			Observed: fmt.Sprintf("%s answered with HTTP %d", registryURL.Host, resp.StatusCode),
			Expected: "HTTP 200 or 401 from the registry API",
			Fix:      fmt.Sprintf("check that the proxy reaches %s and that the registry is up", registryURL.Host),
		}
	}

	return checkResponseIsFromDockerRegistry(resp, registryURL.Host, c.registryMode())
}

// host is passed in rather than read off resp.Request: a response built by hand — every caller
// in a test — carries no request, and the message must not depend on that.
func checkResponseIsFromDockerRegistry(resp *http.Response, host, mode string) error {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf(
			"%w: %s answered with HTTP %d, which is not how a registry answers /v2/. "+
				"Check %s, and any reverse proxy in front of the registry.",
			ErrRegistryUnreachable,
			host,
			resp.StatusCode,
			registryImagesRepoField(mode),
		)
	}

	if resp.Header.Get("Docker-Distribution-API-Version") != "registry/2.0" {
		return fmt.Errorf(
			"%w: the answer carries no Docker-Distribution-API-Version: registry/2.0 header. "+
				"Check %s, and that no reverse proxy strips the header.",
			ErrRegistryUnreachable,
			registryImagesRepoField(mode),
		)
	}

	return nil
}

func RegistryProxy(meta *config.MetaConfig, sshProviderInitializer *providerinitializer.SSHProviderInitializer, legacyMode bool) preflight.Check {
	check := RegistryProxyCheck{
		MetaConfig:    meta,
		NodeInterface: nodeInterfaceResolverFor(sshProviderInitializer),
		LegacyMode:    legacyMode,
	}
	return preflight.Check{
		Name:        RegistryProxyCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}

// registryMode is the mode the registry is configured in; see registrySection.
func (c RegistryProxyCheck) registryMode() string {
	if c.MetaConfig == nil {
		return ""
	}
	return string(c.MetaConfig.Registry.Settings.Mode)
}
