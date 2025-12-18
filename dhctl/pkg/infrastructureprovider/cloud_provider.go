// Copyright 2025 Flant JSC
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

package infrastructureprovider

import (
	"context"
	"fmt"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fsprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func CloudProviderGetter(params CloudProviderGetterParams) infrastructure.CloudProviderGetter {
	// early panic if log is not provided
	_, err := params.getLogger()
	if err != nil {
		panic(err)
	}

	return func(ctx context.Context, metaConfig *config.MetaConfig) (infrastructure.CloudProvider, error) {
		if metaConfig == nil {
			return nil, fmt.Errorf("Cannot get CloudProvider. metaConfig must not be nil")
		}

		if govalue.IsNil(ctx) {
			return nil, fmt.Errorf("Cannot get CloudProvider. context must not be nil")
		}

		clusterUUID, err := metaConfig.GetFullUUID()
		if err != nil {
			return nil, fmt.Errorf("Cannot get CloudProvider. clusterUUID get error: %w", err)
		}

		if clusterUUID == "" {
			return nil, fmt.Errorf("Cannot get CloudProvider. clusterUUID must not be empty")
		}

		providersCache, err := params.getProvidersCache()
		if err != nil {
			return nil, fmt.Errorf("Cannot get CloudProvider. providers cache get error: %w", err)
		}

		logger, err := params.getLogger()
		if err != nil {
			return nil, fmt.Errorf("Cannot get CloudProvider. logger get error: %w", err)
		}

		if metaConfig.ProviderName == "" {
			return providersCache.GetOrAdd(ctx, clusterUUID, metaConfig, logger, func(_ context.Context, clusterUUID string, _ *config.MetaConfig, l log.Logger) (infrastructure.CloudProvider, error) {
				l.LogDebugF("Returns DummyCloudProvider because provider name is empty. Probably it is static cluster: %s\n", clusterUUID)
				return infrastructure.NewDummyCloudProvider(l), nil
			})
		}

		if metaConfig.ClusterPrefix == "" {
			return nil, fmt.Errorf("Empty ClusterPrefix for cluster %s with provider %s", clusterUUID, metaConfig.ProviderName)
		}

		if metaConfig.Layout == "" {
			return nil, fmt.Errorf("Empty Layout in metaconfig for cluster %s/%s with provider %s", clusterUUID, metaConfig.ClusterPrefix, metaConfig.ProviderName)
		}

		return providersCache.GetOrAdd(ctx, clusterUUID, metaConfig, logger, func(ctx context.Context, clusterUUID string, metaConfig *config.MetaConfig, l log.Logger) (infrastructure.CloudProvider, error) {
			tmpDir, err := params.getTmpDir()
			if err != nil {
				return nil, err
			}

			additionalParams, err := params.getAdditionalParams()
			if err != nil {
				return nil, err
			}

			diParams, err := params.gtFSDIParams()
			if err != nil {
				return nil, err
			}

			di, err := fsprovider.GetDi(l, diParams)
			if err != nil {
				return nil, fmt.Errorf("Cannot get fs.GetDI: %w", err)
			}

			err = params.setVersionsContentProviderGetter(di)
			if err != nil {
				return nil, err
			}

			set, err := di.SettingsProvider.GetSettings(ctx, metaConfig.ProviderName, params.AdditionalParams)
			if err != nil {
				return nil, fmt.Errorf("Cannot get settings for cluster %s with provider %s: %w", clusterUUID, metaConfig.ProviderName, err)
			}

			p := cloud.ProviderParams{
				MetaConfig:       metaConfig,
				UUID:             clusterUUID,
				Logger:           l,
				DI:               di,
				TmpDir:           tmpDir,
				IsDebug:          params.isDebug(),
				Settings:         set,
				AdditionalParams: additionalParams,
			}

			provider := cloud.NewProvider(p)
			l.LogDebugF("Cloud %s initialized and added in cache. Root dir is %s\n", provider.String(), provider.RootDir())

			return provider, nil
		})
	}
}
