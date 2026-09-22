/*
Copyright 2021 Flant JSC

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

package http

//go:generate minimock -i Client -o http_mock.go

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

// Client interface
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewClient(options ...Option) Client {
	opts := &httpOptions{
		timeout: 10 * time.Second,
	}

	for _, opt := range options {
		opt(opts)
	}

	dialer := &net.Dialer{
		Timeout: opts.timeout,
	}
	dialContext := dialer.DialContext
	proxy := http.ProxyFromEnvironment
	if opts.restrictedNetworkAccess {
		dialContext = restrictedDialContext(dialer)
		proxy = nil
	}

	tlsConf := &tls.Config{
		InsecureSkipVerify: opts.insecure,
	}

	if !opts.insecure {
		caPool, err := x509.SystemCertPool()
		if err != nil {
			panic(fmt.Errorf("cannot get system cert pool: %v", err))
		}

		for _, ca := range opts.additionalTLSCA {
			caPool.AppendCertsFromPEM(ca)
		}

		tlsConf.RootCAs = caPool
	}

	if opts.tlsServerName != "" {
		tlsConf.ServerName = opts.tlsServerName
	}

	tr := &http.Transport{
		Proxy:                 proxy,
		TLSClientConfig:       tlsConf,
		IdleConnTimeout:       5 * time.Minute,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext:           dialContext,
	}

	otr := otelhttp.NewTransport(
		tr,
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return fmt.Sprintf("HTTP %s %s", operation, r.URL.String())
		}),
		otelhttp.WithFilter(func(request *http.Request) bool {
			span := trace.SpanFromContext(request.Context())
			return span.IsRecording()
		}),
	)

	client := &http.Client{
		Timeout:   opts.timeout,
		Transport: otr,
	}
	if opts.restrictedNetworkAccess {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return fmt.Errorf("redirects are disabled for restricted HTTP clients")
		}
	}

	return client
}

type httpOptions struct {
	timeout                 time.Duration
	insecure                bool
	additionalTLSCA         [][]byte
	tlsServerName           string
	restrictedNetworkAccess bool
}

type Option func(options *httpOptions)

// WithTimeout set custom timeout for http request. Default: 10 seconds
func WithTimeout(t time.Duration) Option {
	return func(options *httpOptions) {
		options.timeout = t
	}
}

// WithInsecureSkipVerify skip tls certificate validation
func WithInsecureSkipVerify() Option {
	return func(options *httpOptions) {
		options.insecure = true
	}
}

func WithAdditionalCACerts(certs [][]byte) Option {
	return func(options *httpOptions) {
		options.additionalTLSCA = append(options.additionalTLSCA, certs...)
	}
}

func WithTLSServerName(name string) Option {
	return func(options *httpOptions) {
		options.tlsServerName = name
	}
}

// WithRestrictedNetworkAccess prevents requests to loopback, link-local, unspecified and multicast
// addresses and disables redirects. RFC1918, unique-local IPv6 and CGNAT destinations stay allowed
// so federation/multicluster can use private networks. HTTP proxies are disabled so the destination
// is validated and dialed directly.
func WithRestrictedNetworkAccess() Option {
	return func(options *httpOptions) {
		options.restrictedNetworkAccess = true
	}
}

func restrictedDialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("parse destination address %q: %w", address, err)
		}

		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve destination host %q: %w", host, err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("destination host %q has no IP addresses", host)
		}

		for _, ip := range ips {
			if isRestrictedIP(ip) {
				return nil, fmt.Errorf("destination host %q resolves to a restricted IP address %s", host, ip)
			}
		}

		var dialErr error
		for _, ip := range ips {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			dialErr = err
		}

		return nil, fmt.Errorf("dial destination host %q: %w", host, dialErr)
	}
}

func isRestrictedIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

func SetBearerToken(req *http.Request, token string) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
}

func GetKubeToken() (string, error) {
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") == "true" {
		return "kube-auth-test-token", nil
	}

	content, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	return string(content), nil
}

func SetKubeAuthToken(req *http.Request) error {
	token, err := GetKubeToken()
	if err != nil {
		return err
	}

	SetBearerToken(req, token)

	return nil
}
