// Copyright 2025 Flant JSC
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
	"regexp"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new/checks/utils"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
)

type RegistryProxyCheck struct {
	MetaConfig *config.MetaConfig
	Node       node.Interface
}

var (
	ErrRegistryUnreachable = errors.New("Could not reach registry over proxy")
)

const (
	registryPath         = "/v2/"
	httpClientTimeoutSec = 20
)

var (
	realmRe   = regexp.MustCompile(`realm="(http[s]{0,1}:\/\/[a-z0-9\.\:\/\-]+)"`)
	serviceRe = regexp.MustCompile(`service="(.*?)"`)
)

const RegistryProxyCheckName preflightnew.CheckName = "registry-access-through-proxy"

func (RegistryProxyCheck) Description() string {
	return "registry access through proxy"
}

func (RegistryProxyCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePostInfra
}

func (RegistryProxyCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.DefaultRetryPolicy
}

func (RegistryProxyCheck) Enabled() bool {
	return true
}

func (c RegistryProxyCheck) Run(ctx context.Context) error {
	wrapper, ok := c.Node.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return nil
	}

	proxyURL, noProxy, err := utils.GetProxyFromMetaConfig(c.MetaConfig)
	if err != nil {
		return fmt.Errorf("get proxy config: %w", err)
	}
	if proxyURL == nil {
		return nil
	}

	registry := c.MetaConfig.Registry.Settings.RemoteData
	registryAddress, _ := registry.AddressAndPath()
	if utils.ShouldSkipProxyCheck(registryAddress, noProxy) {
		return nil
	}

	tun, err := utils.SetupSSHTunnelToProxyAddr(wrapper.Client(), proxyURL)
	if err != nil {
		return fmt.Errorf(`Cannot setup tunnel to control-plane host: %w.
Please check connectivity to control-plane host and that the sshd config parameters 'AllowTcpForwarding' is set to 'yes' and 'DisableForwarding' is set to 'no' on the control-plane node.`, err)
	}
	defer tun.Stop()

	registryURL := &url.URL{
		Scheme: strings.ToLower(string(registry.Scheme)),
		Host:   registryAddress,
		Path:   registryPath,
	}

	ctx, cancel := context.WithTimeout(ctx, httpClientTimeoutSec*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL.String(), nil)
	if err != nil {
		return fmt.Errorf("prepare request: %w", err)
	}

	httpCl := utils.BuildHTTPClientWithLocalhostProxy(proxyURL)
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

func checkResponseIsFromDockerRegistry(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf(
			"%w: got %d status code from the container registry API, this is not a valid registry API response.\n"+
				"Check if container registry address is correct and if there is any reverse proxies that might be misconfigured.",
			ErrRegistryUnreachable,
			resp.StatusCode,
		)
	}

	if resp.Header.Get("Docker-Distribution-API-Version") != "registry/2.0" {
		return fmt.Errorf(
			"%w: expected Docker-Distribution-API-Version=registry/2.0 header in response from registry.\n"+
				"Check if container registry address is correct and if there is any reverse proxies that might be misconfigured",
			ErrRegistryUnreachable,
		)
	}

	return nil
}

func RegistryProxy(meta *config.MetaConfig, nodeInterface node.Interface) preflightnew.Check {
	check := RegistryProxyCheck{MetaConfig: meta, Node: nodeInterface}
	return preflightnew.Check{
		Name:        RegistryProxyCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Enabled:     check.Enabled,
		Run:         check.Run,
	}
}
