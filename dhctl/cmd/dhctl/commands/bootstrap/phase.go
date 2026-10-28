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

	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func DefineBootstrapInstallDeckhouseCommand(cmd *cobra.Command, opts *options.Options) {
	app.DefineSSHFlags(cmd, opts)
	app.DefineConfigFlags(cmd, &opts.Global)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineTFResourceManagementTimeout(cmd, &opts.Cache)
	app.DefineKubeFlags(cmd, &opts.Kube)
	app.DefineDeckhouseFlags(cmd, &opts.Bootstrap)
	app.DefineDeckhouseInstallFlags(cmd, &opts.Bootstrap)
	app.DefineImgBundleFlags(cmd, &opts.Registry)
	// The registry checks run before the Deployment is created, so their skip flags belong here
	// too.
	app.DefinePreflight(cmd, &opts.Preflight)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		sshProviderInitializer, kubeProvider, err := providerinitializer.CLIProviders(ctx, opts, providerinitializer.CLIProvidersOptions{})
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
	}
}

func DefineCreateResourcesCommand(cmd *cobra.Command, opts *options.Options) {
	app.DefineSSHFlags(cmd, opts)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineConfigsForResourcesPhaseFlags(cmd, &opts.Global)
	app.DefineResourcesFlags(cmd, &opts.Bootstrap)
	app.DefineKubeFlags(cmd, &opts.Kube)
	app.DefineCacheFlags(cmd, &opts.Cache)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		sshProviderInitializer, kubeProvider, err := providerinitializer.CLIProviders(ctx, opts, providerinitializer.CLIProvidersOptions{})
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
	}
}

func DefineBootstrapAbortCommand(cmd *cobra.Command, opts *options.Options) {
	app.DefineSSHFlags(cmd, opts)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineConfigFlags(cmd, &opts.Global)
	app.DefineCacheFlags(cmd, &opts.Cache)
	app.DefineSanityFlags(cmd, &opts.Global)
	app.DefineAbortFlags(cmd, &opts.Bootstrap)
	// The same machines the bootstrap named: without them abort resolves a different
	// state cache and finds no record of the cluster it is meant to abort.
	app.DefineImmutableHostsFlags(cmd, &opts.Bootstrap)
	app.DefineImgBundleFlags(cmd, &opts.Registry)
	// Abort runs the static abort suite, but registered the flags nowhere: DisabledChecks() was
	// always empty here, so the two checks that stand between the operator and a cleanup could
	// not be skipped from the command line at all, while the gRPC path could skip them.
	app.DefinePreflight(cmd, &opts.Preflight)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		sshProviderInitializer, kubeProvider, err := providerinitializer.CLIProviders(ctx, opts, providerinitializer.CLIProvidersOptions{})
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
			progressCh, finishProgress, err := phases.InitProgress(ctx, dhlog.FromContext(ctx), "Destroy cluster")
			if err != nil {
				return err
			}
			defer finishProgress()

			onUpdateFunc := func(progress phases.Progress) error {
				progressCh <- progress
				return nil
			}
			bootstraper.OnProgressFunc = onUpdateFunc
		}

		if err = bootstraper.Abort(ctx); err != nil {
			msg := fmt.Sprintf("Failed to abort cluster: %v", err)
			cache.GetGlobalTmpCleaner().DisableCleanup(msg)
			return err
		}

		return nil
	}
}

func DefineBaseInfrastructureCommand(cmd *cobra.Command, opts *options.Options) {
	app.DefineConfigFlags(cmd, &opts.Global)
	app.DefineCacheFlags(cmd, &opts.Cache)
	app.DefineDropCacheFlags(cmd, &opts.Cache)
	app.DefineImgBundleFlags(cmd, &opts.Registry)
	// The SSH provider below is built from these flags. lib-connection used to read their
	// DHCTL_CLI_* variables for this command on its own.
	app.DefineSSHFlags(cmd, opts)
	app.DefineBecomeFlags(cmd, &opts.Become)
	// This command creates the cloud infrastructure, so the configuration checks run ahead of it
	// the same way they do in a full bootstrap — and the flags that turn them off have to exist
	// here too, or a check the operator can skip in `dhctl bootstrap` becomes unskippable here.
	app.DefinePreflight(cmd, &opts.Preflight)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		sshProviderInitializer, kubeProvider, err := providerinitializer.CLIProviders(ctx, opts, providerinitializer.CLIProvidersOptions{})
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
	}
}
