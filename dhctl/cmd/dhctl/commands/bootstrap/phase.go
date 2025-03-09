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
	"fmt"

	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
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
		var sshClient *ssh.Client
		if len(app.SSHHosts) != 0 {
			sshClient = ssh.NewClientFromFlags()
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			NodeInterface:    ssh.NewNodeInterfaceWrapper(sshClient),
			TerraformContext: terraform.NewTerraformContext(),
		})
		return bootstraper.InstallDeckhouse()
	})

	return cmd
}

func DefineBootstrapExecuteBashibleCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineBashibleBundleFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewClientFromFlagsWithHosts()
		if err != nil {
			return fmt.Errorf("unable to create ssh-client: %w", err)
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			NodeInterface:    ssh.NewNodeInterfaceWrapper(sshClient),
			TerraformContext: terraform.NewTerraformContext(),
		})
		return bootstraper.ExecuteBashible()
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
		var sshClient *ssh.Client
		if len(app.SSHHosts) != 0 {
			sshClient = ssh.NewClientFromFlags()
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			NodeInterface:    ssh.NewNodeInterfaceWrapper(sshClient),
			TerraformContext: terraform.NewTerraformContext(),
		})
		return bootstraper.CreateResources()
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
		sshClient := ssh.NewClientFromFlags()
		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			NodeInterface:    ssh.NewNodeInterfaceWrapper(sshClient),
			TerraformContext: terraform.NewTerraformContext(),
		})
		return bootstraper.Abort(app.ForceAbortFromCache)
	})

	return cmd
}

func DefineBaseInfrastructureCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineConfigFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineDropCacheFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TerraformContext: terraform.NewTerraformContext(),
		})
		return bootstraper.BaseInfrastructure()
	})

	return cmd
}

func DefineExecPostBootstrapScript(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)
	app.DefinePostBootstrapScriptFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewClientFromFlagsWithHosts()
		if err != nil {
			return fmt.Errorf("unable to create ssh-client: %w", err)
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			NodeInterface:    ssh.NewNodeInterfaceWrapper(sshClient),
			TerraformContext: terraform.NewTerraformContext(),
		})
		return bootstraper.ExecPostBootstrap()
	})

	return cmd
}
