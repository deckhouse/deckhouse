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
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/pkg/log"
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
}

// ensureLogger sets a default logger if none is provided
func ensureLogger(logger *log.Logger) *log.Logger {
	if logger == nil {
		logger = log.NewLogger().Named("registry-client")
	}

	return logger
}

// buildRemoteOptions constructs remote options including auth and transport configuration
func buildRemoteOptions(auth authn.Authenticator, opts *Options) []remote.Option {
	remoteOptions := []remote.Option{
		remote.WithAuth(auth),
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
