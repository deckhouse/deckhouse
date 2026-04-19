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

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func DefineTestControlPlaneManagerReadyCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)
	app.DefineControlPlaneFlags(cmd, false)

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

		kubeCl := client.NewKubernetesClient().
			WithNodeInterface(
				ssh.NewNodeInterfaceWrapper(sshClient),
			)
		// auto init
		err = kubeCl.Init(client.AppKubernetesInitParams())
		if err != nil {
			return fmt.Errorf("open kubernetes connection: %v", err)
		}

		checker := controlplane.NewManagerReadinessChecker(kubernetes.NewSimpleKubeClientGetter(kubeCl))
		ready, err := checker.IsReady(ctx, app.ControlPlaneHostname)
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

func DefineTestControlPlaneNodeReadyCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)
	app.DefineControlPlaneFlags(cmd, true)

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

		kubeCl := client.NewKubernetesClient().
			WithNodeInterface(
				ssh.NewNodeInterfaceWrapper(sshClient),
			)
		// auto init
		err = kubeCl.Init(client.AppKubernetesInitParams())
		if err != nil {
			return fmt.Errorf("open kubernetes connection: %v", err)
		}

		nodeToHostForChecks := map[string]string{app.ControlPlaneHostname: app.ControlPlaneIP}

		checkers := []hook.NodeChecker{hook.NewKubeNodeReadinessChecker(kubernetes.NewSimpleKubeClientGetter(kubeCl))}

		if app.ControlPlaneHostname != "" {
			checkers = append(checkers, controlplane.NewKubeProxyChecker().WithExternalIPs(nodeToHostForChecks))
		}

		checkers = append(checkers, controlplane.NewManagerReadinessChecker(kubernetes.NewSimpleKubeClientGetter(kubeCl)))

		err = controlplane.NewChecker(nodeToHostForChecks, checkers, "test", controlplane.DefaultConfirm).
			IsAllNodesReady(ctx)
		if err != nil {
			return fmt.Errorf("control plane node is not ready: %v", err)
		}

		log.InfoLn("Control plane manager node is ready")

		return nil
	})
	return cmd
}
