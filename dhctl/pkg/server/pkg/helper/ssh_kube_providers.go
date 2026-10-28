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
	"strings"

	libcon "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/kube"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/settings"
	sshconfig "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/config"
	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util"
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

// RestConfig renders the endpoint as a client-go config, the way dhctl built it
// up to v1.76 in helper/kube_client.go.
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
	allowMissingHostsFromCache   bool
	allowMissingConnectionConfig bool
	kubeConfig                   string
	apiServer                    *APIServerConnection
}

type CreateProvidersOption func(*CreateProvidersOptions)

func AllowMissingHostsFromCache() CreateProvidersOption {
	return func(o *CreateProvidersOptions) {
		o.allowMissingHostsFromCache = true
	}
}

func WithKubeConfig(kubeConfig string) CreateProvidersOption {
	return func(o *CreateProvidersOptions) {
		o.kubeConfig = kubeConfig
	}
}

// WithAPIServer drives the Kubernetes connection over the given API endpoint.
// The kube provider then runs against it directly, so no SSH session is opened
// and no kubectl proxy is started on a master. A nil or empty endpoint, or a
// request that also carries a kubeconfig, leaves the endpoint unused.
func WithAPIServer(apiServer *APIServerConnection) CreateProvidersOption {
	return func(o *CreateProvidersOptions) {
		o.apiServer = apiServer
	}
}

// AllowMissingConnectionConfig hands out a hostless, keyless initializer instead of nil when
// the request carries no connection config. Bootstrap is written against a present initializer:
// cluster-bootstrapper.go takes its settings and config from it, and an immutable cluster is
// installed without ssh at all, so the dhctl command line hands it a hostless initializer too.
func AllowMissingConnectionConfig() CreateProvidersOption {
	return func(o *CreateProvidersOptions) {
		o.allowMissingConnectionConfig = true
	}
}

// SSHProviderOrNil returns nil, not the provider GetSSHProvider hands back alongside the sentinel:
// initializer.go pairs the error with a hostless provider that passes every govalue.IsNil guard
// downstream and only fails deep inside lib-connection once something dials it. A nil initializer
// is safe to call here - its methods guard the nil receiver in initializer.go.
func SSHProviderOrNil(ctx context.Context, initializer *providerinitializer.SSHProviderInitializer, kubeConfig string) (libcon.SSHProvider, error) {
	sshProvider, err := initializer.GetSSHProvider(ctx)
	if err == nil {
		return sshProvider, nil
	}

	if kubeConfig != "" && errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
		return nil, nil
	}

	return nil, err
}

func CreateProviders(ctx context.Context, config string, isDebug bool, tmpDir string, opts ...CreateProvidersOption) (*providerinitializer.SSHProviderInitializer, libcon.KubeProvider, func() error, error) {
	options := &CreateProvidersOptions{}
	for _, opt := range opts {
		opt(options)
	}

	cleanuper := callback.NewCallback()

	params := app.NewProviderParams(dhlog.FromContext(ctx), isDebug, tmpDir)

	ssh, err := connectionConfig(config, params, options.allowMissingConnectionConfig)
	if err != nil {
		return nil, nil, cleanuper.AsFunc(), fmt.Errorf("connection config: %w", err)
	}

	kubeCfg, err := kubeConfig(options, cleanuper)
	if err != nil {
		return nil, nil, cleanuper.AsFunc(), fmt.Errorf("kube config: %w", err)
	}

	sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params, kubeCfg, providerinitializer.GetProvidersOptions{SSH: ssh})
	if err != nil {
		if !options.allowMissingHostsFromCache || !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
			return nil, nil, cleanuper.AsFunc(), fmt.Errorf("initializing providers: %w", err)
		}
	}

	if sshProviderInitializer != nil {
		cleanuper.Add(func() error {
			return sshProviderInitializer.Cleanup(ctx)
		})
	}

	return sshProviderInitializer, kubeProvider, cleanuper.AsFunc(), nil
}

// connectionConfig is the SSH connection a request carries. Without one the operation runs without
// SSH, or with a hostless, keyless connection when it is written against a present one.
func connectionConfig(config string, params settings.ProviderParams, allowMissing bool) (*sshconfig.ConnectionConfig, error) {
	if config != "" {
		return sshconfig.ParseConnectionConfig(strings.NewReader(config), settings.NewBaseProviders(params), sshconfig.ParseWithRequiredSSHHost(false))
	}

	if !allowMissing {
		return nil, nil
	}
	// Config must be non-nil: lib-connection dereferences it while cloning the connection.
	return &sshconfig.ConnectionConfig{Config: &sshconfig.Config{}}, nil
}

// kubeConfig is the Kubernetes route of a request: its kubeconfig, written to a temp file, else its
// API server endpoint, else over SSH. The endpoint is the older of the two direct routes, so a
// kubeconfig wins wherever a request carries both.
func kubeConfig(options *CreateProvidersOptions, cleanuper *callback.Callback) (*kube.Config, error) {
	if options.kubeConfig == "" {
		if options.apiServer.Defined() {
			return &kube.Config{RestConfig: options.apiServer.RestConfig()}, nil
		}

		return &kube.Config{}, nil
	}

	kubeConfigPath, cleanup, err := util.WriteDefaultTempFile([]byte(options.kubeConfig))
	cleanuper.Add(cleanup)
	if err != nil {
		return nil, fmt.Errorf("writing kubeconfig: %w", err)
	}

	// The Commander generates the kubeconfig it sends, so its current-context is the only one that
	// can be meant.
	return &kube.Config{KubeConfig: kubeConfigPath}, nil
}
