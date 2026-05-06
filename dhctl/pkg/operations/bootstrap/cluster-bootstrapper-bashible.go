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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/helper"
)

func (b *ClusterBootstrapper) ExecuteBashible(ctx context.Context) error {
	restore := b.applyParams()
	defer restore()

	registryConfigProvider, err := config.RegistryConfigProvider(func() ([]string, error) {
		return config.FetchDocuments(app.ConfigPaths)
	})
	if err != nil {
		return err
	}

	// Bundle registry shoud run before LoadConfigFromFile
	stop, err := registry.Start(ctx,
		registry.Params{
			Logger:         b.loggerProvider(),
			ConfigProvider: registryConfigProvider,
			BundlePath:     app.ImgBundlePath,
		},
	)
	if err != nil {
		return err
	}
	defer stop()

	metaConfig, err := config.LoadConfigFromFile(
		ctx,
		app.ConfigPaths,
		infrastructureprovider.MetaConfigPreparatorProvider(
			infrastructureprovider.NewPreparatorProviderParams(b.logger),
		),
		b.DirectoryConfig,
	)
	if err != nil {
		return err
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
		NodeIP:         app.InternalNodeIP,
		DevicePath:     app.DevicePath,
		MetaConfig:     metaConfig,
		CommanderMode:  b.CommanderMode,
		DirsConfig:     b.DirectoryConfig,
		LoggerProvider: b.loggerProvider,
	})

	if err != nil {
		return err
	}

	return nil
}
