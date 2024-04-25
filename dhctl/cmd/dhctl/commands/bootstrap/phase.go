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
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

func DefineBootstrapInstallDeckhouseCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("install-deckhouse", "Install deckhouse and wait for its readiness.")
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)
	app.DefineDeckhouseFlags(cmd)
	app.DefineDeckhouseInstallFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			SSHClient:        sshClient,
			TerraformContext: terraform.NewTerraformContext(),
		})
		return bootstraper.InstallDeckhouse()
	})

	return cmd
}

func DefineBootstrapExecuteBashibleCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("execute-bashible-bundle", "Prepare Master node and install Kubernetes.")
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineBashibleBundleFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			SSHClient:        sshClient,
			TerraformContext: terraform.NewTerraformContext(),
		})
		return bootstraper.ExecuteBashible()
	})

	return cmd
}

func DefineCreateResourcesCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("create-resources", "Create resources in Kubernetes cluster.")
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)
	app.DefineResourcesFlags(cmd, true)
	app.DefineKubeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			SSHClient:        sshClient,
			TerraformContext: terraform.NewTerraformContext(),
		})
		return bootstraper.CreateResources()
	})

	return cmd
}

func DefineBootstrapAbortCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("abort", "Delete every node, which was created during bootstrap process.")
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineSanityFlags(cmd)
	app.DefineAbortFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			SSHClient:        sshClient,
			TerraformContext: terraform.NewTerraformContext(),
		})
		return bootstraper.Abort(app.ForceAbortFromCache)
	})

	return cmd
}

func DefineBaseInfrastructureCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("base-infra", "Create base infrastructure for Cloud Kubernetes cluster.")
	app.DefineConfigFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineDropCacheFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			SSHClient:        sshClient,
			TerraformContext: terraform.NewTerraformContext(),
		})
		return bootstraper.BaseInfrastructure()
	})

	return cmd
}

func DefineExecPostBootstrapScript(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("exec-post-bootstrap", "Test scp upload and ssh run uploaded script.")
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)
	app.DefinePostBootstrapScriptFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			SSHClient:        sshClient,
			TerraformContext: terraform.NewTerraformContext(),
		})
		return bootstraper.ExecPostBootstrap()
	})

	return cmd
}
