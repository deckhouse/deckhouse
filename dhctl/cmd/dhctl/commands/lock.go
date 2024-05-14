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

	kingpin "gopkg.in/alecthomas/kingpin.v2"
	v1 "k8s.io/api/coordination/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

const autoConvergerErrorFmt = `Error: converge locked by auto-converger.
If you are confident in your actions, you can use the flag "--yes-i-am-sane-and-i-understand-what-i-am-doing"

Lock info:

%s
`

func DefineReleaseConvergeLockCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("release", "Release converge lock fully. It's remove converge lease lock from cluster regardless of owner. Be careful")
	app.DefineSanityFlags(cmd)
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
		if err := kubeCl.Init(client.AppKubernetesInitParams()); err != nil {
			return err
		}

		confirm := func(lease *v1.Lease) error {
			if app.SanityCheck {
				return nil
			}

			info, _ := client.LockInfo(lease)

			if *lease.Spec.HolderIdentity == converge.AutoConvergerIdentity {
				return fmt.Errorf(autoConvergerErrorFmt, info)
			}

			c := input.NewConfirmation()
			approve := c.WithMessage(fmt.Sprintf("Do you want to release lock:\n\n%s", info)).Ask()
			if !approve {
				return fmt.Errorf("Don't confirm release lock")
			}

			return nil
		}

		cnf := converge.GetLockLeaseConfig("lock-releaser")
		return client.RemoveLease(kubeCl, cnf, confirm)
	})
	return cmd
}
