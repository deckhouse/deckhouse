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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh/frontend"
)

func (pc *Checker) CheckCloudAPIAccessibility() error {
	log.DebugLn("Checking if Cloud Api is accessible from first master host")
	wrapper, ok := pc.nodeInterface.(*ssh.NodeInterfaceWrapper)

	if !ok {
		log.InfoLn("Checking if Cloud Api is accessible through proxy was skipped (local run)")
		return nil
	}

	cloudApiUrl, err := getCloudApiURLFromMetaConfig(pc.metaConfig)

	if err != nil {
		log.ErrorF("cannot parse cloudApiUrl from CloudApiConfiguration: %v", err)
	}

	if cloudApiUrl == nil {
		log.DebugLn("[Skip] Checking if Cloud Api is accessible from first master host")
		return nil
	}

	tun, err := setupSSHTunnelToCloudApi(wrapper.Client(), cloudApiUrl)
	if err != nil {
		return fmt.Errorf(`cannot setup tunnel to control-plane host: %w.
Please check connectivity to control-plane host and that the sshd config parameter 'AllowTcpForwarding' set to 'yes' on control-plane node`, err)
	}
	defer tun.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, cloudApiUrl.String(), nil)
	httpCl := buildHTTPClientWithLocalhostProxy(cloudApiUrl)
	resp, err := httpCl.Do(req)

	if err != nil {
		log.ErrorF("Error making request: %v", err)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.ErrorF("Error reading response body: %v", err)
	}
	body := string(bodyBytes)

	fmt.Printf("status, response: %s %s\n", resp.Status, body)
	return nil
}

func getCloudApiURLFromMetaConfig(metaConfig *config.MetaConfig) (*url.URL, error) {
	providerClusterConfig, exists := metaConfig.ProviderClusterConfig["provider"]
	var cloudApiURLStr string
	var providerConfig map[string]string

	if !exists {
		return nil, fmt.Errorf("provider configuration not found in ProviderClusterConfig")
	}

	if err := json.Unmarshal(providerClusterConfig, &providerConfig); err != nil {
		return nil, fmt.Errorf("unable to unmarshal provider from ProviderClusterConfig: %v", err)
	}

	switch providerName := metaConfig.ProviderName; providerName {
	case "OpenStack":
		cloudApiURLStr = providerConfig["authURL"]
	case "vSphere":
		cloudApiURLStr = providerConfig["server"]
	default:
		return nil, nil
	}

	if cloudApiURLStr == "" {
		return nil, fmt.Errorf("cloud API URL is empty for provider: %s", metaConfig.ProviderName)
	}
	cloudApiURL, err := url.Parse(cloudApiURLStr)
	if err != nil {
		return nil, fmt.Errorf("invalid cloud API URL '%s': %v", cloudApiURLStr, err)
	}

	return cloudApiURL, nil
}

func setupSSHTunnelToCloudApi(sshCl *ssh.Client, cloudApiUrl *url.URL) (*frontend.Tunnel, error) {
	tunnel := strings.Join([]string{ProxyTunnelPort, cloudApiUrl.Hostname(), cloudApiUrl.Port()}, ":")
	tun := sshCl.Tunnel("L", tunnel)
	err := tun.Up()
	if err != nil {
		return nil, err
	}
	return tun, nil
}
