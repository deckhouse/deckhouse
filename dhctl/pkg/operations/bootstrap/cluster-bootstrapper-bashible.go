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
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
)

func (b *ClusterBootstrapper) ExecuteBashible(ctx context.Context) error {
	if restore, err := b.applyParams(); err != nil {
		return err
	} else {
		defer restore()
	}

	metaConfig, err := config.ParseConfig(
		ctx,
		app.ConfigPaths,
		infrastructureprovider.MetaConfigPreparatorProvider(
			infrastructureprovider.NewPreparatorProviderParams(b.logger),
		),
	)
	if err != nil {
		return err
	}

	if metaConfig.ProviderSecondaryDevicesConfig.RegistryDataDeviceEnable && len(app.SystemRegistryDataDevicePath) == 0 {
		return fmt.Errorf("the '--system-registry-device-path' flag must be specified at RegistryMode!=Direct")
	}

	err = terminal.AskBecomePassword()
	if err != nil {
		return err
	}
	if err := terminal.AskBastionPassword(); err != nil {
		return err
	}

	if wrapper, ok := b.NodeInterface.(*ssh.NodeInterfaceWrapper); ok {
		if err = wrapper.Client().Start(); err != nil {
			return fmt.Errorf("unable to start ssh client: %w", err)
		}
		if err = WaitForSSHConnectionOnMaster(ctx, wrapper.Client()); err != nil {
			return fmt.Errorf("failed to wait for SSH connection on master: %v", err)
		}
	}

	if err := RunBashiblePipeline(
		ctx,
		b.NodeInterface,
		metaConfig,
		app.InternalNodeIP,
		infrastructure.DataDevices{
			KubeDataDevicePath:           app.KubeDataDevicePath,
			SystemRegistryDataDevicePath: app.SystemRegistryDataDevicePath,
		},
	); err != nil {
		return err
	}

	return nil
}
