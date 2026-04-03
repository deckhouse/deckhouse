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
	"fmt"
	"strings"

	"github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/kube"
	"github.com/deckhouse/lib-connection/pkg/provider"
	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	libdhctl_log "github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util/callback"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

func CreateProviders(ctx context.Context, config string, logger log.Logger, isDebug bool, tmpDir string) (*providerinitializer.SSHProviderInitializer, pkg.KubeProvider, func() error, error) {
	cleanuper := callback.NewCallback()

	loggerProvider := libdhctl_log.SimpleLoggerProvider(logger.(*log.ExternalLogger).GetLogger())
	params := settings.ProviderParams{LoggerProvider: loggerProvider, IsDebug: isDebug, NodeTmpPath: app.DeckhouseNodeTmpPath, NodeBinPath: app.DeckhouseNodeBinPath, TmpDir: tmpDir}
	sett := settings.NewBaseProviders(params)

	sshConfig, err := sshconfig.ParseConnectionConfig(strings.NewReader(config), sett, sshconfig.ParseWithRequiredSSHHost(false))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing connection config: %w", err)
	}

	kubecfg := &kube.Config{}
	sshProviderInitializer := providerinitializer.NewSSHProviderInitializer(sett, sshConfig)
	cleanuper.Add(func() error {
		return sshProviderInitializer.Cleanup(ctx)
	})

	mastersIPs, err := state.GetMasterHostsIPs(cache.Global())
	if err != nil {
		return nil, nil, cleanuper.AsFunc(), fmt.Errorf("getting IPs from cache: %w", err)
	}
	var sshHosts []sshconfig.Host
	if len(mastersIPs) > 0 {
		for _, h := range mastersIPs {
			sshHosts = append(sshHosts, sshconfig.Host{Host: h.Host})
		}
	}
	sshProviderInitializer.SetAdditionalHosts(sshHosts)

	runnerInterface, err := provider.GetRunnerInterface(ctx, kubecfg, sett, sshProviderInitializer)
	if err != nil {
		return nil, nil, cleanuper.AsFunc(), err
	}
	kubeProvider := provider.NewDefaultKubeProvider(sett, kubecfg, runnerInterface)
	cleanuper.Add(func() error {
		return kubeProvider.Cleanup(ctx)
	})

	return sshProviderInitializer, kubeProvider, cleanuper.AsFunc(), nil
}
