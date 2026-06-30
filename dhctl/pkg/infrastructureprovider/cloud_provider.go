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
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

func CloudProviderGetter(params CloudProviderGetterParams) infrastructure.CloudProviderGetter {
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

		providersCache := params.getProvidersCache(ctx)

		if metaConfig.ProviderName == "" {
			return providersCache.GetOrAdd(ctx, clusterUUID, metaConfig, func(ctx context.Context, clusterUUID string, _ *config.MetaConfig) (infrastructure.CloudProvider, error) {
				dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Returning DummyCloudProvider because provider name is empty. Probably it is a static cluster: %s", clusterUUID))
				return infrastructure.NewDummyCloudProvider(), nil
			})
		}

		if metaConfig.ClusterPrefix == "" {
			return nil, fmt.Errorf("Empty ClusterPrefix for cluster %s with provider %s", clusterUUID, metaConfig.ProviderName)
		}

		if metaConfig.Layout == "" {
			return nil, fmt.Errorf("Empty Layout in metaconfig for cluster %s/%s with provider %s", clusterUUID, metaConfig.ClusterPrefix, metaConfig.ProviderName)
		}

		return providersCache.GetOrAdd(ctx, clusterUUID, metaConfig, func(ctx context.Context, clusterUUID string, metaConfig *config.MetaConfig) (infrastructure.CloudProvider, error) {
			tmpDir, err := params.getTmpDir(ctx)
			if err != nil {
				return nil, err
			}

			additionalParams := params.getAdditionalParams()
			diParams, err := params.getFSDIParams(ctx)
			if err != nil {
				return nil, err
			}

			di, err := fsprovider.GetDi(ctx, diParams)
			if err != nil {
				return nil, fmt.Errorf("Cannot get fs.GetDI: %w", err)
			}

			params.setVersionsContentProviderGetter(ctx, di)

			set, err := di.SettingsProvider.GetSettings(ctx, metaConfig.ProviderName, params.AdditionalParams)
			if err != nil {
				return nil, fmt.Errorf("Cannot get settings for cluster %s with provider %s: %w", clusterUUID, metaConfig.ProviderName, err)
			}

			p := cloud.ProviderParams{
				MetaConfig:       metaConfig,
				UUID:             clusterUUID,
				DI:               di,
				TmpDir:           tmpDir,
				IsDebug:          params.isDebug(),
				Settings:         set,
				AdditionalParams: additionalParams,
			}

			provider := cloud.NewProvider(p)
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Cloud %s initialized and added to cache. Root dir is %s", provider.String(), provider.RootDir()))

			return provider, nil
		})
	}
}
