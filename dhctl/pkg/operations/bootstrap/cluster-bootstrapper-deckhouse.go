// Copyright 2023 Flant JSC
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
	"context"
	"fmt"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/suites"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func (b *ClusterBootstrapper) InstallDeckhouse(ctx context.Context) error {
	// Registry shoud run before LoadConfigFromFile
	registryStop, err := registry.InitFromConfig(
		ctx,
		dhlog.FromContext(ctx),
		b.Options.Global.ConfigPaths,
		b.Options.Registry.ImgBundlePath,
	)
	if err != nil {
		return err
	}
	defer registryStop()

	metaConfig, err := config.ParseConfig(
		ctx,
		b.Options.Global.ConfigPaths,
		infrastructureprovider.MetaConfigValidatorProvider(),
		&b.Options.Global,
	)
	if err != nil {
		return err
	}

	if err := config.ApplyCNIBootstrap(ctx, metaConfig, &b.Options.Global); err != nil {
		return fmt.Errorf("apply cni bootstrap: %w", err)
	}

	body := func(_ chan phases.Progress) error {
		if err := metaConfig.LoadInstallerVersion(); err != nil {
			return err
		}

		installConfig, err := config.PrepareDeckhouseInstallConfig(ctx, metaConfig, &b.Options.Global)
		if err != nil {
			return err
		}

		installConfig.KubeadmBootstrap = b.Options.Bootstrap.KubeadmBootstrap
		installConfig.MasterNodeSelector = b.Options.Bootstrap.MasterNodeSelector

		// This command pulls the Deckhouse image into a cluster that already exists, and it had
		// no checks at all: a registry it cannot reach, or credentials it is not accepted with,
		// surfaced fifteen minutes later as a Deckhouse pod stuck in Pending. The global suite
		// asks exactly that, in one round trip, before the Deployment is created.
		if err := preflight.RunSuite(ctx, suites.NewGlobalSuite(suites.GlobalDeps{
			MetaConfig:    metaConfig,
			InstallConfig: installConfig,
			BuildInfo:     b.Options.BuildInfo,
		}), preflight.PhasePreInfra, "Preflight checks: install-deckhouse", &b.Options.Preflight); err != nil {
			return err
		}

		kubeCl, err := b.KubeProvider.Client(ctx)
		if err != nil {
			return err
		}

		// The node interface for the same reason as in the full bootstrap: this phase performs the
		// handover of the store on the first master, which runs commands there.
		nodeInterface, err := helper.GetNodeInterface(ctx, b.SSHProviderInitializer, b.SSHProviderInitializer.GetSettings())
		if err != nil {
			return fmt.Errorf("Could not get NodeInterface: %w", err)
		}

		_, err = InstallDeckhouse(
			ctx,
			(&client.KubernetesClient{KubeClient: kubeCl}).WithNodeInterface(nodeInterface),
			installConfig,
			InstallDeckhouseParams{
				BeforeDeckhouseTask: func() error { return nil },
				DeckhouseTimeout:    b.Options.Bootstrap.DeckhouseTimeout,
			},
		)

		return err
	}

	interactive := input.IsTerminal() && !b.Options.Global.ShowProgress
	if interactive {
		return runProgress(ctx, dhlog.FromContext(ctx), "Install Deckhouse", body)
	}

	return body(nil)
}
