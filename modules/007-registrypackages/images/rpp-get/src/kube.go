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

package main

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

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type kubeAPIClient struct {
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

// newKubeClient detects available credentials and builds the appropriate API client
// For bootstrap mode with multiple endpoints, the endpoint is selected round-robin based on attempt
func newKubeClient(apiServerEndpoints []string, attempt int) (*kubeAPIClient, error) {
	// kubelet.conf
	exists, err := fileExists(defaultKubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("stat kubelet kubeconfig: %w", err)
	}
	if exists {
		return newKubeletAPIClient()
	}
	// /var/lib/bashible/bootstrap-token
	exists, err = fileExists(defaultBootstrapTokenPath)
	if err != nil {
		return nil, fmt.Errorf("stat bootstrap token: %w", err)
	}
	if !exists {
		return nil, errNoKubeAPIConfig
	}

	if len(apiServerEndpoints) == 0 {
		return nil, errNoBootstrapAPIServerEndpoints
	}

	endpoint := apiServerEndpoints[(attempt-1)%len(apiServerEndpoints)]
	return newBootstrapAPIClient(endpoint)
}

func newKubeletAPIClient() (*kubeAPIClient, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", defaultKubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("build kube client config: %w", err)
	}
	cfg.Timeout = kubeRequestTimeout

	return newKubeAPIClient(cfg)
}

func newBootstrapAPIClient(endpoint string) (*kubeAPIClient, error) {
	tokenBytes, err := os.ReadFile(defaultBootstrapTokenPath)
	if err != nil {
		return nil, fmt.Errorf("read bootstrap token: %w", err)
	}

	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return nil, errEmptyBootstrapToken
	}

	return newKubeAPIClient(&rest.Config{
		Host:        endpoint,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			CAFile: defaultBootstrapCAPath,
		},
		Timeout: kubeRequestTimeout,
	})
}

func newKubeAPIClient(cfg *rest.Config) (*kubeAPIClient, error) {
	httpClient, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, err
	}

	baseURL := strings.TrimSpace(cfg.Host)
	if baseURL == "" {
		return nil, errNoKubeAPIConfig
	}
	if !strings.HasPrefix(baseURL, "https://") && !strings.HasPrefix(baseURL, "http://") {
		baseURL = "https://" + baseURL
	}

	return &kubeAPIClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  httpClient,
	}, nil
}

func (c *kubeAPIClient) GetEndpoints(ctx context.Context) ([]string, error) {
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

func (c *kubeAPIClient) GetToken(ctx context.Context) (string, error) {
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

func (c *kubeAPIClient) getJSON(ctx context.Context, path string, query url.Values, dst any) error {
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
		return fmt.Errorf("kube api %s returned HTTP %d: %s", requestURL, response.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(response.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode kube api response %s: %w", requestURL, err)
	}

	return nil
}

func shouldRetryKube(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) ||
		errors.Is(err, errNoBootstrapAPIServerEndpoints) {
		return false
	}

	return true
}
