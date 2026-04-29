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

package commands

import (
	"errors"
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

func DefineInfrastructureConvergeExporterCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineKubeFlags(cmd)
	app.DefineConvergeExporterFlags(cmd)
	app.DefineSSHFlags(cmd, nil)
	app.DefineBecomeFlags(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		logger := log.GetDefaultLogger()

		exporter := operations.NewConvergeExporter(operations.ExporterParams{
			Address:  app.ListenAddress,
			Path:     app.MetricsPath,
			Interval: app.CheckInterval,
			TmpDir:   app.TmpDirName,
			Logger:   logger,
			IsDebug:  app.IsDebug,
		})

		exporter.Start(ctx)

		return nil
	})
}

func DefineInfrastructureCheckCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineKubeFlags(cmd)
	app.DefineOutputFlag(cmd)
	app.DefineSSHFlags(cmd, nil)
	app.DefineBecomeFlags(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		logger := log.GetDefaultLogger()
		params, err := defaultProviderParams()
		if err != nil {
			return err
		}
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params, providerinitializer.WithKubeFlagsDefined(app.KubeFlagsDefined()))
		if err != nil {
			if !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
				return err
			}
		}
		if kubeProvider == nil {
			return fmt.Errorf("Not enough flags were passed to perform the operation.\n" +
				"Use dhctl terraform check --help to get available flags.\n" +
				"Ssh host is not provided. Need to pass --ssh-host, or specify SSHHost manifest in the --connection-config file")
		}
		if sshProviderInitializer != nil {
			defer sshProviderInitializer.Cleanup(ctx)
		}
		logger.LogInfoLn("Check started ...\n")

		kube, err := kubeProvider.Client(ctx)
		if err != nil {
			return err
		}
		kubeCl := &client.KubernetesClient{KubeClient: kube}

		metaConfig, err := config.ParseConfigInCluster(
			ctx,
			kubeCl,
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(logger),
			),
			app.GetDirConfig(),
		)
		if err != nil {
			return err
		}

		metaConfig.UUID, err = infrastructurestate.GetClusterUUID(ctx, kubeCl)
		if err != nil {
			return err
		}

		providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           app.TmpDirName,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			Logger:           logger,
			IsDebug:          app.IsDebug,
		})

		provider, err := providerGetter(ctx, metaConfig)
		if err != nil {
			return err
		}

		statistic, needMigrationToTofu, err := check.CheckState(
			ctx,
			kubeCl,
			metaConfig,
			infrastructure.NewContextWithProvider(providerGetter, logger),
			check.CheckStateOptions{},
			false,
		)
		if err != nil {
			return err
		}

		data, err := statistic.Format(app.OutputFormat)
		if err != nil {
			return fmt.Errorf("Failed to format check result: %w", err)
		}

		// todo(log): why do not use logger?
		fmt.Print(string(data))

		if provider.NeedToUseTofu() && needMigrationToTofu {
			// todo(log): why do not use logger?
			fmt.Printf("\nNeed migrate to tofu: %v\n", needMigrationToTofu)
		}

		return provider.Cleanup()
	})
}
