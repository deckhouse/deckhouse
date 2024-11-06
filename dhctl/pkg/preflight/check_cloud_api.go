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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh/frontend"
)

var (
	ErrCloudApiUnreachable = errors.New("could not reach Cloud API from master node")
)

type OpenStackProvider struct {
	AuthURL    string `json:"authURL,omitempty" yaml:"authURL,omitempty"`
	CACert     string `json:"caCert,omitempty" yaml:"caCert,omitempty"`
	DomainName string `json:"domainName,omitempty" yaml:"domainName,omitempty"`
	TenantName string `json:"tenantName,omitempty" yaml:"tenantName,omitempty"`
	TenantID   string `json:"tenantID,omitempty" yaml:"tenantID,omitempty"`
	Username   string `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string `json:"password,omitempty" yaml:"password,omitempty"`
	Region     string `json:"region,omitempty" yaml:"region,omitempty"`
}

type VSphereProvider struct {
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
	Insecure bool   `json:"insecure"`
}

type CloudApiConfig struct {
	URL      *url.URL
	Insecure bool
	CACert   string
}

func (pc *Checker) CheckCloudAPIAccessibility() error {

	if app.PreflightSkipCloudAPIAccessibility {
		log.InfoLn("Checking  Cloud API is accessible from first master host was skipped (via skip flag)")
		return nil
	}

	log.DebugLn("Checking if Cloud API is accessible from first master host")
	wrapper, ok := pc.nodeInterface.(*ssh.NodeInterfaceWrapper)
	var tun *frontend.Tunnel

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
Please check connectivity to control-plane host and that the sshd config parameter 'AllowTcpForwarding' set to 'yes' on control-plane node`, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	defer tun.Stop()

	resp, err := executeHTTPRequest(ctx, http.MethodGet, cloudAPIConfig, proxyUrl)

	if err != nil {
		log.ErrorF("Error while accessing Cloud API: %v", err)
		return ErrCloudApiUnreachable
	}
	if resp.StatusCode >= 500 {
		return ErrCloudApiUnreachable
	}

	return nil
}

func buildSSHTunnelHTTPClient(cloudAPIConfig *CloudApiConfig) (*http.Client, error) {

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

func executeHTTPRequest(ctx context.Context, method string, cloudAPIConfig *CloudApiConfig, proxyUrl *url.URL) (*http.Response, error) {

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

	// Debug
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.ErrorF("Error reading response body: %v", err)
	}
	body := string(bodyBytes)
	statusCode := resp.StatusCode
	log.DebugF("status, response: %d %s", statusCode, body)
	// Debug

	return resp, nil
}

func getCloudApiConfigFromMetaConfig(metaConfig *config.MetaConfig) (*CloudApiConfig, error) {
	providerClusterConfig, exists := metaConfig.ProviderClusterConfig["provider"]
	if !exists {
		return nil, fmt.Errorf("provider configuration not found in ProviderClusterConfig")
	}

	var cloudAPIURLStr string
	var insecure bool
	var cacert string

	switch providerName := metaConfig.ProviderName; providerName {
	case "openstack":
		var openStackConfig OpenStackProvider
		if err := json.Unmarshal(providerClusterConfig, &openStackConfig); err != nil {
			return nil, fmt.Errorf("unable to unmarshal provider config for OpenStack: %v", err)
		}
		cloudAPIURLStr = openStackConfig.AuthURL
		cacert = openStackConfig.CACert

	case "vsphere":
		var vsphereConfig VSphereProvider
		if err := json.Unmarshal(providerClusterConfig, &vsphereConfig); err != nil {
			return nil, fmt.Errorf("unable to unmarshal provider config for vSphere: %v", err)
		}
		cloudAPIURLStr = vsphereConfig.Server
		insecure = vsphereConfig.Insecure

	default:
		log.DebugF("[Skip] Checking if Cloud API is accessible from first master host. Unsupported provider: %v", providerName)
		return nil, nil
	}

	if cloudAPIURLStr == "" {
		return nil, fmt.Errorf("cloud API URL is empty for provider: %s", metaConfig.ProviderName)
	}
	if !strings.Contains(cloudAPIURLStr, "://") {
		cloudAPIURLStr = "https://" + cloudAPIURLStr
	}

	cloudAPIURL, err := url.Parse(cloudAPIURLStr)

	if err != nil {
		return nil, fmt.Errorf("invalid cloud API URL '%s': %v", cloudAPIURLStr, err)
	}

	return &CloudApiConfig{
		URL:      cloudAPIURL,
		Insecure: insecure,
		CACert:   cacert,
	}, nil
}
