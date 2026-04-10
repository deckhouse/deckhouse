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
	"context"
	"fmt"
	"strings"

	kingpin "gopkg.in/alecthomas/kingpin.v2"

	libdhctl_log "github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func DefineBootstrapInstallDeckhouseCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineTFResourceManagementTimeout(cmd)
	app.DefineKubeFlags(cmd)
	app.DefineDeckhouseFlags(cmd)
	app.DefineDeckhouseInstallFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()
		ctx := context.Background()

		teeLogger, ok := logger.(*log.TeeLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to TeeLogger")
		}

		externalLogger, ok := teeLogger.GetLogger().(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to ExternalLogger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())
		params := app.GetProviderParams(loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params)
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:                 app.TmpDirName,
			Logger:                 logger,
			IsDebug:                app.IsDebug,
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			DirectoryConfig:        app.GetDirConfig(),
		})
		return bootstraper.InstallDeckhouse(ctx)
	})

	return cmd
}

func DefineBootstrapExecuteBashibleCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineBashibleBundleFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()
		ctx := context.Background()

		teeLogger, ok := logger.(*log.TeeLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to TeeLogger")
		}

		externalLogger, ok := teeLogger.GetLogger().(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to ExternalLogger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())
		params := app.GetProviderParams(loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params)
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:                 app.TmpDirName,
			Logger:                 logger,
			IsDebug:                app.IsDebug,
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			DirectoryConfig:        app.GetDirConfig(),
		})
		return bootstraper.ExecuteBashible(ctx)
	})

	return cmd
}

func DefineCreateResourcesCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefineConfigsForResourcesPhaseFlags(cmd)
	app.DefineResourcesFlags(cmd, false)
	app.DefineKubeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()
		ctx := context.Background()

		teeLogger, ok := logger.(*log.TeeLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to TeeLogger")
		}

		externalLogger, ok := teeLogger.GetLogger().(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to ExternalLogger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())
		params := app.GetProviderParams(loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params)
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:                 app.TmpDirName,
			Logger:                 logger,
			IsDebug:                app.IsDebug,
			DirectoryConfig:        app.GetDirConfig(),
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
		})
		return bootstraper.CreateResources(ctx)
	})

	return cmd
}

func DefineBootstrapAbortCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineSanityFlags(cmd)
	app.DefineAbortFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()
		ctx := context.Background()

		teeLogger, ok := logger.(*log.TeeLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to TeeLogger")
		}

		externalLogger, ok := teeLogger.GetLogger().(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to ExternalLogger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())
		params := app.GetProviderParams(loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params)
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:                 app.TmpDirName,
			Logger:                 logger,
			IsDebug:                app.IsDebug,
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			DirectoryConfig:        app.GetDirConfig(),
		})

		err = bootstraper.Abort(context.Background(), app.ForceAbortFromCache)
		if err != nil {
			msg := fmt.Sprintf("Failed to abort cluster: %v", err)
			cache.GetGlobalTmpCleaner().DisableCleanup(msg)
			return err
		}

		return nil
	})

	return cmd
}

func DefineBaseInfrastructureCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineConfigFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineDropCacheFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()
		ctx := context.Background()

		teeLogger, ok := logger.(*log.TeeLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to TeeLogger")
		}

		externalLogger, ok := teeLogger.GetLogger().(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to ExternalLogger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())
		params := app.GetProviderParams(loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params)
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:                 app.TmpDirName,
			Logger:                 logger,
			IsDebug:                app.IsDebug,
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			DirectoryConfig:        app.GetDirConfig(),
		})

		err = bootstraper.BaseInfrastructure(context.Background())
		cache.GetGlobalTmpCleaner().DisableCleanup("Create base infra for cluster")
		return err
	})

	return cmd
}

func DefineExecPostBootstrapScript(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.NewConnectionConfigParser())
	app.DefineBecomeFlags(cmd)
	app.DefinePostBootstrapScriptFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()
		ctx := context.Background()

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:          app.TmpDirName,
			Logger:          logger,
			IsDebug:         app.IsDebug,
			DirectoryConfig: app.GetDirConfig(),
		})
		return bootstraper.ExecPostBootstrap(ctx)
	})

	return cmd
}
