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
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	libdhctl_log "github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge"
	statecache "github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func DefineConvergeCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, config.NewConnectionConfigParser(opts))
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineKubeFlags(cmd, &opts.Kube)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		logger := log.GetDefaultLogger()

		externalLogger, ok := logger.(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to ExternalLogger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())
		params := app.ProviderParams(&opts.Global, loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params, providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()))
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           opts.Global.TmpDir,
			DownloadDir:      opts.Global.DownloadDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			Logger:           logger,
			IsDebug:          opts.Global.IsDebug,
		})

		converger := converge.NewConverger(&converge.Params{
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			ChangesSettings: infrastructure.ChangeActionSettings{
				SkipChangesOnDeny: false,
				AutomaticSettings: infrastructure.AutomaticSettings{
					AutoDismissChanges:     false,
					AutoDismissDestructive: false,
					AutoApproveSettings: infrastructure.AutoApproveSettings{
						AutoApprove: false,
					},
				},
			},
			ProviderGetter:     providerGetter,
			TmpDir:             opts.Global.TmpDir,
			Logger:             logger,
			IsDebug:            opts.Global.IsDebug,
			DirectoryConfig:    opts.DirConfig(),
			Options:            opts,
			NoSwitchToNodeUser: app.ForceNoSwitchToNodeUser(),
		})

		cacheIdentity := ""
		if opts.Kube.InCluster {
			cacheIdentity = "in-cluster"
		}

		if sshProviderInitializer != nil {
			if sshProviderInitializer.CheckHosts() {
				sshProvider, err := sshProviderInitializer.GetSSHProvider(ctx)
				if err != nil {
					return err
				}

				sshClient, err := sshProvider.Client(ctx)
				if err != nil {
					return err
				}

				cacheIdentity = sshClient.Check().String()
			}
		}

		if opts.Kube.Config != "" {
			cacheIdentity = statecache.GetCacheIdentityFromKubeconfig(
				opts.Kube.Config,
				opts.Kube.ConfigContext,
			)
		}

		converger.CacheID = cacheIdentity

		_, err = converger.Converge(ctx)
		if err != nil {
			msg := fmt.Sprintf("Converge failed with error: %v", err)
			cache.GetGlobalTmpCleaner().DisableCleanup(msg)

			return err
		}

		return nil
	})
}

func DefineAutoConvergeCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineAutoConvergeFlags(cmd, &opts.AutoConverge)
	app.DefineSSHFlags(cmd, &opts.SSH, config.NewConnectionConfigParser(opts))
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineKubeFlags(cmd, &opts.Kube)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		logger := log.GetDefaultLogger()
		externalLogger, ok := logger.(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to ExternalLogger")
		}

		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())

		params := app.ProviderParams(&opts.Global, loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params, providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()))
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           opts.Global.TmpDir,
			DownloadDir:      opts.Global.DownloadDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			Logger:           logger,
			IsDebug:          opts.Global.IsDebug,
		})

		converger := converge.NewConverger(&converge.Params{
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			ChangesSettings: infrastructure.ChangeActionSettings{
				SkipChangesOnDeny: true,
				AutomaticSettings: infrastructure.AutomaticSettings{
					AutoDismissDestructive: true,
					AutoDismissChanges:     false,
					AutoApproveSettings: infrastructure.AutoApproveSettings{
						AutoApprove: true,
					},
				},
			},
			ProviderGetter:  providerGetter,
			TmpDir:          opts.Global.TmpDir,
			Logger:          logger,
			IsDebug:         opts.Global.IsDebug,
			DirectoryConfig: opts.DirConfig(),
			Options:         opts,
		})

		return converger.AutoConverge(ctx, opts.AutoConverge.ListenAddress, opts.AutoConverge.ApplyInterval)
	})
}

func DefineConvergeMigrationCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, config.NewConnectionConfigParser(opts))
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineKubeFlags(cmd, &opts.Kube)
	app.DefineCheckHasTerraformStateBeforeMigrateToTofu(cmd, &opts.Converge)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		logger := log.GetDefaultLogger()

		externalLogger, ok := logger.(*log.ExternalLogger)
		if !ok {
			return fmt.Errorf("cannot convert logger to ExternalLogger")
		}
		loggerProvider := libdhctl_log.SimpleLoggerProvider(externalLogger.GetLogger())

		params := app.ProviderParams(&opts.Global, loggerProvider)

		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params, providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()))
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		loggerFor := log.GetDefaultLogger()

		providersGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           opts.Global.TmpDir,
			DownloadDir:      opts.Global.DownloadDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			Logger:           loggerFor,
			IsDebug:          opts.Global.IsDebug,
		})

		converger := converge.NewConverger(&converge.Params{
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			ChangesSettings: infrastructure.ChangeActionSettings{
				AutomaticSettings: infrastructure.AutomaticSettings{
					AutoDismissDestructive: true,
					AutoDismissChanges:     true,
					AutoApproveSettings: infrastructure.AutoApproveSettings{
						AutoApprove: true,
					},
				},
				SkipChangesOnDeny: true,
			},
			CheckHasTerraformStateBeforeMigration: opts.Converge.CheckHasTerraformStateBeforeMigrateToTofu,
			ProviderGetter:                        providersGetter,
			TmpDir:                                opts.Global.TmpDir,
			Logger:                                loggerFor,
			IsDebug:                               opts.Global.IsDebug,
			DirectoryConfig:                       opts.DirConfig(),
			Options:                               opts,
		})

		cacheIdentity := ""
		if opts.Kube.InCluster {
			cacheIdentity = "in-cluster"
		}

		if sshProviderInitializer != nil {
			if sshProviderInitializer.CheckHosts() {
				sshProvider, err := sshProviderInitializer.GetSSHProvider(ctx)
				if err != nil {
					return err
				}

				sshClient, err := sshProvider.Client(ctx)
				if err != nil {
					return err
				}

				cacheIdentity = sshClient.Check().String()
			}
		}

		if opts.Kube.Config != "" {
			cacheIdentity = statecache.GetCacheIdentityFromKubeconfig(
				opts.Kube.Config,
				opts.Kube.ConfigContext,
			)
		}
		converger.CacheID = cacheIdentity

		if err := converger.ConvergeMigration(ctx); err != nil {
			msg := fmt.Sprintf("ConvergeMigration failed with error: %v", err)
			cache.GetGlobalTmpCleaner().DisableCleanup(msg)

			return err
		}

		return nil
	})
}
