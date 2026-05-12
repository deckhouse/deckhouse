// Copyright 2026 Flant JSC
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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
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

func DefineDestroyCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, config.NewConnectionConfigParser(opts))
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineCacheFlags(cmd, &opts.Cache)
	app.DefineSanityFlags(cmd, &opts.Global)
	app.DefineDestroyResourcesFlags(cmd, &opts.Destroy)
	app.DefineTFResourceManagementTimeout(cmd, &opts.Cache)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		logger := log.GetDefaultLogger()

		params, err := app.DefaultProviderParams(&opts.Global)
		if err != nil {
			return err
		}

		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params)
		if err != nil {
			return err
		}

		if !opts.Global.SanityCheck {
			logger.LogWarnLn(destroyApprovalsMessage)

			if !input.NewConfirmation().WithYesByDefault().WithMessage("Do you really want to DELETE all cluster resources?").Ask() {
				return fmt.Errorf("Cleanup cluster resources disallow")
			}
		}

		sshProvider, err := sshProviderInitializer.GetSSHProvider(ctx)
		if err != nil {
			return err
		}

		sshClient, err := sshProvider.Client(ctx)
		if err != nil {
			return err
		}

		if err = cache.Init(ctx, sshClient.Check().String(), opts.Cache); err != nil {
			return fmt.Errorf(destroyCacheErrorMessage, err)
		}

		destroyer, err := destroy.NewClusterDestroyer(ctx, &destroy.Params{
			SSHProvider:     sshProvider,
			KubeProvider:    kubeProvider,
			StateCache:      cache.Global(),
			SkipResources:   opts.Destroy.SkipResources,
			LoggerProvider:  log.SimpleLoggerProvider(logger),
			IsDebug:         opts.Global.IsDebug,
			TmpDir:          opts.Global.TmpDir,
			DirectoryConfig: opts.DirConfig(),
		})
		if err != nil {
			return err
		}

		err = destroyer.DestroyCluster(ctx, opts.Global.SanityCheck)
		if err != nil {
			msg := fmt.Sprintf("Failed to destroy cluster: %v", err)
			tmp.GetGlobalTmpCleaner().DisableCleanup(msg)

			return err
		}

		return nil
	})
}
