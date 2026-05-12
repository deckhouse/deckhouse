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

package bootstrap

import (
	"fmt"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	libdhctl_log "github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func DefineBootstrapCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, config.NewConnectionConfigParser(opts))
	app.DefineConfigFlags(cmd, &opts.Global)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineCacheFlags(cmd, &opts.Cache)
	app.DefineDropCacheFlags(cmd, &opts.Cache)
	app.DefineResourcesFlags(cmd, &opts.Bootstrap, false)
	app.DefineTFResourceManagementTimeout(cmd, &opts.Cache)
	app.DefineDeckhouseFlags(cmd, &opts.Bootstrap)
	app.DefineDontUsePublicImagesFlags(cmd, &opts.Bootstrap)
	app.DefinePostBootstrapScriptFlags(cmd, &opts.Bootstrap)
	app.DefinePreflight(cmd, &opts.Preflight)
	app.DefineImgBundleFlags(cmd, &opts.Registry)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		logger := log.GetDefaultLogger()
		extLogger, ok := logger.(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("could not get external logger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(extLogger.GetLogger())
		providerParams := app.ProviderParams(&opts.Global, loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, providerParams)
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:                 opts.Global.TmpDir,
			Logger:                 logger,
			IsDebug:                opts.Global.IsDebug,
			ResetInitialState:      false,
			DirectoryConfig:        opts.DirConfig(),
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			Options:                opts,
		})
		err = bootstraper.Bootstrap(ctx)
		if err != nil {
			msg := fmt.Sprintf("Bootstrap failed with error: %v", err)
			cache.GetGlobalTmpCleaner().DisableCleanup(msg)
			return err
		}

		return nil
	})
}
