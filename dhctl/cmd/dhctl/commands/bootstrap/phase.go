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

func DefineBootstrapInstallDeckhouseCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, config.NewConnectionConfigParser(opts))
	app.DefineConfigFlags(cmd, &opts.Global)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineTFResourceManagementTimeout(cmd, &opts.Cache)
	app.DefineKubeFlags(cmd, &opts.Kube)
	app.DefineDeckhouseFlags(cmd, &opts.Bootstrap)
	app.DefineDeckhouseInstallFlags(cmd, &opts.Bootstrap)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		logger := log.GetDefaultLogger()

		externalLogger, ok := logger.(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to ExternalLogger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())
		providerParams := app.ProviderParams(&opts.Global, loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, providerParams, providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()))
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:                 opts.Global.TmpDir,
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			Logger:                 logger,
			IsDebug:                opts.Global.IsDebug,
			DirectoryConfig:        opts.Global.DirConfig(),
			Options:                opts,
		})

		return bootstraper.InstallDeckhouse(ctx)
	})
}

func DefineBootstrapExecuteBashibleCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, config.NewConnectionConfigParser(opts))
	app.DefineConfigFlags(cmd, &opts.Global)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineBashibleBundleFlags(cmd, &opts.Bootstrap)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()
		ctx := kpcontext.ExtractContext(c)

		externalLogger, ok := logger.(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to ExternalLogger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())
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
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			DirectoryConfig:        opts.Global.DirConfig(),
			Options:                opts,
		})
		return bootstraper.ExecuteBashible(ctx)
	})
}

func DefineCreateResourcesCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, config.NewConnectionConfigParser(opts))
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineConfigsForResourcesPhaseFlags(cmd, &opts.Global)
	app.DefineResourcesFlags(cmd, &opts.Bootstrap, false)
	app.DefineKubeFlags(cmd, &opts.Kube)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()
		ctx := kpcontext.ExtractContext(c)

		externalLogger, ok := logger.(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to ExternalLogger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())
		providerParams := app.ProviderParams(&opts.Global, loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, providerParams, providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()))
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:                 opts.Global.TmpDir,
			Logger:                 logger,
			IsDebug:                opts.Global.IsDebug,
			DirectoryConfig:        opts.Global.DirConfig(),
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			Options:                opts,
		})
		return bootstraper.CreateResources(ctx)
	})
}

func DefineBootstrapAbortCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, config.NewConnectionConfigParser(opts))
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineConfigFlags(cmd, &opts.Global)
	app.DefineCacheFlags(cmd, &opts.Cache)
	app.DefineSanityFlags(cmd, &opts.Global)
	app.DefineAbortFlags(cmd, &opts.Bootstrap)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		logger := log.GetDefaultLogger()
		ctx := kpcontext.ExtractContext(c)

		externalLogger, ok := logger.(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to ExternalLogger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())
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
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			DirectoryConfig:        opts.Global.DirConfig(),
			Options:                opts,
		})

		if err = bootstraper.Abort(ctx, opts.Bootstrap.ForceAbortFromCache); err != nil {
			msg := fmt.Sprintf("Failed to abort cluster: %v", err)
			cache.GetGlobalTmpCleaner().DisableCleanup(msg)
			return err
		}

		return nil
	})
}

func DefineBaseInfrastructureCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineConfigFlags(cmd, &opts.Global)
	app.DefineCacheFlags(cmd, &opts.Cache)
	app.DefineDropCacheFlags(cmd, &opts.Cache)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)
		logger := log.GetDefaultLogger()

		externalLogger, ok := logger.(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to ExternalLogger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())
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
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			DirectoryConfig:        opts.Global.DirConfig(),
			Options:                opts,
		})

		err = bootstraper.BaseInfrastructure(ctx)
		cache.GetGlobalTmpCleaner().DisableCleanup("Create base infra for cluster")
		return err
	})
}

func DefineExecPostBootstrapScript(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, config.NewConnectionConfigParser(opts))
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefinePostBootstrapScriptFlags(cmd, &opts.Bootstrap)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		logger := log.GetDefaultLogger()

		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
			TmpDir:          opts.Global.TmpDir,
			Logger:          logger,
			IsDebug:         opts.Global.IsDebug,
			DirectoryConfig: opts.Global.DirConfig(),
			Options:         opts,
		})

		return bootstraper.ExecPostBootstrap(ctx)
	})
}
