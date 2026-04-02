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

	"github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/kube"
	"github.com/deckhouse/lib-connection/pkg/provider"
	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
)

type SSHProviderInitializer struct {
	provider pkg.SSHProvider
	sett     *settings.BaseProviders
	config   *sshconfig.ConnectionConfig

	additionalHosts []sshconfig.Host

	mut sync.Mutex
}

func NewSSHProviderInitializer(sett *settings.BaseProviders, config *sshconfig.ConnectionConfig, additionalHosts ...sshconfig.Host) *SSHProviderInitializer {
	initializer := &SSHProviderInitializer{
		sett:            sett,
		config:          config,
		additionalHosts: additionalHosts,
	}

	var opts provider.SSHClientOption
	if len(config.Hosts) > 0 {
		opts = provider.SSHClientWithStartAfterCreate(true)
	} else if len(additionalHosts) > 0 {
		config.Hosts = additionalHosts
		opts = provider.SSHClientWithStartAfterCreate(true)
	}

	if opts != nil {
		initializer.provider = provider.NewDefaultSSHProvider(sett, config, opts)
	} else {
		initializer.provider = provider.NewDefaultSSHProvider(sett, config)
	}

	return initializer
}

func (i *SSHProviderInitializer) GetSSHProvider(_ context.Context) (pkg.SSHProvider, error) {
	if len(i.config.Hosts) > 0 {
		return i.provider, nil
	}

	if len(i.additionalHosts) > 0 {
		i.config.Hosts = i.additionalHosts
		opts := provider.SSHClientWithStartAfterCreate(true)
		i.provider = provider.NewDefaultSSHProvider(i.sett, i.config, opts)
		return i.provider, nil
	}

	return i.provider, fmt.Errorf("no hosts for ssh passed")
}

func (i *SSHProviderInitializer) Cleanup(ctx context.Context) error {
	if govalue.Nil(i.provider) {
		return nil
	}

	return i.provider.Cleanup(ctx)
}

func (i *SSHProviderInitializer) SetAdditionalHosts(hosts []sshconfig.Host) {
	i.mut.Lock()
	defer i.mut.Unlock()

	i.additionalHosts = hosts
}

func (i *SSHProviderInitializer) GetKubeProvider(ctx context.Context) pkg.KubeProvider {
	cfg := &kube.Config{}
	runnerInterface, err := provider.GetRunnerInterface(ctx, cfg, i.sett, i)
	if err != nil {
		return nil
	}
	return provider.NewDefaultKubeProvider(i.sett, cfg, runnerInterface)
}
