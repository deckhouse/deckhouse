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
	"fmt"

	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	state_terraform "github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

func DefineTerraformConvergeExporterCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineKubeFlags(cmd)
	app.DefineConvergeExporterFlags(cmd)
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		exporter := operations.NewConvergeExporter(app.ListenAddress, app.MetricsPath, app.CheckInterval)
		exporter.Start(context.Background())
		return nil
	})
	return cmd
}

func DefineTerraformCheckCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineKubeFlags(cmd)
	app.DefineOutputFlag(cmd)
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		log.InfoLn("Check started ...\n")

		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		if sshClient == nil && !app.KubeConfigInCluster {
			return fmt.Errorf("Not enough flags were passed to perform the operation.\nUse dhctl terraform check --help to get available flags.\nSsh host is not provided. Need to pass --ssh-host, or specify SSHHost manifest in the --connection-config file")
		}

		ctx := context.Background()

		kubeCl, err := kubernetes.ConnectToKubernetesAPI(ctx, ssh.NewNodeInterfaceWrapper(sshClient))
		if err != nil {
			return err
		}

		metaConfig, err := config.ParseConfigInCluster(ctx, kubeCl)
		if err != nil {
			return err
		}

		metaConfig.UUID, err = state_terraform.GetClusterUUID(ctx, kubeCl)
		if err != nil {
			return err
		}

		statistic, err := check.CheckState(
			ctx, kubeCl, metaConfig, terraform.NewTerraformContext(), check.CheckStateOptions{},
		)
		if err != nil {
			return err
		}

		data, err := statistic.Format(app.OutputFormat)
		if err != nil {
			return fmt.Errorf("Failed to format check result: %w", err)
		}

		fmt.Print(string(data))
		return nil
	})
	return cmd
}
