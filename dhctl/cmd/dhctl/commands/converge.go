// Copyright 2021 Flant JSC
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

package commands

import (
	"context"

	"github.com/name212/govalue"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func DefineConvergeCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {

		if err := terminal.AskBecomePassword(); err != nil {
			return err
		}
		if err := terminal.AskBastionPassword(); err != nil {
			return err
		}

		sshClient, err := sshclient.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		tmpDir := app.TmpDirName
		logger := log.GetDefaultLogger()
		isDebug := app.IsDebug

		providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           tmpDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			Logger:           logger,
			IsDebug:          isDebug,
		})

		converger := converge.NewConverger(&converge.Params{
			SSHClient: sshClient,
			ChangesSettings: infrastructure.ChangeActionSettings{
				SkipChangesOnDeny: false,
				AutomaticSettings: infrastructure.AutomaticSettings{
					AutoDismissChanges:     false,
					AutoDismissDestructive: false,
					AutoApproveSettings: infrastructure.AutoApproveSettings{
						AutoApprove: false,
					},
				},
			},
			ProviderGetter: providerGetter,
			TmpDir:         tmpDir,
			Logger:         logger,
			IsDebug:        isDebug,
		})
		_, err = converger.Converge(context.Background())

		return err
	})
	return cmd
}

func DefineAutoConvergeCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineAutoConvergeFlags(cmd)
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		tmpDir := app.TmpDirName
		logger := log.GetDefaultLogger()
		isDebug := app.IsDebug

		providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           tmpDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			Logger:           logger,
			IsDebug:          isDebug,
		})

		converger := converge.NewConverger(&converge.Params{
			ChangesSettings: infrastructure.ChangeActionSettings{
				SkipChangesOnDeny: true,
				AutomaticSettings: infrastructure.AutomaticSettings{
					AutoDismissDestructive: true,
					AutoDismissChanges:     false,
					AutoApproveSettings: infrastructure.AutoApproveSettings{
						AutoApprove: true,
					},
				},
			},
			ProviderGetter: providerGetter,
			TmpDir:         tmpDir,
			Logger:         logger,
			IsDebug:        isDebug,
		})
		return converger.AutoConverge(app.AutoConvergeListenAddress, app.ApplyInterval)
	})
	return cmd
}

func DefineConvergeMigrationCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)
	app.DefineCheckHasTerraformStateBeforeMigrateToTofu(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		if err := terminal.AskBecomePassword(); err != nil {
			return err
		}
		if err := terminal.AskBastionPassword(); err != nil {
			return err
		}

		sshClient, err := sshclient.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		if govalue.IsNil(sshClient) {
			sshClient = nil
		}

		tmpDir := app.TmpDirName
		loggerFor := log.GetDefaultLogger()
		isDebug := app.IsDebug

		providersGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           tmpDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			Logger:           loggerFor,
			IsDebug:          isDebug,
		})

		converger := converge.NewConverger(&converge.Params{
			SSHClient: sshClient,
			ChangesSettings: infrastructure.ChangeActionSettings{
				AutomaticSettings: infrastructure.AutomaticSettings{
					AutoDismissDestructive: true,
					AutoDismissChanges:     true,
					AutoApproveSettings: infrastructure.AutoApproveSettings{
						AutoApprove: true,
					},
				},
				SkipChangesOnDeny: true,
			},
			CheckHasTerraformStateBeforeMigration: app.CheckHasTerraformStateBeforeMigrateToTofu,
			ProviderGetter:                        providersGetter,
			TmpDir:                                tmpDir,
			Logger:                                loggerFor,
			IsDebug:                               isDebug,
		})
		return converger.ConvergeMigration(context.Background())
	})
	return cmd
}
