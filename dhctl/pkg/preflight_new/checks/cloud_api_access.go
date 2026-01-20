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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new/checks/utils"
	cca "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new/checks/utils/check-cloud-api"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
)

var ErrCloudAPIUnreachable = errors.New("could not reach Cloud API from master node")

type CloudAPICheck struct {
	MetaConfig *config.MetaConfig
	Node       node.Interface
}

const CloudAPICheckName preflightnew.CheckName = "cloud-api-accessibility"

func (CloudAPICheck) Description() string {
	return "access to cloud api from master host"
}

func (CloudAPICheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePostInfra
}

func (CloudAPICheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.DefaultRetryPolicy
}

func (CloudAPICheck) Enabled() bool {
	return true
}

func (c CloudAPICheck) Run(ctx context.Context) error {
	if c.MetaConfig == nil {
		return nil
	}

	wrapper, ok := c.Node.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return nil
	}

	cloudAPIConfig, err := getCloudAPIConfig(c.MetaConfig)
	if err != nil {
		return err
	}
	if cloudAPIConfig == nil {
		return nil
	}

	proxyURL, noProxyAddresses, err := utils.GetProxyFromMetaConfig(c.MetaConfig)
	if err != nil {
		return fmt.Errorf("get proxy config: %w", err)
	}

	targetURL := proxyURL
	if targetURL == nil || utils.ShouldSkipProxyCheck(cloudAPIConfig.URL.Hostname(), noProxyAddresses) {
		targetURL = cloudAPIConfig.URL
		proxyURL = nil
	}

	tun, err := utils.SetupSSHTunnelToProxyAddr(wrapper.Client(), targetURL)
	if err != nil {
		return wrapTunnelErr(err)
	}
	defer tun.Stop()

	return c.check(ctx, cloudAPIConfig, proxyURL)
}

func (c CloudAPICheck) check(ctx context.Context, cloudAPIConfig *cca.CloudApiConfig, proxyURL *url.URL) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	resp, err := executeHTTPRequest(ctx, http.MethodGet, cloudAPIConfig, proxyURL)
	if err != nil {
		return ErrCloudAPIUnreachable
	}
	if resp.StatusCode >= 500 {
		return ErrCloudAPIUnreachable
	}
	return nil
}

const ProxyTunnelPort = "22323"

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

func executeHTTPRequest(ctx context.Context, method string, cloudAPIConfig *cca.CloudApiConfig, proxyUrl *url.URL) (*http.Response, error) {
	cloudAPIUrlString := cloudAPIConfig.URL.String()
	req, err := http.NewRequestWithContext(ctx, method, cloudAPIUrlString, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %w", err)
	}

	var client *http.Client
	if proxyUrl == nil {
		client, err = buildSSHTunnelHTTPClient(cloudAPIConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build HTTP client: %w", err)
		}
	} else {
		client = utils.BuildHTTPClientWithLocalhostProxy(proxyUrl)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	return resp, nil
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
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{
				Timeout:   20 * time.Second,
				KeepAlive: 20 * time.Second,
			}
			return d.DialContext(ctx, network, net.JoinHostPort("localhost", utils.ProxyTunnelPort))
		},
	}

	client := &http.Client{
		Transport: transport,
	}

	return client, nil
}

func getCloudAPIConfig(meta *config.MetaConfig) (*cca.CloudApiConfig, error) {
	if meta == nil {
		return nil, nil
	}

	configProvider, ok := cloudAPIConfigsProviders[meta.ProviderName]
	if !ok {
		return nil, nil
	}

	providerConfig, ok := meta.ProviderClusterConfig["provider"]
	if !ok || len(providerConfig) == 0 {
		return nil, fmt.Errorf("provider configuration not found in ProviderClusterConfig")
	}
	return configProvider(providerConfig)
}

var cloudAPIConfigsProviders = map[string]func(providerClusterConfig []byte) (*cca.CloudApiConfig, error){
	"openstack": cca.HandleOpenStackProvider,
	"vsphere":   cca.HandleVSphereProvider,
}

func wrapTunnelErr(err error) error {
	return fmt.Errorf(`cannot setup tunnel to control-plane host: %w.
Please check connectivity to control-plane host and that the sshd config parameters 'AllowTcpForwarding' is set to 'yes' and 'DisableForwarding' is set to 'no' on the control-plane node.`, err)
}

func CloudAPIAccess(meta *config.MetaConfig, nodeInterface node.Interface) preflightnew.Check {
	check := CloudAPICheck{
		MetaConfig: meta,
		Node:       nodeInterface,
	}
	return preflightnew.Check{
		Name:        CloudAPICheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Enabled:     check.Enabled,
		Run:         check.Run,
	}
}
