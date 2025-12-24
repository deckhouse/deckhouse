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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
)

func (b *ClusterBootstrapper) BaseInfrastructure(ctx context.Context) error {
	if restore, err := b.applyParams(); err != nil {
		return err
	} else {
		defer restore()
	}

	preparatorParams := infrastructureprovider.NewPreparatorProviderParams(b.logger)
	preparatorParams.WithPhaseBootstrap()
	metaConfig, err := config.ParseConfig(
		ctx,
		app.ConfigPaths,
		infrastructureprovider.MetaConfigPreparatorProvider(preparatorParams),
	)
	if err != nil {
		return err
	}

	providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
		TmpDir:           b.TmpDir,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		Logger:           b.logger,
		IsDebug:          b.IsDebug,
	})

	b.InfrastructureContext = infrastructure.NewContextWithProvider(providerGetter, b.logger)

	if metaConfig.ClusterType != config.CloudClusterType {
		return fmt.Errorf(bootstrapPhaseBaseInfraNonCloudMessage)
	}

	cachePath := metaConfig.CachePath()
	if err = cache.InitWithOptions(cachePath, cache.CacheOptions{InitialState: b.InitialState, ResetInitialState: b.ResetInitialState}); err != nil {
		// TODO: it's better to ask for confirmation here
		return fmt.Errorf(cacheMessage, cachePath, err)
	}

	stateCache := cache.Global()

	if app.DropCache {
		stateCache.Clean()
		stateCache.Delete(state.TombstoneKey)
	}

	clusterUUID, err := generateClusterUUID(stateCache)
	if err != nil {
		return err
	}
	metaConfig.UUID = clusterUUID

	cleanup, err := b.getCleanupFunc(ctx, metaConfig)
	if err != nil {
		return err
	}

	defer cleanup()

	return log.Process("bootstrap", "Cloud infrastructure", func() error {
		baseRunner, err := b.Params.InfrastructureContext.GetBootstrapBaseInfraRunner(ctx, metaConfig, stateCache)
		if err != nil {
			return err
		}

		_, err = infrastructure.ApplyPipeline(ctx, baseRunner, "Kubernetes cluster", infrastructure.GetBaseInfraResult)
		return err
	})
}
