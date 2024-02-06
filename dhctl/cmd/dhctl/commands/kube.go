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
	"os"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infra/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

func DefineTestKubernetesAPIConnectionCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("kubernetes-api-connection", "Test connection to kubernetes api via ssh or directly.")
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		doneCh := make(chan struct{})
		tomb.RegisterOnShutdown("wait kubernetes-api-connection to stop", func() {
			<-doneCh
		})

		checker := controlplane.NewKubeProxyChecker().
			WithLogResult(true).
			WithAskPassword(true).
			WithInitParams(client.AppKubernetesInitParams())

		proxyClose := func() {
			log.InfoLn("Press Ctrl+C to close proxy connection.")
			ch := make(chan struct{})
			<-ch
		}

		// ip is empty because we want check via ssh-hosts passed via cm args
		ready, err := checker.IsReady("")
		if err != nil {
			proxyClose()
			return err
		}

		if !ready {
			proxyClose()
			return fmt.Errorf("Proxy not ready")
		}

		TestCommandDelay()
		close(doneCh)

		return nil
	})
	return cmd
}

func DefineWaitDeploymentReadyCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("deployment-ready", "Wait while deployment is ready.")
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	var Namespace string
	var Name string

	cmd.Flag("namespace", "Use namespace").
		StringVar(&Namespace)
	cmd.Flag("name", "Deployment name").
		StringVar(&Name)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		err = log.Process("bootstrap", "Wait for Deckhouse to become Ready", func() error {
			kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
			// auto init
			err = kubeCl.Init(client.AppKubernetesInitParams())
			if err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}

			err = deckhouse.WaitForReadiness(kubeCl)
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

func TestCommandDelay() {
	delayStr := os.Getenv("TEST_DELAY")
	if delayStr == "" || delayStr == "no" {
		return
	}

	delay, err := time.ParseDuration(delayStr)
	if err != nil {
		delay = time.Minute
	}

	time.Sleep(delay)
}
