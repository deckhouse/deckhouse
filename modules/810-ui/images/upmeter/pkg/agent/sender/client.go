/*
Copyright 2023 Flant JSC

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

package sender

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

type Client struct {
	url       string
	client    *http.Client
	UserAgent string
}

func getEndpoint(config *ClientConfig) string {
	schema := "https"
	if !config.TLS {
		schema = "http"
	}

	host := config.Host
	if config.Port != "" {
		host += ":" + config.Port
	}
	return fmt.Sprintf("%s://%s/downtime", schema, host)
}

type ClientConfig struct {
	Host      string
	Port      string
	CAPath    string
	TLS       bool
	UserAgent string
	Timeout   time.Duration
}

func NewClient(config *ClientConfig) *Client {
	return &Client{
		url:       getEndpoint(config),
		client:    NewHttpClient(config),
		UserAgent: config.UserAgent,
	}
}

func (c *Client) Send(reqBody []byte) error {
	req, err := http.NewRequest(http.MethodPost, c.url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("preparing POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reding server response body: %v", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected upmeter response: status=%d, body=%q", resp.StatusCode, string(body))
	}

	return nil
}

func NewHttpClient(config *ClientConfig) *http.Client {
	client, err := createSecureHttpClient(config.TLS, config.CAPath, config.Timeout)
	if err != nil {
		log.Errorf("falling back to default HTTP client: %v", err)
		return &http.Client{
			Timeout:   config.Timeout,
			Transport: newTransport(config.Timeout),
		}
	}
	return client
}

func createSecureHttpClient(useTLS bool, caPath string, timeout time.Duration) (*http.Client, error) {
	if !useTLS {
		return nil, fmt.Errorf("TLS is off by client")
	}

	tlsTransport, err := createSecureTransport(caPath, timeout)
	if err != nil {
		return nil, err
	}

	// Wrap tls transport to add Authorization header.
	bearerToken, err := getServiceAccountToken()
	if err != nil {
		return nil, err
	}

	// Create https client with checking CA certificate and Authorization header
	client := &http.Client{
		Transport: NewKubeBearerTransport(tlsTransport, bearerToken),
		Timeout:   timeout,
	}

	return client, nil
}

func createSecureTransport(caPath string, timeout time.Duration) (*http.Transport, error) {
	tr := newTransport(timeout)

	if caPath == "" {
		// Unsecure
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		return tr, nil
	}

	caCertBytes, err := ioutil.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read CA certificate from '%s': %v", caPath, err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertBytes)

	tr.TLSClientConfig = &tls.Config{RootCAs: caCertPool}

	return tr, nil
}

func defaultTransportDialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return dialer.DialContext
}

func newTransport(timeout time.Duration) *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: defaultTransportDialContext(&net.Dialer{
			Timeout:   timeout,
			KeepAlive: timeout / 2,
		}),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          1,                // we need only one connection to the server
		IdleConnTimeout:       time.Minute,      // double scrape interval which is 30s by design
		TLSHandshakeTimeout:   10 * time.Second, // 10s is the default value
		ResponseHeaderTimeout: timeout,
	}
}

func getServiceAccountToken() (string, error) {
	bs, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return "", fmt.Errorf("cannot read service account file: %v", err)
	}
	return string(bs), nil
}

func NewKubeBearerTransport(next http.RoundTripper, bearer string) *KubeBearerTransport {
	return &KubeBearerTransport{
		next:        next,
		bearerToken: bearer,
	}
}

type KubeBearerTransport struct {
	next        http.RoundTripper
	bearerToken string
}

func (t *KubeBearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", "Bearer "+t.bearerToken)
	return t.next.RoundTrip(req)
}
