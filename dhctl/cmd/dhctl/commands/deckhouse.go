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
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

func DefineDeckhouseRemoveDeployment(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, nil)
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		params, err := defaultProviderParams()
		if err != nil {
			return err
		}
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params, providerinitializer.WithKubeFlagsDefined(app.KubeFlagsDefined()))
		if err != nil {
			return err
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

func DefineDeckhouseCreateDeployment(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, nil)
	app.DefineBecomeFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineKubeFlags(cmd)

	var dryRun bool
	cmd.Flag("dry-run", "Output deployment yaml").BoolVar(&dryRun)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		logger := log.GetDefaultLogger()
		params, err := defaultProviderParams()
		if err != nil {
			return err
		}
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params, providerinitializer.WithKubeFlagsDefined(app.KubeFlagsDefined()))
		if err != nil {
			return err
		}
		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}
		if sshProviderInitializer != nil {
			defer sshProviderInitializer.Cleanup(ctx)
		}

		metaConfig, err := config.ParseConfig(
			ctx,
			app.ConfigPaths,
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(logger),
			),
			app.GetDirConfig(),
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

			if err := deckhouse.WaitForReadiness(ctx, kubeCl); err != nil {
				return fmt.Errorf("deckhouse install: %v", err)
			}

			return nil
		})
	})
}
