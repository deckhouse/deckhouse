// Copyright 2026 Flant JSC
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

package helper

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/client-go/rest"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util/callback"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// APIServerConnection is the direct Kubernetes API endpoint an operation talks
// to instead of reaching the API over SSH. Deckhouse Commander sends it with
// every check and converge request: the URL is its AMPG tunnel to the managed
// cluster's kube-apiserver and the token belongs to the agent's service account.
type APIServerConnection struct {
	URL                      string
	Token                    string
	InsecureSkipTLSVerify    bool
	CertificateAuthorityData []byte
}

// Defined reports whether the endpoint can be used to reach the API server.
func (c *APIServerConnection) Defined() bool {
	return c != nil && c.URL != ""
}

// RestConfig renders the endpoint as a client-go config.
func (c *APIServerConnection) RestConfig() *rest.Config {
	if !c.Defined() {
		return nil
	}

	return &rest.Config{
		Host:        c.URL,
		BearerToken: c.Token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData:   c.CertificateAuthorityData,
			Insecure: c.InsecureSkipTLSVerify,
		},
	}
}

type CreateProvidersOptions struct {
	allowMissingHostsFromCache bool
	apiServer                  *APIServerConnection
}

type CreateProvidersOption func(*CreateProvidersOptions)

func AllowMissingHostsFromCache() CreateProvidersOption {
	return func(o *CreateProvidersOptions) {
		o.allowMissingHostsFromCache = true
	}
}

// WithAPIServer drives the Kubernetes connection over the given API endpoint.
// The kube provider then runs against that endpoint directly, so no SSH session
// is opened and no kubectl proxy is started on a master. A nil or empty endpoint
// leaves the SSH path in place.
func WithAPIServer(apiServer *APIServerConnection) CreateProvidersOption {
	return func(o *CreateProvidersOptions) {
		o.apiServer = apiServer
	}
}

func CreateProviders(ctx context.Context, config string, isDebug bool, tmpDir string, opts ...CreateProvidersOption) (*providerinitializer.SSHProviderInitializer, libcon.KubeProvider, func() error, error) {
	options := &CreateProvidersOptions{}
	for _, opt := range opts {
		opt(options)
	}

	cleanuper := callback.NewCallback()

	params := settings.ProviderParams{
		Logger:      dhlog.FromContext(ctx),
		IsDebug:     isDebug,
		NodeTmpPath: app.DeckhouseNodeTmpPath,
		NodeBinPath: app.DeckhouseNodeBinPath,
		TmpDir:      tmpDir,
	}

	providerOpts := []providerinitializer.ProviderOptions{providerinitializer.WithConnectionConfig(config)}
	if options.apiServer.Defined() {
		providerOpts = append(
			providerOpts,
			// Without this the initializer would parse the dhctl-server's own
			// os.Args as SSH flags when the request carries no connection config.
			providerinitializer.WithConnectionConfigOnly(),
			providerinitializer.WithKubeRestConfig(options.apiServer.RestConfig()),
		)
	}

	sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params, providerOpts...)
	if err != nil {
		if !options.allowMissingHostsFromCache || !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
			return nil, nil, nil, fmt.Errorf("initializing providers: %w", err)
		}
	}

	if sshProviderInitializer == nil && options.apiServer.Defined() {
		// Config must be non-nil: lib-connection dereferences it while cloning the connection.
		sshProviderInitializer = providerinitializer.NewSSHProviderInitializer(
			settings.NewBaseProviders(params),
			&sshconfig.ConnectionConfig{Config: &sshconfig.Config{}},
		)
	}

	if sshProviderInitializer != nil {
		cleanuper.Add(func() error {
			return sshProviderInitializer.Cleanup(ctx)
		})
	}

	return sshProviderInitializer, kubeProvider, cleanuper.AsFunc(), nil
}
