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
	"os"
	"time"

	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

func DefineTestKubernetesAPIConnectionCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
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
		ready, err := checker.IsReady(context.Background(), "")
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

func DefineWaitDeploymentReadyCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	var Namespace string
	var Name string

	cmd.Flag("namespace", "Use namespace").
		StringVar(&Namespace)
	cmd.Flag("name", "Deployment name").
		StringVar(&Name)

	cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := context.Background()
		if err := terminal.AskBecomePassword(); err != nil {
			return err
		}
		if err := terminal.AskBastionPassword(); err != nil {
			return err
		}

		sshClient, err := sshclient.NewInitClientFromFlags(ctx, true)
		if err != nil {
			return err
		}

		err = log.Process("bootstrap", "Wait for Deckhouse to become Ready", func() error {
			kubeCl := client.NewKubernetesClient().
				WithNodeInterface(
					ssh.NewNodeInterfaceWrapper(sshClient),
				)
			// auto init
			err = kubeCl.Init(client.AppKubernetesInitParams())
			if err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}

			err = deckhouse.WaitForReadiness(ctx, kubeCl)
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
