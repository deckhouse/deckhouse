/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package httpclient

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

var (
	defaultCAPath    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	defaultUserAgent = "system-registry-manager"
)

type ClientConfig struct {
	TLS        bool
	CAPath     string
	SkipVerify bool
	UserAgent  string
	Timeout    time.Duration
}

type Client struct {
	client    *http.Client
	token     *Token
	userAgent string
}

func NewDefaultHttpClient() (*Client, error) {
	config := ClientConfig{
		CAPath:     defaultCAPath,
		UserAgent:  defaultUserAgent,
		TLS:        true,
		SkipVerify: true,
		Timeout:    15 * time.Second,
	}
	return NewHttpClient(&config)
}

func NewHttpClient(config *ClientConfig) (*Client, error) {
	transport, err := NewHttpClientTransport(config)
	if err != nil {
		return nil, err
	}

	token, err := NewToken()
	if err != nil {
		return nil, err
	}

	return &Client{
		client: &http.Client{
			Timeout:   config.Timeout,
			Transport: transport,
		},
		token:     token,
		userAgent: config.UserAgent,
	}, nil
}

func (c *Client) SendJSON(url, method string, body any) ([]byte, error) {
	token, err := c.token.GetToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %v", err)
	}

	var reqBody []byte
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %v", err)
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("preparing request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading server response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return respBody, fmt.Errorf("unexpected response: status=%d, body=%q", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func NewHttpClientTransport(config *ClientConfig) (*http.Transport, error) {
	tr := newTransport(config.Timeout)
	var tlsConfig *tls.Config = nil

	if config.TLS {
		caCertPool := x509.NewCertPool()
		if config.CAPath != "" {
			caCertBytes, err := os.ReadFile(config.CAPath)
			if err != nil {
				return nil, fmt.Errorf("cannot read CA certificate from '%s': %v", config.CAPath, err)
			}
			if ok := caCertPool.AppendCertsFromPEM(caCertBytes); !ok {
				return nil, fmt.Errorf("failed to append CA certs from '%s'", config.CAPath)
			}
		}

		tlsConfig = &tls.Config{
			RootCAs:            caCertPool,
			InsecureSkipVerify: config.SkipVerify,
		}
	}

	tr.TLSClientConfig = tlsConfig
	return tr, nil
}

func newTransport(timeout time.Duration) *http.Transport {
	return &http.Transport{
		// Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: timeout / 2,
		}).DialContext,
		ForceAttemptHTTP2: true,
		// MaxIdleConns:          10,
		// IdleConnTimeout:       time.Minute,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
}
