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
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util"
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

func WithKubeConfig(kubeConfig string) CreateProvidersOption {
	return func(o *CreateProvidersOptions) {
		o.kubeConfig = kubeConfig
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

	params := settings.ProviderParams{
		Logger:      dhlog.FromContext(ctx),
		IsDebug:     isDebug,
		NodeTmpPath: app.DeckhouseNodeTmpPath,
		NodeBinPath: app.DeckhouseNodeBinPath,
		TmpDir:      tmpDir,
	}

	providerOpts, err := providerOptions(config, options, cleanuper)
	if err != nil {
		return nil, nil, cleanuper.AsFunc(), err
	}

	sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params, providerOpts...)
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

func providerOptions(config string, options *CreateProvidersOptions, cleanuper *callback.Callback) ([]providerinitializer.ProviderOptions, error) {
	if options.kubeConfig == "" {
		return []providerinitializer.ProviderOptions{providerinitializer.WithConnectionConfig(config)}, nil
	}

	kubeConfigPath, cleanup, err := util.WriteDefaultTempFile([]byte(options.kubeConfig))
	cleanuper.Add(cleanup)

	if err != nil {
		return nil, fmt.Errorf("writing kubeconfig: %w", err)
	}

	return []providerinitializer.ProviderOptions{
		providerinitializer.WithConnectionConfig(config),
		providerinitializer.WithConnectionConfigOnly(),
		// The Commander generates the kubeconfig it sends, so its current-context is the only
		// one that can be meant.
		providerinitializer.WithKubeConfig(kubeConfigPath, "", false),
	}, nil
}
