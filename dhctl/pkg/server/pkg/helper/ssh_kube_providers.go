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

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/settings"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util/callback"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

type CreateProvidersOptions struct {
	allowMissingHostsFromCache bool
	kubeConfig                 string
}

type CreateProvidersOption func(*CreateProvidersOptions)

func AllowMissingHostsFromCache() CreateProvidersOption {
	return func(o *CreateProvidersOptions) {
		o.allowMissingHostsFromCache = true
	}
}

// WithKubeConfig points the kube provider at an already existing API server through the
// kubeconfig at the given path, the way the --kubeconfig flag does on the CLI. An empty path
// keeps the previous behaviour: connect over SSH.
func WithKubeConfig(kubeConfig string) CreateProvidersOption {
	return func(o *CreateProvidersOptions) {
		o.kubeConfig = kubeConfig
	}
}

// SSHProviderOrNil resolves the SSH provider an operation should run with. A request that carries a
// kubeconfig drives the cluster through the API server, so it may legitimately have no master hosts
// to connect to; there the provider is nil and every consumer has to guard on it. Without a
// kubeconfig, missing hosts are a real misconfiguration and stay an error.
//
// The nil matters: GetSSHProvider hands back a hostless provider alongside the sentinel, and that
// value looks connectable to every govalue.IsNil guard downstream while failing deep inside
// lib-connection once something dials it.
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

	params := settings.ProviderParams{
		Logger:      dhlog.FromContext(ctx),
		IsDebug:     isDebug,
		NodeTmpPath: app.DeckhouseNodeTmpPath,
		NodeBinPath: app.DeckhouseNodeBinPath,
		TmpDir:      tmpDir,
	}

	providerOpts := []providerinitializer.ProviderOptions{providerinitializer.WithConnectionConfig(config)}
	if options.kubeConfig != "" {
		providerOpts = append(providerOpts,
			providerinitializer.WithKubeFlagsDefined(true),
			providerinitializer.WithKubeConfig(options.kubeConfig, "", false),
		)
	}

	sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params, providerOpts...)
	if err != nil {
		if !options.allowMissingHostsFromCache || !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
			return nil, nil, nil, fmt.Errorf("initializing providers: %w", err)
		}
	}
	if sshProviderInitializer != nil {
		cleanuper.Add(func() error {
			return sshProviderInitializer.Cleanup(ctx)
		})
	}

	return sshProviderInitializer, kubeProvider, cleanuper.AsFunc(), nil
}
