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
	"context"
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/progressbar"
)

func DefineDeckhouseRemoveDeployment(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, nil)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineKubeFlags(cmd, &opts.Kube)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)
		logger := log.GetDefaultLogger()

		loggerProvider := log.ExternalLoggerProvider(logger)
		params := app.ProviderParams(&opts.Global, loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(
			ctx,
			params,
			providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()),
			providerinitializer.WithRequiredKubeProvider(),
		)
		if err != nil {
			return err
		}

		if input.IsTerminal() {
			intLogger, ok := logger.(*log.InteractiveLogger)
			if !ok {
				return fmt.Errorf("logger is not interactive")
			}
			labelChan := intLogger.GetPhaseChan()
			phasesChan := make(chan phases.Progress, 5)
			pbParam := progressbar.NewPbParams(100, "Remove Deckhouse deployment", labelChan, phasesChan)

			if err := progressbar.InitProgressBar(pbParam); err != nil {
				return err
			}

			onComplete := func() {
				pb := progressbar.GetDefaultPb()
				pb.ProgressBarPrinter.Add(100 - pb.ProgressBarPrinter.Current)
				pb.MultiPrinter.Stop()
			}
			defer onComplete()
		}

		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}
		if sshProviderInitializer != nil {
			defer sshProviderInitializer.Cleanup(ctx)
		}

		return log.ProcessCtx(ctx, "default", "Remove Deckhouse️", func(ctx context.Context) error {
			kube, err := kubeProvider.Client(ctx)
			if err != nil {
				return fmt.Errorf("open kubernetes connection: %w", err)
			}
			kubeCl := &client.KubernetesClient{KubeClient: kube}

			return deckhouse.DeleteDeckhouseDeployment(ctx, kubeCl)
		})
	})
}

func DefineDeckhouseCreateDeployment(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, nil)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineConfigFlags(cmd, &opts.Global)
	app.DefineKubeFlags(cmd, &opts.Kube)

	var dryRun bool
	cmd.Flag("dry-run", "Output deployment yaml").BoolVar(&dryRun)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		logger := log.GetDefaultLogger()

		loggerProvider := log.ExternalLoggerProvider(logger)
		params := app.ProviderParams(&opts.Global, loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(
			ctx,
			params,
			providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()),
			providerinitializer.WithRequiredKubeProvider(),
		)
		if err != nil {
			return err
		}
		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}
		if sshProviderInitializer != nil {
			defer sshProviderInitializer.Cleanup(ctx)
		}

		if input.IsTerminal() {
			intLogger, ok := logger.(*log.InteractiveLogger)
			if !ok {
				return fmt.Errorf("logger is not interactive")
			}
			labelChan := intLogger.GetPhaseChan()
			phasesChan := make(chan phases.Progress, 5)
			pbParam := progressbar.NewPbParams(100, "Create Deckhouse deployment", labelChan, phasesChan)

			if err := progressbar.InitProgressBar(pbParam); err != nil {
				return err
			}

			onComplete := func() {
				pb := progressbar.GetDefaultPb()
				pb.ProgressBarPrinter.Add(100 - pb.ProgressBarPrinter.Current)
				pb.MultiPrinter.Stop()
			}
			defer onComplete()
		}

		metaConfig, err := config.ParseConfig(
			ctx,
			opts.Global.ConfigPaths,
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(logger),
			),
			opts.DirConfig(),
		)
		if err != nil {
			return err
		}

		installConfig, err := config.PrepareDeckhouseInstallConfig(metaConfig)
		if err != nil {
			return err
		}

		if dryRun {
			manifest := deckhouse.CreateDeckhouseDeploymentManifest(installConfig)
			out, err := yaml.Marshal(manifest)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
		}

		return log.ProcessCtx(ctx, "bootstrap", "Create Deckhouse Deployment", func(ctx context.Context) error {
			kube, err := kubeProvider.Client(ctx)
			if err != nil {
				return fmt.Errorf("open kubernetes connection: %w", err)
			}
			kubeCl := &client.KubernetesClient{KubeClient: kube}

			if err := deckhouse.CreateDeckhouseDeployment(ctx, kubeCl, installConfig); err != nil {
				return fmt.Errorf("deckhouse install: %v", err)
			}

			if err := deckhouse.WaitForReadiness(ctx, kubeCl, opts.Bootstrap.DeckhouseTimeout); err != nil {
				return fmt.Errorf("deckhouse install: %v", err)
			}

			return nil
		})
	})
}
