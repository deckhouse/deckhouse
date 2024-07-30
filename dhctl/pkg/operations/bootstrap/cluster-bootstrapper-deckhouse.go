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
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func (b *ClusterBootstrapper) InstallDeckhouse() error {
	if restore, err := b.applyParams(); err != nil {
		return err
	} else {
		defer restore()
	}

	metaConfig, err := config.ParseConfig(app.ConfigPaths)
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

	if b.SSHClient != nil {
		if _, err := b.SSHClient.Start(); err != nil {
			return fmt.Errorf("unable to start ssh-client: %w", err)
		}
		err = terminal.AskBecomePassword()
		if err != nil {
			return err
		}
	}

	kubeCl, err := operations.ConnectToKubernetesAPI(b.SSHClient)
	if err != nil {
		return err
	}

	return InstallDeckhouse(kubeCl, installConfig)
}
