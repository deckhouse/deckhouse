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
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

func DefineInfrastructureConvergeExporterCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineKubeFlags(cmd, &opts.Kube)
	app.DefineConvergeExporterFlags(cmd, &opts.Converge)
	app.DefineSSHFlags(cmd, &opts.SSH, nil)
	app.DefineBecomeFlags(cmd, &opts.Become)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		opts.Global = opts.Global.RecheckNeedDownload(options.ConvergerPodsSpiCheckPaths...)

		params, err := app.DefaultProviderParams(ctx, &opts.Global)
		if err != nil {
			return err
		}
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(
			ctx,
			params,
			providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()),
			providerinitializer.WithKubeConfig(opts.Kube.Config, opts.Kube.ConfigContext, opts.Kube.InCluster),
			providerinitializer.WithRequiredKubeProvider(),
		)
		if err != nil {
			return err
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}

		kube, err := kubeProvider.Client(ctx)
		if err != nil {
			return err
		}
		kubeCl := &client.KubernetesClient{KubeClient: kube}

		exporter := operations.NewConvergeExporter(operations.ExporterParams{
			Address:       opts.Converge.ListenAddress,
			Path:          opts.Converge.MetricsPath,
			Interval:      opts.Converge.CheckInterval,
			KubeCl:        kubeCl,
			GlobalOptions: &opts.Global,
			IsDebug:       opts.Global.IsDebug,
		})

		exporter.Start(ctx)

		return nil
	})
}

func DefineInfrastructureCheckCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineKubeFlags(cmd, &opts.Kube)
	app.DefineOutputFlag(cmd, &opts.Converge)
	app.DefineSSHFlags(cmd, &opts.SSH, nil)
	app.DefineBecomeFlags(cmd, &opts.Become)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		params, err := app.DefaultProviderParams(ctx, &opts.Global)
		if err != nil {
			return err
		}
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(
			ctx,
			params,
			providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()),
			providerinitializer.WithKubeConfig(opts.Kube.Config, opts.Kube.ConfigContext, opts.Kube.InCluster),
			providerinitializer.WithRequiredKubeProvider(),
		)
		if err != nil {
			return err
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}

		dhlog.FromContext(ctx).InfoContext(ctx, "Check started ...")

		kube, err := kubeProvider.Client(ctx)
		if err != nil {
			return err
		}
		kubeCl := &client.KubernetesClient{KubeClient: kube}

		metaConfig, err := config.ParseConfigInCluster(
			ctx,
			kubeCl,
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(),
			),
			&opts.Global,
		)
		if err != nil {
			return err
		}

		metaConfig.UUID, err = infrastructurestate.GetClusterUUID(ctx, kubeCl)
		if err != nil {
			return err
		}

		providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           opts.Global.TmpDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			IsDebug:          opts.Global.IsDebug,
			GlobalOptions:    &opts.Global,
		})

		provider, err := providerGetter(ctx, metaConfig)
		if err != nil {
			return err
		}

		statistic, needMigrationToTofu, err := check.CheckState(
			ctx,
			kubeCl,
			metaConfig,
			infrastructure.NewContextWithProvider(providerGetter).
				WithUseTfCache(opts.Cache.UseTfCache).
				WithDebug(opts.Global.IsDebug),
			check.CheckStateOptions{},
			false,
			&opts.Global,
		)
		if err != nil {
			return err
		}

		data, err := statistic.Format(opts.Converge.OutputFormat)
		if err != nil {
			return fmt.Errorf("Failed to format check result: %w", err)
		}

		// todo(log): why do not use logger?
		fmt.Print(string(data))

		if provider.NeedToUseTofu() && needMigrationToTofu {
			dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Need to migrate to tofu: %v", needMigrationToTofu), dhlog.ShowInCompacted())
		}

		return provider.Cleanup()
	})
}
