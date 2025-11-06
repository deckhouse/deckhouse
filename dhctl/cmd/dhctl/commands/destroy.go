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
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
	tmp "github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

const (
	destroyApprovalsMessage = `You will be asked for approve multiple times.
If you understand what you are doing, you can use flag "--yes-i-am-sane-and-i-understand-what-i-am-doing" to skip approvals.
`
	destroyCacheErrorMessage = `Create cache:
	Error: %v

	Probably that Kubernetes cluster was already deleted.
	If you want to continue, please delete the cache folder manually.
`
)

func DefineDestroyCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineSanityFlags(cmd)
	app.DefineDestroyResourcesFlags(cmd)
	app.DefineTFResourceManagementTimeout(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()

		if !app.SanityCheck {
			logger.LogWarnLn(destroyApprovalsMessage)
			if !input.NewConfirmation().WithYesByDefault().WithMessage("Do you really want to DELETE all cluster resources?").Ask() {
				return fmt.Errorf("Cleanup cluster resources disallow")
			}
		}

		if err := terminal.AskBecomePassword(); err != nil {
			return err
		}
		if err := terminal.AskBastionPassword(); err != nil {
			return err
		}

		sshClient, err := sshclient.NewClientFromFlags()
		if err != nil {
			return err
		}

		if err = cache.Init(sshClient.Check().String()); err != nil {
			return fmt.Errorf(destroyCacheErrorMessage, err)
		}

		destroyer, err := destroy.NewClusterDestroyer(context.TODO(), &destroy.Params{
			NodeInterface: ssh.NewNodeInterfaceWrapper(sshClient),
			StateCache:    cache.Global(),
			SkipResources: app.SkipResources,
			Logger:        logger,
			IsDebug:       app.IsDebug,
			TmpDir:        app.TmpDirName,
		})
		if err != nil {
			return err
		}

		err = destroyer.DestroyCluster(context.Background(), app.SanityCheck)
		if err != nil {
			msg := fmt.Sprintf("Failed to destroy cluster: %v", err)
			tmp.GetGlobalTmpCleaner().DisableCleanup(msg)
			return err
		}

		return nil
	})

	return cmd
}
