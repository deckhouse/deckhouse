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
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// defaultTimeout is used for network dial and keep-alive settings.
	defaultTimeout = 120 * time.Second

	// HTTP transport defaults for connection pooling and timeouts.
	defaultMaxIdleConns          = 100
	defaultIdleConnTimeout       = 90 * time.Second
	defaultTLSHandshakeTimeout   = 10 * time.Second
	defaultExpectContinueTimeout = 1 * time.Second
)

// Options contains configuration options for the registry client.
// Use With* functions to construct an Options value for NewClient,
// or fill the struct directly when passing to NewClientWithOptions.
type Options struct {
	// Auth provides authentication for registry access (takes precedence over Keychain)
	Auth authn.Authenticator
	// Keychain provides a custom keychain for authentication (alternative to Auth)
	Keychain authn.Keychain

	// UserAgent sets the User-Agent header for requests
	UserAgent string
	// Insecure allows connecting to registries over HTTP instead of HTTPS
	Insecure bool
	// TLSSkipVerify skips TLS certificate verification
	TLSSkipVerify bool
	// CA sets a custom CA certificate for TLS verification
	CA string

	// Scheme sets the URL scheme (http or https)
	// TODO: remove Scheme field in favor of Insecure field
	Scheme string

	// Timeout sets the timeout for registry operations
	Timeout time.Duration

	// Transport overrides the HTTP transport used for registry requests.
	// When set, CA, TLSSkipVerify, Insecure and ProxyURL transport settings are ignored.
	Transport http.RoundTripper

	// ProxyURL sets an explicit HTTP/HTTPS proxy for registry requests.
	// When nil, proxy settings are taken from the environment (HTTP_PROXY / HTTPS_PROXY).
	ProxyURL *url.URL

	// Middlewares are transport middlewares applied in order around the base
	// HTTP transport. Use WithMiddleware to add them via functional options.
	Middlewares []TransportMiddleware

	// Logger for client operations
	Logger *log.Logger
}

// Option is a functional option that configures an Options value.
type Option func(*Options)

// WithAuth sets an explicit authenticator (takes precedence over WithKeychain).
func WithAuth(auth authn.Authenticator) Option {
	return func(o *Options) { o.Auth = auth }
}

// WithKeychain sets a custom authentication keychain.
func WithKeychain(keychain authn.Keychain) Option {
	return func(o *Options) { o.Keychain = keychain }
}

// WithLoginPassword sets auth using a username and password.
// Equivalent to docker login -- credentials are sent as HTTP Basic auth.
func WithLoginPassword(username, password string) Option {
	return func(o *Options) {
		o.Auth = authn.FromConfig(authn.AuthConfig{
			Username: username,
			Password: password,
		})
	}
}

// WithDockercfg parses a Docker config JSON (raw or base64-encoded) and
// extracts credentials for the given registry repo.
// Returns an error if the config cannot be parsed or no matching entry is found.
func WithDockercfg(repo, dockercfg string) (Option, error) {
	auth, err := authFromDockerConfig(repo, dockercfg)
	if err != nil {
		return nil, fmt.Errorf("withDockercfg: %w", err)
	}
	return func(o *Options) { o.Auth = auth }, nil
}

// WithUserAgent sets the User-Agent header for requests.
func WithUserAgent(ua string) Option {
	return func(o *Options) { o.UserAgent = ua }
}

// WithInsecure enables plain HTTP instead of HTTPS.
func WithInsecure(insecure bool) Option {
	return func(o *Options) { o.Insecure = insecure }
}

// WithTLSSkipVerify disables TLS certificate verification.
func WithTLSSkipVerify(skip bool) Option {
	return func(o *Options) { o.TLSSkipVerify = skip }
}

// WithCA sets a custom PEM-encoded CA certificate for TLS verification.
func WithCA(ca string) Option {
	return func(o *Options) { o.CA = ca }
}

// WithScheme sets the URL scheme ("http" or "https").
//
// Deprecated: prefer WithInsecure.
func WithScheme(scheme string) Option {
	return func(o *Options) { o.Scheme = scheme }
}

// WithTimeout sets the timeout for registry operations.
func WithTimeout(d time.Duration) Option {
	return func(o *Options) { o.Timeout = d }
}

// WithLogger sets the logger used by the client.
func WithLogger(logger *log.Logger) Option {
	return func(o *Options) { o.Logger = logger }
}

// WithCustomTransport sets a custom HTTP transport for registry requests.
// When provided, it takes precedence over any transport built from CA,
// TLSSkipVerify, Insecure, or ProxyURL settings.
func WithCustomTransport(transport http.RoundTripper) Option {
	return func(o *Options) { o.Transport = transport }
}

// WithProxy sets an explicit proxy URL for registry requests.
// Overrides any proxy configured via environment variables.
// Pass nil to disable proxying entirely.
func WithProxy(proxyURL *url.URL) Option {
	return func(o *Options) { o.ProxyURL = proxyURL }
}

