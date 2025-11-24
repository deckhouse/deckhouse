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

package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	defaultTimeout = 120 * time.Second

	tracerName = "container-registry-client"
)

// Options contains configuration options for the registry client
type Options struct {
	// Auth provides authentication for registry access (takes precedence over Username/Password/LicenseToken)
	Auth authn.Authenticator
	// Insecure allows connecting to registries over HTTP instead of HTTPS
	Insecure bool
	// TLSSkipVerify skips TLS certificate verification
	TLSSkipVerify bool
	// Logger for client operations
	Logger *log.Logger
	// UserAgent sets the User-Agent header for requests
	UserAgent string
	// CA sets a custom CA certificate for TLS verification
	CA string
	// Timeout sets the timeout for registry operations
	Timeout time.Duration
}

// ensureLogger sets a default logger if none is provided
func ensureLogger(logger *log.Logger) *log.Logger {
	if logger == nil {
		logger = log.NewLogger().Named("registry-client")
	}

	return logger
}

// buildRemoteOptions constructs remote options including auth and transport configuration
func buildRemoteOptions(opts *Options) []remote.Option {
	remoteOptions := []remote.Option{}

	if opts.Auth != nil {
		remoteOptions = append(remoteOptions, remote.WithAuth(opts.Auth))
	}

	if opts.UserAgent != "" {
		remoteOptions = append(remoteOptions, remote.WithUserAgent(opts.UserAgent))
	}

	if opts.CA != "" {
		remoteOptions = append(remoteOptions, remote.WithTransport(GetHTTPTransport(opts.CA)))
	}

	if needsCustomTransport(opts) {
		transport := configureTransport(opts)
		remoteOptions = append(remoteOptions, remote.WithTransport(transport))
	}

	return remoteOptions
}

// needsCustomTransport checks if custom transport configuration is required
func needsCustomTransport(opts *Options) bool {
	return opts.Insecure || opts.TLSSkipVerify
}

// configureTransport creates and configures an HTTP transport with TLS settings
func configureTransport(opts *Options) *http.Transport {
	transport := remote.DefaultTransport.(*http.Transport).Clone()

	if opts.TLSSkipVerify {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = true
	}

	return transport
}

func GetHTTPTransport(ca string) http.RoundTripper {
	if ca == "" {
		return http.DefaultTransport
	}
	caPool, err := x509.SystemCertPool()
	if err != nil {
		panic(fmt.Errorf("cannot get system cert pool: %v", err))
	}

	caPool.AppendCertsFromPEM([]byte(ca))

	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   defaultTimeout,
			KeepAlive: defaultTimeout,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{RootCAs: caPool},
		TLSNextProto:          make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}
}
