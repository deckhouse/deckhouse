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
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/progressbar"
)

func (b *ClusterBootstrapper) ExecuteBashible(ctx context.Context) error {
	// Registry shoud run before LoadConfigFromFile
	registryStop, err := registry.InitFromConfig(
		ctx,
		b.loggerProvider(),
		b.Options.Global.ConfigPaths,
		b.Options.Registry.ImgBundlePath,
	)
	if err != nil {
		return err
	}
	defer registryStop()

	metaConfig, err := config.LoadConfigFromFile(
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

	interactive := input.IsTerminal()
	if interactive {
		intLogger, ok := b.logger.(*log.InteractiveLogger)
		if !ok {
			return fmt.Errorf("logger is not interactive")
		}
		labelChan := intLogger.GetPhaseChan()
		phasesChan := make(chan phases.Progress, 5)
		pbParam := progressbar.NewPbParams(100, "Bashible bundle", labelChan, phasesChan)

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

	sshProvider, err := b.SSHProviderInitializer.GetSSHProvider(ctx)
	if err != nil {
		return err
	}

	sshClient, err := sshProvider.Client(ctx)
	if err == nil {
		if err = WaitForSSHConnectionOnMaster(ctx, sshClient); err != nil {
			return fmt.Errorf("failed to wait for SSH connection on master: %v", err)
		}
	}

	nodeInterface, err := helper.GetNodeInterface(ctx, b.SSHProviderInitializer, b.SSHProviderInitializer.GetSettings())
	if err != nil {
		return fmt.Errorf("Could not get NodeInterface: %w", err)
	}

	err = RunBashiblePipeline(ctx, &BashiblePipelineParams{
		Node:           nodeInterface,
		NodeIP:         b.Options.Bootstrap.InternalNodeIP,
		DevicePath:     b.Options.Bootstrap.DevicePath,
		MetaConfig:     metaConfig,
		CommanderMode:  b.CommanderMode,
		IsDebug:        b.IsDebug,
		DirsConfig:     b.DirectoryConfig,
		LoggerProvider: b.loggerProvider,
	})

	if err != nil {
		return err
	}

	return nil
}
