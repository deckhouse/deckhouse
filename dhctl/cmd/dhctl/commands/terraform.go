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
	"encoding/json"
	"fmt"
	state_terraform "github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"

	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"

	"gopkg.in/alecthomas/kingpin.v2"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
)

func DefineTerraformConvergeExporterCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("converge-exporter", "Run terraform converge exporter.")
	app.DefineKubeFlags(cmd)
	app.DefineConvergeExporterFlags(cmd)
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		exporter := operations.NewConvergeExporter(app.ListenAddress, app.MetricsPath, app.CheckInterval)
		exporter.Start()
		return nil
	})
	return cmd
}

func DefineTerraformCheckCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("check", "Check differences between state of Kubernetes cluster and Terraform state.")
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

		kubeCl, err := operations.ConnectToKubernetesAPI(sshClient)
		if err != nil {
			return err
		}

		metaConfig, err := config.ParseConfigInCluster(kubeCl)
		if err != nil {
			return err
		}

		metaConfig.UUID, err = state_terraform.GetClusterUUID(kubeCl)
		if err != nil {
			return err
		}

		statistic, err := converge.CheckState(kubeCl, metaConfig, terraform.NewTerraformContext(), converge.CheckStateOptions{})
		if err != nil {
			return err
		}

		var data []byte
		switch app.OutputFormat {
		case "yaml":
			data, err = yaml.Marshal(statistic)
			if err != nil {
				return err
			}
		case "json":
			data, err = json.Marshal(statistic)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("Unknown output format %s", app.OutputFormat)
		}

		fmt.Print(string(data))
		return nil
	})
	return cmd
}
