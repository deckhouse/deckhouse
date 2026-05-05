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

package bootstrap

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	ottrace "go.opentelemetry.io/otel/trace"
	"gopkg.in/alecthomas/kingpin.v2"

	libdhctl_log "github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func DefineBootstrapCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineDropCacheFlags(cmd)
	app.DefineResourcesFlags(cmd, false)
	app.DefineTFResourceManagementTimeout(cmd)
	app.DefineDeckhouseFlags(cmd)
	app.DefineDontUsePublicImagesFlags(cmd)
	app.DefinePostBootstrapScriptFlags(cmd)
	app.DefinePreflight(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		ctx, span := telemetry.StartSpan(
			ctx,
			"dhctl/bootstrap/command",
			ottrace.WithAttributes(
				attribute.Int("dhctl.bootstrap.config_path_count", len(app.ConfigPaths)),
				attribute.Bool("dhctl.bootstrap.drop_cache", app.DropCache),
				attribute.Bool("dhctl.bootstrap.debug", app.IsDebug),
			),
		)
		defer func() {
			span.End()
		}()

		span.AddEvent("bootstrap command started")

		logger := log.GetDefaultLogger()
		extLogger, ok := logger.(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("could not get external logger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(extLogger.GetLogger())
		params := app.GetProviderParams(loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params)
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}

			span.AddEvent("providers initialized without cached hosts")
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:                 app.TmpDirName,
			Logger:                 logger,
			IsDebug:                app.IsDebug,
			ResetInitialState:      false,
			DirectoryConfig:        app.GetDirConfig(),
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
		})
		err = bootstraper.Bootstrap(ctx)
		if err != nil {
			msg := fmt.Sprintf("Bootstrap failed with error: %v", err)
			cache.GetGlobalTmpCleaner().DisableCleanup(msg)
			return err
		}

		span.AddEvent("bootstrap command completed")

		return nil
	})
}
