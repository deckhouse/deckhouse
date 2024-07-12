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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infra/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
)

func DefineTestControlPlaneManagerReadyCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("manager", "Test control plane manager is ready.")
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)
	app.DefineControlPlaneFlags(cmd, false)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
		// auto init
		err = kubeCl.Init(client.AppKubernetesInitParams())
		if err != nil {
			return fmt.Errorf("open kubernetes connection: %v", err)
		}

		checker := controlplane.NewManagerReadinessChecker(kubeCl)
		ready, err := checker.IsReady(app.ControlPlaneHostname)
		if err != nil {
			return fmt.Errorf("Control plane manager is not ready: %s", err)
		}

		if ready {
			log.InfoLn("Control plane manager is ready")
		} else {
			log.WarnLn("Control plane manager is not ready")
		}

		return nil
	})
	return cmd
}

func DefineTestControlPlaneNodeReadyCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("node", "Test control plane node is ready.")
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)
	app.DefineControlPlaneFlags(cmd, true)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
		// auto init
		err = kubeCl.Init(client.AppKubernetesInitParams())
		if err != nil {
			return fmt.Errorf("open kubernetes connection: %v", err)
		}

		checker := controlplane.NewHook(kubeCl, map[string]string{
			app.ControlPlaneHostname: app.ControlPlaneIP,
		}, "").WithSourceCommandName("test")

		err = checker.IsReady()
		if err != nil {
			return fmt.Errorf("Control plane node is not ready: %v", err)
		}

		log.InfoLn("Control plane manager node is ready")

		return nil
	})
	return cmd
}
