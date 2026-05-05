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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/bundle"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
)

func (b *ClusterBootstrapper) InstallDeckhouse(ctx context.Context) error {
	restore := b.applyParams()
	defer restore()

	registryConfigProvider, err := config.RegistryConfigProvider(func() ([]string, error) {
		return config.FetchDocuments(app.ConfigPaths)
	})
	if err != nil {
		return err
	}

	// Bundle registry shoud run before ParseConfig
	stop, err := bundle.StartRegistry(ctx,
		bundle.RegistryParams{
			Logger:                 b.loggerProvider(),
			RegistryConfigProvider: registryConfigProvider,
			BundlePath:             app.ImgBundlePath,
		},
	)
	if err != nil {
		return err
	}
	defer stop()

	metaConfig, err := config.ParseConfig(
		ctx,
		app.ConfigPaths,
		infrastructureprovider.MetaConfigPreparatorProvider(
			infrastructureprovider.NewPreparatorProviderParams(b.logger),
		),
		b.DirectoryConfig,
	)
	if err != nil {
		return err
	}

	if err := metaConfig.LoadInstallerVersion(); err != nil {
		return err
	}

	installConfig, err := config.PrepareDeckhouseInstallConfig(metaConfig)
	if err != nil {
		return err
	}

	installConfig.KubeadmBootstrap = app.KubeadmBootstrap
	installConfig.MasterNodeSelector = app.MasterNodeSelector

	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return err
	}

	_, err = InstallDeckhouse(ctx, &client.KubernetesClient{KubeClient: kubeCl}, installConfig, InstallDeckhouseParams{
		BeforeDeckhouseTask: func() error { return nil },
		State:               NewBootstrapState(cache.Global()),
	})

	return err
}
