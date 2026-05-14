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
	"errors"
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

var (
	ErrHostsFromCacheNotFound = errors.New("failed to get hosts from cache")

	ErrSSHHostRequiredForKubernetesConnection = errors.New(
		"SSH connection parameters are not configured. Verify SSH connection settings, or use direct Kubernetes access via --kubeconfig or --kube-client-from-cluster",
	)
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

	if config != nil && len(config.Hosts) > 0 {
		initializer.provider = provider.NewDefaultSSHProvider(
			baseProviderSettings,
			config,
			provider.SSHClientWithStartAfterCreate(true),
		)
	}

	return initializer
}

func (i *SSHProviderInitializer) GetSSHProvider(_ context.Context) (libcon.SSHProvider, error) {
	if i == nil {
		return nil, nil
	}

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

	return provider.NewDefaultSSHProvider(i.baseProviderSettings, i.config), ErrHostsFromCacheNotFound
}

func (i *SSHProviderInitializer) Cleanup(ctx context.Context) error {
	if govalue.Nil(i.provider) {
		return nil
	}

	return i.provider.Cleanup(ctx)
}

func (i *SSHProviderInitializer) GetKubeProvider(ctx context.Context) libcon.KubeProvider {
	if i == nil {
		return nil
	}

	cfg := &kube.Config{}
	runnerInterface, err := provider.GetRunnerInterface(ctx, cfg, i.baseProviderSettings, i)
	if err != nil {
		return nil
	}
	return provider.NewDefaultKubeProvider(i.baseProviderSettings, cfg, runnerInterface)
}

func (i *SSHProviderInitializer) GetSettings() *settings.BaseProviders {
	return i.baseProviderSettings
}

func (i *SSHProviderInitializer) GetConfig() *sshconfig.ConnectionConfig {
	return i.config
}

// IsLegacyMode reports whether the connection config opts the SSH backend
// into the legacy clissh path (sshconfig.Config.ForceLegacy). Returns false
// when the connection config is not yet initialised.
func (i *SSHProviderInitializer) IsLegacyMode() bool {
	if i == nil || i.config == nil || i.config.Config == nil {
		return false
	}
	return i.config.Config.ForceLegacy
}

func (i *SSHProviderInitializer) CheckHosts() bool {
	if i == nil {
		return false
	}

	if i.config != nil {
		if len(i.config.Hosts) > 0 {
			return true
		}
	}

	lateHosts, err := i.hostsProvider()
	if err != nil {
		return false
	}
	if len(lateHosts) > 0 {
		return true
	}

	return false
}
