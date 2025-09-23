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
	"path/filepath"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	global "github.com/deckhouse/deckhouse/dhctl/pkg/global/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/interfaces"
)

type CloudProviderGetterParams struct {
	TmpDir           string
	AdditionalParams cloud.ProviderAdditionalParams
	Logger           log.Logger
	FSDIParams       *fs.DIParams
	IsDebug          bool

	VersionProviderGetter cloud.VersionsContentProviderGetter
	ProviderCache         CloudProvidersCache
}

func CloudProviderGetter(params CloudProviderGetterParams) infrastructure.CloudProviderGetter {
	if interfaces.IsNil(params.Logger) {
		panic(fmt.Errorf("CloudProviderGetterParams must have a non-nil pointer logger"))
	}

	tmpDir := params.TmpDir

	if tmpDir == "" {
		tmpDir = app.TmpDirName
		params.Logger.LogWarnF("CloudProviderGetterParams tmp dir is empty. Using global default %s\n", tmpDir)
	}

	defaultFSDIParams := &fs.DIParams{
		InfraVersionsFile: global.GetInfrastructureVersions(),
		BinariesDir:       filepath.Join(global.GetDhctlPath(), "bin"),
		CloudProviderDir:  filepath.Join(global.GetDhctlPath(), "deckhouse", "candi", "cloud-providers"),
		PluginsDir:        filepath.Join(global.GetDhctlPath(), "plugins"),
	}

	providersCache := params.ProviderCache
	providersCacheLogMessage := "Provider cache is not nil. Using custom\n"
	if interfaces.IsNil(providersCache) {
		providersCacheLogMessage = "Provider cache is nil. Using default\n"
		providersCache = defaultProvidersCache
	}

	params.Logger.LogDebugF(providersCacheLogMessage)

	getFSDIParams := func() *fs.DIParams {
		diParams := defaultFSDIParams
		diParamsLog := "Use default"
		if params.FSDIParams != nil {
			diParams = params.FSDIParams
			diParamsLog = "Using custom"
		}

		params.Logger.LogDebugF("%s FSDIParams: %+v\n", diParamsLog, diParams)
		return diParams
	}

	setVersionsContentProviderGetter := func(di *cloud.ProviderDI) {
		if di.VersionsContentProviderGetter != nil {
			params.Logger.LogDebugF("fs.GetDI provider our own VersionProviderGetter\n")
			return
		}

		if params.VersionProviderGetter != nil {
			params.Logger.LogDebugF("Use custom VersionProviderGetter\n")
			di.VersionsContentProviderGetter = params.VersionProviderGetter
			return
		}

		params.Logger.LogDebugF("Use default VersionProviderGetter\n")
		di.VersionsContentProviderGetter = cloud.DefaultVersionContentProvider
	}

	return func(ctx context.Context, metaConfig *config.MetaConfig) (infrastructure.CloudProvider, error) {
		if metaConfig == nil {
			return nil, fmt.Errorf("Cannot get CloudProvider. metaConfig must not be nil")
		}

		if interfaces.IsNil(ctx) {
			return nil, fmt.Errorf("Cannot get CloudProvider. context must not be nil")
		}

		clusterUUID, err := metaConfig.GetFullUUID()
		if err != nil {
			return nil, fmt.Errorf("Cannot get CloudProvider. clusterUUID get error: %w", err)
		}

		if clusterUUID == "" {
			return nil, fmt.Errorf("Cannot get CloudProvider. clusterUUID must not be empty")
		}

		if metaConfig.ProviderName == "" {
			return providersCache.GetOrAdd(ctx, clusterUUID, metaConfig, params.Logger, func(_ context.Context, clusterUUID string, _ *config.MetaConfig, logger log.Logger) (infrastructure.CloudProvider, error) {
				logger.LogDebugF("Returns DummyCloudProvider because provider name is empty. Probably it is static cluster: %s\n", clusterUUID)
				return infrastructure.NewDummyCloudProvider(logger), nil
			})
		}

		if metaConfig.ClusterPrefix == "" {
			return nil, fmt.Errorf("Empty ClusterPrefix for cluster %s with provider %s", clusterUUID, metaConfig.ProviderName)
		}

		if metaConfig.Layout == "" {
			return nil, fmt.Errorf("Empty Layout in metaconfig for cluster %s/%s with provider %s", clusterUUID, metaConfig.ClusterPrefix, metaConfig.ProviderName)
		}

		return providersCache.GetOrAdd(ctx, clusterUUID, metaConfig, params.Logger, func(ctx context.Context, clusterUUID string, metaConfig *config.MetaConfig, logger log.Logger) (infrastructure.CloudProvider, error) {
			di, err := fs.GetDi(logger, getFSDIParams())
			if err != nil {
				return nil, fmt.Errorf("Cannot get fs.GetDI: %w", err)
			}

			setVersionsContentProviderGetter(di)

			set, err := di.SettingsProvider.GetSettings(ctx, metaConfig.ProviderName, params.AdditionalParams)
			if err != nil {
				return nil, fmt.Errorf("Cannot get settings for cluster %s with provider %s: %w", clusterUUID, metaConfig.ProviderName, err)
			}

			p := cloud.ProviderParams{
				MetaConfig:       metaConfig,
				UUID:             clusterUUID,
				Logger:           logger,
				DI:               di,
				TmpDir:           tmpDir,
				IsDebug:          params.IsDebug,
				Settings:         set,
				AdditionalParams: params.AdditionalParams,
			}

			provider := cloud.NewProvider(p)
			logger.LogDebugF("Cloud %s initialized and added in cache. Root dir is %s\n", provider.String(), provider.RootDir())

			return provider, nil
		})
	}
}
