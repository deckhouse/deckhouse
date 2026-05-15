// Copyright 2023 Flant JSC
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

package bootstrap

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/progressbar"
)

func (b *ClusterBootstrapper) InstallDeckhouse(ctx context.Context) error {
	metaConfig, err := config.ParseConfig(
		ctx,
		b.Options.Global.ConfigPaths,
		infrastructureprovider.MetaConfigPreparatorProvider(
			infrastructureprovider.NewPreparatorProviderParams(b.logger),
		),
		b.DirectoryConfig,
	)
	if err != nil {
		return err
	}

	interactive := input.IsTerminal() && !b.Options.Global.ShowProgress
	if interactive {
		intLogger, ok := b.logger.(*log.InteractiveLogger)
		if !ok {
			return fmt.Errorf("logger is not interactive")
		}
		labelChan := intLogger.GetPhaseChan()
		phasesChan := make(chan phases.Progress, 5)
		pbParam := progressbar.NewPbParams(100, "Install Deckhouse", labelChan, phasesChan)

		if err := progressbar.InitProgressBar(pbParam); err != nil {
			return err
		}

		onComplete := func() {
			pb := progressbar.GetDefaultPb()
			pb.ProgressBarPrinter.Add(100 - pb.ProgressBarPrinter.Current)
			pb.MultiPrinter.Stop()
		}
		defer onComplete()
	}

	if err := metaConfig.LoadInstallerVersion(); err != nil {
		return err
	}

	installConfig, err := config.PrepareDeckhouseInstallConfig(metaConfig)
	if err != nil {
		return err
	}

	installConfig.KubeadmBootstrap = b.Options.Bootstrap.KubeadmBootstrap
	installConfig.MasterNodeSelector = b.Options.Bootstrap.MasterNodeSelector

	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return err
	}

	_, err = InstallDeckhouse(ctx, &client.KubernetesClient{KubeClient: kubeCl}, installConfig, InstallDeckhouseParams{
		BeforeDeckhouseTask: func() error { return nil },
		State:               NewBootstrapState(cache.Global()),
		DeckhouseTimeout:    b.Options.Bootstrap.DeckhouseTimeout,
	})

	return err
}
