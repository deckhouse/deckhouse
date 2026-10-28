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

	libcon "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/kube"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/provider"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/settings"
	sshconfig "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/config"
)

type GetProvidersOptions struct {
	// SSH is the SSH connection; without one the operation has no SSH at all.
	SSH *sshconfig.ConnectionConfig
	// RequireKube refuses a Kubernetes connection over SSH while no SSH host is known, neither
	// from the connection config nor from the state cache.
	RequireKube bool
}

// GetProviders builds the SSH initializer and the Kubernetes provider. An empty kubeCfg reaches
// Kubernetes over SSH; a kubeconfig becomes the impersonating client configuration, see
// impersonateKubeConfig.
func GetProviders(ctx context.Context, params settings.ProviderParams, kubeCfg *kube.Config, o GetProvidersOptions) (*SSHProviderInitializer, libcon.KubeProvider, error) {
	sett := settings.NewBaseProviders(params)

	var sshProviderInitializer *SSHProviderInitializer
	if o.SSH != nil {
		sshProviderInitializer = NewSSHProviderInitializer(sett, o.SSH)
	}

	cfg, err := impersonateKubeConfig(ctx, kubeCfg)
	if err != nil {
		return nil, nil, err
	}

	if cfg.OverSSH() {
		if sshProviderInitializer == nil {
			return nil, nil, ErrSSHHostRequiredForKubernetesConnection
		}

		if o.RequireKube && !sshProviderInitializer.CheckHosts(ctx) {
			return sshProviderInitializer, nil, ErrSSHHostRequiredForKubernetesConnection
		}
	}

	runnerInterface, err := provider.GetRunnerInterface(ctx, cfg, sett, sshProviderInitializer)
	if err != nil {
		return sshProviderInitializer, nil, err
	}

	return sshProviderInitializer, provider.NewDefaultKubeProvider(sett, cfg, runnerInterface), nil
}
