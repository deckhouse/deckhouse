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
	"context"
	"fmt"

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
)

func DefineDeckhouseRemoveDeployment(cmd *cobra.Command, opts *options.Options) {
	app.DefineSSHFlags(cmd, opts)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineKubeFlags(cmd, &opts.Kube)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		l := logger.FromContext(ctx)

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		sshProviderInitializer, kubeProvider, err := providerinitializer.CLIProviders(ctx, opts, providerinitializer.CLIProvidersOptions{RequireKube: true})
		if err != nil {
			return err
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}

		return logger.RunProcess(ctx, l, "Remove Deckhouse", func(ctx context.Context) error {
			kube, err := kubeProvider.Client(ctx)
			if err != nil {
				return fmt.Errorf("open kubernetes connection: %w", err)
			}

			kubeCl := &client.KubernetesClient{KubeClient: kube}

			return deckhouse.DeleteDeckhouseDeployment(ctx, kubeCl)
		})
	}
}

func DefineDeckhouseCreateDeployment(cmd *cobra.Command, opts *options.Options) {
	app.DefineSSHFlags(cmd, opts)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineConfigFlags(cmd, &opts.Global)
	app.DefineKubeFlags(cmd, &opts.Kube)

	var dryRun bool
	app.BoolFlag(cmd.Flags(), app.Flag{Name: "dry-run", Help: "Output the deployment YAML"}, &dryRun)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		l := logger.FromContext(ctx)

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		sshProviderInitializer, kubeProvider, err := providerinitializer.CLIProviders(ctx, opts, providerinitializer.CLIProvidersOptions{RequireKube: true})
		if err != nil {
			return err
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}

		metaConfig, err := config.ParseConfig(
			ctx,
			opts.Global.ConfigPaths,
			infrastructureprovider.MetaConfigValidatorProvider(),
			&opts.Global,
		)
		if err != nil {
			return err
		}

		installConfig, err := config.PrepareDeckhouseInstallConfig(ctx, metaConfig, &opts.Global)
		if err != nil {
			return err
		}

		if dryRun {
			manifest, err := deckhouse.CreateDeckhouseDeploymentManifest(installConfig)
			if err != nil {
				return err
			}
			out, err := yaml.Marshal(manifest)
			if err != nil {
				return err
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(out)); err != nil {
				return fmt.Errorf("print manifest: %w", err)
			}
			return nil
		}

		return logger.RunProcess(ctx, l, "Create Deckhouse Deployment", func(ctx context.Context) error {
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
	}
}
