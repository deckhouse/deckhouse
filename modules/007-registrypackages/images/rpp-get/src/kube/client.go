/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kube

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultKubeconfigPath     = "/etc/kubernetes/kubelet.conf"
	defaultBootstrapTokenPath = "/var/lib/bashible/bootstrap-token"
	defaultBootstrapCAPath    = "/var/lib/bashible/ca.crt"

	requestTimeout = 10 * time.Second

	defaultRPPNamespace       = "d8-cloud-instance-manager"
	defaultRPPTokenSecretName = "registry-packages-proxy-token"
	defaultRPPLabelSelector   = "app=registry-packages-proxy"
	defaultRPPPort            = 4219
)

var (
	ErrNoConfig = errors.New("can't configure kube-api client: no kubelet.conf or bootstrap-token found")

	errEmptyBootstrapToken = errors.New("bootstrap-token file is empty")
	errNoEndpoints         = errors.New("no RPP endpoints found in kube")
	errNoToken             = errors.New("no RPP token found in kube")
)

type kubeHTTPError struct {
	url        string
	statusCode int
	body       string
}

func (e *kubeHTTPError) Error() string {
	return fmt.Sprintf("kube api %s returned HTTP %d: %s", e.url, e.statusCode, e.body)
}

type Client interface {
	GetRPPEndpoints(ctx context.Context) ([]string, error)
	GetRPPToken(ctx context.Context) (string, error)
}

var _ Client = (*apiClient)(nil)

type apiClient struct {
	baseURL string
	client  *http.Client
}

type podListResponse struct {
	Items []struct {
		Status struct {
			Phase string `json:"phase"`
			PodIP string `json:"podIP"`
		} `json:"status"`
	} `json:"items"`
}

type secretResponse struct {
	Data map[string]string `json:"data"`
}

func NewKubeletClient() (Client, error) {
	if _, err := os.Stat(defaultKubeconfigPath); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoConfig
		}
		return nil, fmt.Errorf("stat kubelet kubeconfig: %w", err)
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", defaultKubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("build kube client config: %w", err)
	}
	cfg.Timeout = requestTimeout

	return newAPIClient(cfg)
}

func NewBootstrapClient(endpoint string) (Client, error) {
	tokenBytes, err := os.ReadFile(defaultBootstrapTokenPath)
	if err != nil {
		return nil, fmt.Errorf("read bootstrap token: %w", err)
	}

	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return nil, errEmptyBootstrapToken
	}

	return newAPIClient(&rest.Config{
		Host:        endpoint,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			CAFile: defaultBootstrapCAPath,
		},
		Timeout: requestTimeout,
	})
}

func newAPIClient(cfg *rest.Config) (*apiClient, error) {
	httpClient, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, err
	}

	baseURL := strings.TrimSpace(cfg.Host)
	if baseURL == "" {
		return nil, ErrNoConfig
	}
	if !strings.HasPrefix(baseURL, "https://") && !strings.HasPrefix(baseURL, "http://") {
		baseURL = "https://" + baseURL
	}

	return &apiClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  httpClient,
	}, nil
}

func (c *apiClient) GetRPPEndpoints(ctx context.Context) ([]string, error) {
	var response podListResponse
	if err := c.getJSON(ctx, "/api/v1/namespaces/"+defaultRPPNamespace+"/pods", url.Values{
		"labelSelector": {defaultRPPLabelSelector},
	}, &response); err != nil {
		return nil, err
	}

	endpoints := make([]string, 0, len(response.Items))
	for _, pod := range response.Items {
		if pod.Status.Phase == "Running" && strings.TrimSpace(pod.Status.PodIP) != "" {
			endpoints = append(endpoints, net.JoinHostPort(pod.Status.PodIP, strconv.Itoa(defaultRPPPort)))
		}
	}

	if len(endpoints) == 0 {
		return nil, errNoEndpoints
	}

	return endpoints, nil
}

func (c *apiClient) GetRPPToken(ctx context.Context) (string, error) {
	var response secretResponse
	if err := c.getJSON(ctx, "/api/v1/namespaces/"+defaultRPPNamespace+"/secrets/"+defaultRPPTokenSecretName, nil, &response); err != nil {
		return "", err
	}

	encodedToken := strings.TrimSpace(response.Data["token"])
	if encodedToken == "" {
		return "", errNoToken
	}

	tokenBytes, err := base64.StdEncoding.DecodeString(encodedToken)
	if err != nil {
		return "", fmt.Errorf("decode token from secret %s/%s: %w", defaultRPPNamespace, defaultRPPTokenSecretName, err)
	}

	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return "", errNoToken
	}

	return token, nil
}

func (c *apiClient) getJSON(ctx context.Context, path string, query url.Values, dst any) error {
	requestURL := c.baseURL + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("build kube api request %s: %w", requestURL, err)
	}

	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("access kube api %s: %w", requestURL, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 255))
		return &kubeHTTPError{url: requestURL, statusCode: response.StatusCode, body: strings.TrimSpace(string(body))}
	}

	if err := json.NewDecoder(response.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode kube api response %s: %w", requestURL, err)
	}

	return nil
}

func ShouldRetry(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}

	return true
}