// resolveLogger returns the provided logger, or a default named logger when nil.
func resolveLogger(logger *log.Logger) *log.Logger {
	if logger == nil {
		logger = log.NewLogger().Named("registry-client")
	}

	return logger
}

// resolveTransport returns the base HTTP transport from options.
func resolveTransport(opts *Options) http.RoundTripper {
	var rt = http.DefaultTransport

	if opts.Transport != nil {
		rt = opts.Transport
	}

	if opts.CA != "" || needsCustomTransport(opts) {
		rt = buildTransport(opts)
	}

	// Apply transport middlewares in order (first middleware = outermost).
	for i := len(opts.Middlewares) - 1; i >= 0; i-- {
		rt = opts.Middlewares[i](rt)
	}

	return rt
}

// buildRemoteOptions constructs remote options including auth and transport configuration.
func buildRemoteOptions(opts *Options, logger *log.Logger, baseTransport http.RoundTripper) []remote.Option {
	remoteOptions := []remote.Option{}

	if opts.Auth != nil {
		remoteOptions = append(remoteOptions, remote.WithAuth(opts.Auth))
	}

	// If Auth is not set but Keychain is provided, use the Keychain for authentication
	// It is an error to use both WithAuth and WithAuthFromKeychain in the same Option set
	if opts.Auth == nil && opts.Keychain != nil {
		remoteOptions = append(remoteOptions, remote.WithAuthFromKeychain(opts.Keychain))
	}

	if opts.UserAgent != "" {
		remoteOptions = append(remoteOptions, remote.WithUserAgent(opts.UserAgent))
	}

	// Build transport configuration - use custom transport if provided,
	// otherwise combine CA and TLS settings into a single transport.
	if opts.Transport != nil {
		logger.Info("WithCustomTransport is set: Insecure option must be equal to the transport configuration")

		// Warn about options that are silently ignored when a custom transport is set.
		if opts.CA != "" {
			logger.Warn("WithCustomTransport is set: CA option will be ignored",
				slog.String("ca", opts.CA))
		}

		if opts.TLSSkipVerify {
			logger.Warn("WithCustomTransport is set: TLSSkipVerify option will be ignored")
		}

		remoteOptions = append(remoteOptions, remote.WithTransport(baseTransport))

		return remoteOptions
	}

	if baseTransport != nil && baseTransport != http.DefaultTransport {
		if opts.TLSSkipVerify {
			logger.Debug("TLS certificate verification disabled")
		}

		if opts.Insecure {
			logger.Debug("Insecure HTTP mode enabled")
		}

		remoteOptions = append(remoteOptions, remote.WithTransport(baseTransport))
	}

	return remoteOptions
}

// needsCustomTransport checks if custom transport configuration is required
func needsCustomTransport(opts *Options) bool {
	return opts.Insecure || opts.TLSSkipVerify || opts.ProxyURL != nil
}

// buildTransport creates a single transport that combines CA and TLS settings
func buildTransport(opts *Options) http.RoundTripper {
	if opts.CA != "" {
		// Start with CA transport as base
		transport := GetHTTPTransport(opts.CA).(*http.Transport).Clone()

		// Apply TLS skip verify if needed
		if opts.TLSSkipVerify {
			if transport.TLSClientConfig == nil {
				transport.TLSClientConfig = &tls.Config{}
			}

			transport.TLSClientConfig.InsecureSkipVerify = true
		}

		if opts.ProxyURL != nil {
			transport.Proxy = http.ProxyURL(opts.ProxyURL)
		}

		return transport
	}

	// No CA, use custom transport for TLS settings
	if needsCustomTransport(opts) {
		return configureTransport(opts)
	}

	// Default case - should not reach here due to caller check
	return http.DefaultTransport
}

// configureTransport creates and configures an HTTP transport with TLS settings
func configureTransport(opts *Options) *http.Transport {
	proxyFunc := http.ProxyFromEnvironment
	if opts.ProxyURL != nil {
		proxyFunc = http.ProxyURL(opts.ProxyURL)
	}

	transport := &http.Transport{
		Proxy: proxyFunc,
		DialContext: (&net.Dialer{
			Timeout:   defaultTimeout,
			KeepAlive: defaultTimeout,
		}).DialContext,
		MaxIdleConns:          defaultMaxIdleConns,
		IdleConnTimeout:       defaultIdleConnTimeout,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ExpectContinueTimeout: defaultExpectContinueTimeout,
		TLSClientConfig:       &tls.Config{},
	}

	if opts.TLSSkipVerify {
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
		MaxIdleConns:          defaultMaxIdleConns,
		IdleConnTimeout:       defaultIdleConnTimeout,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ExpectContinueTimeout: defaultExpectContinueTimeout,
		TLSClientConfig:       &tls.Config{RootCAs: caPool},
		TLSNextProto:          make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}
}
