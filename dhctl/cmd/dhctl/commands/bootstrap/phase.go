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

package bootstrap

import (
	"context"
	"fmt"

	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
)

func DefineBootstrapInstallDeckhouseCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineTFResourceManagementTimeout(cmd)
	app.DefineKubeFlags(cmd)
	app.DefineDeckhouseFlags(cmd)
	app.DefineDeckhouseInstallFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()

		var sshClient node.SSHClient
		var err error
		if len(app.SSHHosts) != 0 {
			sshClient, err = sshclient.NewClientFromFlags()
			if err != nil {
				return err
			}
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:        app.TmpDirName,
			NodeInterface: ssh.NewNodeInterfaceWrapper(sshClient),
			Logger:        logger,
			IsDebug:       app.IsDebug,
		})
		return bootstraper.InstallDeckhouse(context.Background())
	})

	return cmd
}

func DefineBootstrapExecuteBashibleCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineBashibleBundleFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()

		sshClient, err := sshclient.NewClientFromFlagsWithHosts()
		if err != nil {
			return fmt.Errorf("unable to create ssh-client: %w", err)
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:        app.TmpDirName,
			NodeInterface: ssh.NewNodeInterfaceWrapper(sshClient),
			Logger:        logger,
			IsDebug:       app.IsDebug,
		})
		return bootstraper.ExecuteBashible(context.Background())
	})

	return cmd
}

func DefineCreateResourcesCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)
	app.DefineConfigsForResourcesPhaseFlags(cmd)
	app.DefineResourcesFlags(cmd, false)
	app.DefineKubeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()

		var sshClient node.SSHClient
		var err error

		if len(app.SSHHosts) != 0 {
			sshClient, err = sshclient.NewClientFromFlags()
			if err != nil {
				return err
			}
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:        app.TmpDirName,
			NodeInterface: ssh.NewNodeInterfaceWrapper(sshClient),
			Logger:        logger,
			IsDebug:       app.IsDebug,
		})
		return bootstraper.CreateResources(context.Background())
	})

	return cmd
}

func DefineBootstrapAbortCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineSanityFlags(cmd)
	app.DefineAbortFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()

		sshClient, err := sshclient.NewClientFromFlags()
		if err != nil {
			return err
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:        app.TmpDirName,
			NodeInterface: ssh.NewNodeInterfaceWrapper(sshClient),
			Logger:        logger,
			IsDebug:       app.IsDebug,
		})
		return bootstraper.Abort(context.Background(), app.ForceAbortFromCache)
	})

	return cmd
}

func DefineBaseInfrastructureCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineConfigFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineDropCacheFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:  app.TmpDirName,
			Logger:  logger,
			IsDebug: app.IsDebug,
		})
		return bootstraper.BaseInfrastructure(context.Background())
	})

	return cmd
}

func DefineExecPostBootstrapScript(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)
	app.DefinePostBootstrapScriptFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()

		sshClient, err := sshclient.NewClientFromFlagsWithHosts()
		if err != nil {
			return fmt.Errorf("unable to create ssh-client: %w", err)
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:        app.TmpDirName,
			NodeInterface: ssh.NewNodeInterfaceWrapper(sshClient),
			Logger:        logger,
			IsDebug:       app.IsDebug,
		})
		return bootstraper.ExecPostBootstrap(context.Background())
	})

	return cmd
}
