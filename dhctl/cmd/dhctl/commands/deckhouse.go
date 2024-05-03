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
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
)

func DefineDeckhouseRemoveDeployment(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("remove-deployment", "Delete deckhouse deployment.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewInitClientFromFlags(true)

		err = log.Process("default", "Remove Deckhouse️", func() error {
			kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
			// auto init
			err = kubeCl.Init(client.AppKubernetesInitParams())
			if err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}

			err = deckhouse.DeleteDeckhouseDeployment(kubeCl)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}

		return nil
	})

	return cmd
}

func DefineDeckhouseCreateDeployment(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("create-deployment", "Install deckhouse after terraform is applied successful.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineKubeFlags(cmd)

	var DryRun bool
	cmd.Flag("dry-run", "Output deployment yaml").
		BoolVar(&DryRun)

	cmd.Action(func(c *kingpin.ParseContext) error {
		// Load deckhouse config
		metaConfig, err := config.ParseConfig(app.ConfigPaths)
		if err != nil {
			return err
		}

		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		installConfig, err := config.PrepareDeckhouseInstallConfig(metaConfig)
		if err != nil {
			return err
		}

		if DryRun {
			manifest := deckhouse.CreateDeckhouseDeploymentManifest(installConfig)
			out, err := yaml.Marshal(manifest)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
		}

		err = log.Process("bootstrap", "Create Deckhouse Deployment", func() error {
			kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
			if err := kubeCl.Init(client.AppKubernetesInitParams()); err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}

			err = deckhouse.CreateDeckhouseDeployment(kubeCl, installConfig)
			if err != nil {
				return fmt.Errorf("deckhouse install: %v", err)
			}

			err = deckhouse.WaitForReadiness(kubeCl)
			if err != nil {
				return fmt.Errorf("deckhouse install: %v", err)
			}
			return nil

		})
		if err != nil {
			return err
		}
		return nil
	})
	return cmd
}
