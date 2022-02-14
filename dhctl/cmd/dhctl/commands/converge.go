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
	"time"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

func DefineConvergeCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	cmd := kpApp.Command("converge", "Converge kubernetes cluster.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	runFunc := func(sshClient *ssh.Client) error {
		kubeCl, err := operations.ConnectToKubernetesAPI(sshClient)
		if err != nil {
			return err
		}

		cacheIdentity := ""
		if app.KubeConfigInCluster {
			cacheIdentity = "in-cluster"
		}

		if sshClient != nil {
			cacheIdentity = sshClient.Check().String()
		}

		if cacheIdentity == "" {
			return fmt.Errorf("Incorrect cache identity. Need to pass --ssh-host or --kube-client-from-cluster")
		}

		err = cache.Init(cacheIdentity)
		if err != nil {
			return err
		}
		inLockRunner := converge.NewInLockLocalRunner(kubeCl, "local-converger")

		runner := converge.NewRunner(kubeCl, inLockRunner)
		runner.WithChangeSettings(&terraform.ChangeActionSettings{
			AutoDismissDestructive: false,
		})

		err = runner.RunConverge()
		if err != nil {
			return fmt.Errorf("converge problem: %v", err)
		}

		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		return runFunc(sshClient)
	})
	return cmd
}

func DefineAutoConvergeCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	cmd := kpApp.Command("converge-periodical", "Start service for periodical run converge.")
	app.DefineAutoConvergeFlags(cmd)
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		if app.RunningNodeName == "" {
			return fmt.Errorf("Need to pass running node name. It is may taints terraform state while converge")
		}

		sshClient, err := ssh.NewInitClientFromFlags(false)
		if err != nil {
			return err
		}

		kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
		if err := kubeCl.Init(client.AppKubernetesInitParams()); err != nil {
			return err
		}

		inLockRunner := converge.NewInLockRunner(kubeCl, converge.AutoConvergerIdentity).
			// never force lock
			WithForceLock(false)

		app.DeckhouseTimeout = 1 * time.Hour

		runner := converge.NewRunner(kubeCl, inLockRunner).
			WithChangeSettings(&terraform.ChangeActionSettings{
				AutoDismissDestructive: true,
				AutoApprove:            true,
			}).
			WithExcludedNodes([]string{app.RunningNodeName}).
			WithSkipPhases([]converge.Phase{converge.PhaseAllNodes})

		converger := operations.NewAutoConverger(runner, app.AutoConvergeListenAddress, app.ApplyInterval)
		return converger.Start()
	})
	return cmd
}
