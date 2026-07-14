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
	"errors"
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func DefineBootstrapInstallDeckhouseCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, config.NewConnectionConfigParser(opts))
	app.DefineConfigFlags(cmd, &opts.Global)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineTFResourceManagementTimeout(cmd, &opts.Cache)
	app.DefineKubeFlags(cmd, &opts.Kube)
	app.DefineDeckhouseFlags(cmd, &opts.Bootstrap)
	app.DefineDeckhouseInstallFlags(cmd, &opts.Bootstrap)
	app.DefineImgBundleFlags(cmd, &opts.Registry)

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

		bootstraper := bootstrap.NewClusterBootstrapper(ctx, &bootstrap.Params{
			TmpDir:                 opts.Global.TmpDir,
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			IsDebug:                opts.Global.IsDebug,
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
	app.DefineImgBundleFlags(cmd, &opts.Registry)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		params := app.ProviderParams(&opts.Global, dhlog.FromContext(ctx))
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params)
		if err != nil {
			if !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
				return err
			}
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		bootstraper := bootstrap.NewClusterBootstrapper(ctx, &bootstrap.Params{
			TmpDir:                 opts.Global.TmpDir,
			IsDebug:                opts.Global.IsDebug,
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
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

		bootstraper := bootstrap.NewClusterBootstrapper(ctx, &bootstrap.Params{
			TmpDir:                 opts.Global.TmpDir,
			IsDebug:                opts.Global.IsDebug,
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
	app.DefineImgBundleFlags(cmd, &opts.Registry)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		params := app.ProviderParams(&opts.Global, dhlog.FromContext(ctx))
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params)
		if err != nil {
			if !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
				return err
			}
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		bootstraper := bootstrap.NewClusterBootstrapper(ctx, &bootstrap.Params{
			TmpDir:                 opts.Global.TmpDir,
			IsDebug:                opts.Global.IsDebug,
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
			Options:                opts,
		})

		interactive := input.IsTerminal() && !opts.Global.ShowProgress
		if interactive {
			progressCh, finishProgress := phases.InitProgress(ctx, dhlog.FromContext(ctx), "Destroy cluster")
			defer finishProgress()

			onUpdateFunc := func(progress phases.Progress) error {
				progressCh <- progress
				return nil
			}
			bootstraper.OnProgressFunc = onUpdateFunc
		}

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
	app.DefineImgBundleFlags(cmd, &opts.Registry)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		params := app.ProviderParams(&opts.Global, dhlog.FromContext(ctx))
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params)
		if err != nil {
			if !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
				return err
			}
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		bootstraper := bootstrap.NewClusterBootstrapper(ctx, &bootstrap.Params{
			TmpDir:                 opts.Global.TmpDir,
			IsDebug:                opts.Global.IsDebug,
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
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

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		params := app.ProviderParams(&opts.Global, dhlog.FromContext(ctx))
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params)
		if err != nil {
			if !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
				return err
			}
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		bootstraper := bootstrap.NewClusterBootstrapper(ctx, &bootstrap.Params{
			TmpDir:                 opts.Global.TmpDir,
			IsDebug:                opts.Global.IsDebug,
			Options:                opts,
			SSHProviderInitializer: sshProviderInitializer,
			KubeProvider:           kubeProvider,
		})

		return bootstraper.ExecPostBootstrap(ctx)
	})
}
