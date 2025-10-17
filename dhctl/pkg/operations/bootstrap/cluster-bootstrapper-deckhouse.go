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
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func (b *ClusterBootstrapper) InstallDeckhouse(ctx context.Context) error {
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

	err = metaConfig.LoadInstallerVersion()
	if err != nil {
		return err
	}

	installConfig, err := config.PrepareDeckhouseInstallConfig(metaConfig)
	if err != nil {
		return err
	}

	installConfig.KubeadmBootstrap = app.KubeadmBootstrap
	installConfig.MasterNodeSelector = app.MasterNodeSelector

	err = terminal.AskBecomePassword()
	if err != nil {
		return err
	}
	if err := terminal.AskBastionPassword(); err != nil {
		return err
	}

	if wrapper, ok := b.NodeInterface.(*ssh.NodeInterfaceWrapper); ok && wrapper != nil {
		sshClient := wrapper.Client()
		if sshClient != nil {
			if err = sshClient.Start(ctx); err != nil {
				return fmt.Errorf("unable to start ssh-client: %w", err)
			}
		}
	}

	kubeCl, err := kubernetes.ConnectToKubernetesAPI(ctx, b.NodeInterface)
	if err != nil {
		return err
	}

	_, err = InstallDeckhouse(ctx, kubeCl, installConfig, func() error {
		return nil
	})
	return err
}
