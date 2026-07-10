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
	"errors"
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
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

		params := app.ProviderParams(&opts.Global, dhlog.FromContext(ctx))
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params,
			providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()),
			providerinitializer.WithKubeConfig(opts.Kube.Config, opts.Kube.ConfigContext, opts.Kube.InCluster),
		)
		if err != nil {
			if !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
				return err
			}
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           opts.Global.TmpDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			IsDebug:          opts.Global.IsDebug,
			GlobalOptions:    &opts.Global,
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
			IsDebug:            opts.Global.IsDebug,
			Options:            opts,
			NoSwitchToNodeUser: app.ForceNoSwitchToNodeUser(),
		})

		cacheIdentity := ""
		if opts.Kube.InCluster {
			cacheIdentity = "in-cluster"
		}

		if sshProviderInitializer != nil {
			if sshProviderInitializer.CheckHosts(ctx) {
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

		// in general path we check that /deckhouse/modules, /deckhouse/global-hooks,
		// /deckhouse/candi/version_map.yml is present and if not download all deps from registry
		// but in exporter and autoconverger we do not need it
		// and we reset it here
		// unfortianally global params parsed in place when we do no have command
		// that user ran
		opts.Global = opts.Global.RecheckNeedDownload(options.ConvergerPodsSpiCheckPaths...)

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		params := app.ProviderParams(&opts.Global, dhlog.FromContext(ctx))
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params,
			providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()),
			providerinitializer.WithKubeConfig(opts.Kube.Config, opts.Kube.ConfigContext, opts.Kube.InCluster),
		)
		if err != nil {
			if !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
				return err
			}
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           opts.Global.TmpDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			IsDebug:          opts.Global.IsDebug,
			GlobalOptions:    &opts.Global,
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
			ProviderGetter: providerGetter,
			TmpDir:         opts.Global.TmpDir,
			IsDebug:        opts.Global.IsDebug,
			Options:        opts,
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

		// in general path we check that /deckhouse/modules, /deckhouse/global-hooks,
		// /deckhouse/candi/version_map.yml is present and if not download all deps from registry
		// but in exporter and autoconverger we do not need it
		// and we reset it here
		// unfortianally global params parsed in place when we do no have command
		// that user ran
		// converge migration also can run as sidecar of auto-converger pod
		opts.Global = opts.Global.RecheckNeedDownload(options.ConvergerPodsSpiCheckPaths...)

		params := app.ProviderParams(&opts.Global, dhlog.FromContext(ctx))

		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params,
			providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()),
			providerinitializer.WithKubeConfig(opts.Kube.Config, opts.Kube.ConfigContext, opts.Kube.InCluster),
		)
		if err != nil {
			if !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
				return err
			}
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		providersGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           opts.Global.TmpDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			IsDebug:          opts.Global.IsDebug,
			GlobalOptions:    &opts.Global,
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
			IsDebug:                               opts.Global.IsDebug,
			Options:                               opts,
		})

		cacheIdentity := ""
		if opts.Kube.InCluster {
			cacheIdentity = "in-cluster"
		}

		if sshProviderInitializer != nil {
			if sshProviderInitializer.CheckHosts(ctx) {
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
