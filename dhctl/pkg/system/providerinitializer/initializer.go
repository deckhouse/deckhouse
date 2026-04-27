// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package providerinitializer

import (
	"context"
	"fmt"
	"sync"

	"github.com/name212/govalue"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/kube"
	"github.com/deckhouse/lib-connection/pkg/provider"
	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"

	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
)

type SSHProviderInitializer struct {
	provider             libcon.SSHProvider
	baseProviderSettings *settings.BaseProviders
	config               *sshconfig.ConnectionConfig

	hostsProvider func() ([]sshconfig.Host, error)

	mut sync.Mutex
}

func NewSSHProviderInitializer(baseProviderSettings *settings.BaseProviders, config *sshconfig.ConnectionConfig) *SSHProviderInitializer {
	initializer := &SSHProviderInitializer{
		baseProviderSettings: baseProviderSettings,
		config:               config,
		hostsProvider: func() ([]sshconfig.Host, error) {
			c := cache.Global()
			if c == nil {
				return nil, fmt.Errorf("global cache is not initialized yet")
			}
			return state.GetMasterHosts(context.Background(), c)
		},
	}

	if len(config.Hosts) > 0 {
		initializer.provider = provider.NewDefaultSSHProvider(
			baseProviderSettings,
			config,
			provider.SSHClientWithStartAfterCreate(true),
		)
	}

	return initializer
}

func (i *SSHProviderInitializer) GetSSHProvider(_ context.Context) (libcon.SSHProvider, error) {
	i.mut.Lock()
	defer i.mut.Unlock()

	if govalue.NotNil(i.provider) {
		return i.provider, nil
	}

	lateHosts, err := i.hostsProvider()
	if err == nil && len(lateHosts) > 0 {
		i.config.Hosts = lateHosts
		opts := provider.SSHClientWithStartAfterCreate(true)
		i.provider = provider.NewDefaultSSHProvider(i.baseProviderSettings, i.config, opts)
		return i.provider, nil
	}

	return provider.NewDefaultSSHProvider(i.baseProviderSettings, i.config), fmt.Errorf("failed to get hosts from cache: %w", err)
}

func (i *SSHProviderInitializer) Cleanup(ctx context.Context) error {
	if govalue.Nil(i.provider) {
		return nil
	}

	return i.provider.Cleanup(ctx)
}

func (i *SSHProviderInitializer) GetKubeProvider(ctx context.Context) libcon.KubeProvider {
	cfg := &kube.Config{}
	runnerInterface, err := provider.GetRunnerInterface(ctx, cfg, i.baseProviderSettings, i)
	if err != nil {
		return nil
	}
	return provider.NewDefaultKubeProvider(i.baseProviderSettings, cfg, runnerInterface)
}
