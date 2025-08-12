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
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	cca "github.com/deckhouse/deckhouse/dhctl/pkg/preflight/check-cloud-api"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
)

var (
	ErrCloudApiUnreachable = errors.New("could not reach Cloud API from master node")
)

func (pc *Checker) CheckCloudAPIAccessibility(ctx context.Context) error {

	if app.PreflightSkipCloudAPIAccessibility {
		log.InfoLn("Checking  Cloud API is accessible from first master host was skipped (via skip flag)")
		return nil
	}

	log.DebugLn("Checking if Cloud API is accessible from first master host")
	wrapper, ok := pc.nodeInterface.(*ssh.NodeInterfaceWrapper)
	var tun node.Tunnel

	if !ok {
		log.InfoLn("Checking if Cloud API is accessible through proxy was skipped (local run)")
		return nil
	}

	proxyUrl, noProxyAddresses, err := getProxyFromMetaConfig(pc.metaConfig)
	if err != nil {
		return fmt.Errorf("get proxy config: %w", err)
	}

	cloudAPIConfig, err := getCloudApiConfigFromMetaConfig(pc.metaConfig)
	if err != nil {
		log.ErrorF("Cannot parse Cloud API Configuration: %v", err)
		return err
	}

	if cloudAPIConfig == nil {
		return nil
	}

	if proxyUrl == nil || shouldSkipProxyCheck(cloudAPIConfig.URL.Hostname(), noProxyAddresses) {
		proxyUrl = nil
		tun, err = setupSSHTunnelToProxyAddr(wrapper.Client(), cloudAPIConfig.URL)
	} else {
		tun, err = setupSSHTunnelToProxyAddr(wrapper.Client(), proxyUrl)
	}
	if err != nil {
		return fmt.Errorf(`cannot setup tunnel to control-plane host: %w.
Please check connectivity to control-plane host and that the sshd config parameters 'AllowTcpForwarding' is set to 'yes' and 'DisableForwarding' is set to 'no' on the control-plane node.`, err)
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	defer tun.Stop()

	resp, err := executeHTTPRequest(ctx, http.MethodGet, cloudAPIConfig, proxyUrl)

	if err != nil {
		log.ErrorF("Error while accessing Cloud API: %v", err)
		return ErrCloudApiUnreachable
	}
	log.DebugF("GET %s: %s\n", cloudAPIConfig.URL.String(), resp.Status)
	if resp.StatusCode >= 500 {
		return ErrCloudApiUnreachable
	}

	return nil
}

func buildSSHTunnelHTTPClient(cloudAPIConfig *cca.CloudApiConfig) (*http.Client, error) {

	tlsConfig := &tls.Config{
		ServerName: cloudAPIConfig.URL.Hostname(),
	}

	if cloudAPIConfig.Insecure {
		tlsConfig.InsecureSkipVerify = true
	}

	if len(cloudAPIConfig.CACert) > 0 {
		certPool := x509.NewCertPool()
		if ok := certPool.AppendCertsFromPEM([]byte(cloudAPIConfig.CACert)); !ok {
			return nil, fmt.Errorf("invalid cert in CA PEM")
		}
		tlsConfig.RootCAs = certPool
	}

	transport := &http.Transport{
		TLSClientConfig:   tlsConfig,
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			localhostAddr := net.JoinHostPort("localhost", ProxyTunnelPort)
			d := net.Dialer{
				Timeout:   20 * time.Second,
				KeepAlive: 20 * time.Second,
			}
			return d.DialContext(ctx, network, localhostAddr)
		},
	}

	client := &http.Client{
		Transport: transport,
	}

	return client, nil
}

func executeHTTPRequest(ctx context.Context, method string, cloudAPIConfig *cca.CloudApiConfig, proxyUrl *url.URL) (*http.Response, error) {

	cloudAPIUrlString := cloudAPIConfig.URL.String()

	var client *http.Client

	req, err := http.NewRequestWithContext(ctx, method, cloudAPIUrlString, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %w", err)
	}

	if proxyUrl == nil {
		client, err = buildSSHTunnelHTTPClient(cloudAPIConfig)
	} else {
		client = buildHTTPClientWithLocalhostProxy(proxyUrl)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP client: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	return resp, nil
}

func getCloudApiConfigFromMetaConfig(metaConfig *config.MetaConfig) (*cca.CloudApiConfig, error) {
	providerClusterConfig, exists := metaConfig.ProviderClusterConfig["provider"]
	if !exists {
		return nil, fmt.Errorf("provider configuration not found in ProviderClusterConfig")
	}

	var cloudApiConfig cca.CloudApiConfig
	var err error

	switch providerName := metaConfig.ProviderName; providerName {
	case "openstack":
		cloudApiConfig, err = cca.HandleOpenStackProvider(providerClusterConfig)
	case "vsphere":
		cloudApiConfig, err = cca.HandleVSphereProvider(providerClusterConfig)
	default:
		log.DebugF("[Skip] Checking if Cloud API is accessible from first master host. Unsupported provider: %v", providerName)
		return nil, nil
	}
	return &cloudApiConfig, err
}
