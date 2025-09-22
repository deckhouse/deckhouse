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
			cachedProvider, stored := providersCache.Get(clusterUUID, metaConfig, params.Logger)
			if stored {
				return cachedProvider, nil
			}

			params.Logger.LogDebugLn("Returns DummyCloudProvider because provider name is empty. Probably it is static cluster")

			newProvider := infrastructure.NewDummyCloudProvider(params.Logger)
			return providersCache.Add(clusterUUID, metaConfig, newProvider, params.Logger), nil
		}

		providerName := metaConfig.ProviderName

		if metaConfig.ClusterPrefix == "" {
			return nil, fmt.Errorf("Empty ClusterPrefix for cluster %s with provider %s", clusterUUID, providerName)
		}

		if metaConfig.Layout == "" {
			return nil, fmt.Errorf("Empty Layout in metaconfig for cluster %s/%s with provider %s", clusterUUID, metaConfig.ClusterPrefix, providerName)
		}

		cachedProvider, stored := providersCache.Get(clusterUUID, metaConfig, params.Logger)
		if stored {
			return cachedProvider, nil
		}

		diParams := defaultFSDIParams
		diParamsLog := "Use default"
		if params.FSDIParams != nil {
			diParams = params.FSDIParams
			diParamsLog = "Using custom"
		}

		params.Logger.LogDebugF("%s FSDIParams: %v\n", diParamsLog, diParams)

		di, err := fs.GetDi(params.Logger, diParams)
		if err != nil {
			return nil, fmt.Errorf("Cannot get fs.GetDI: %w", err)
		}

		if di.VersionsContentProviderGetter == nil {
			if params.VersionProviderGetter != nil {
				params.Logger.LogDebugF("Use custom VersionProviderGetter\n")
				di.VersionsContentProviderGetter = params.VersionProviderGetter
			} else {
				params.Logger.LogDebugF("Use default VersionProviderGetter\n")
				di.VersionsContentProviderGetter = cloud.DefaultVersionContentProvider
			}
		} else {
			params.Logger.LogDebugF("fs.GetDI provider our own VersionProviderGetter\n")
		}

		set, err := di.SettingsProvider.GetSettings(ctx, providerName, params.AdditionalParams)
		if err != nil {
			return nil, fmt.Errorf("Cannot get settings for cluster %s with provider %s: %w", clusterUUID, providerName, err)
		}

		p := cloud.ProviderParams{
			MetaConfig:       metaConfig,
			UUID:             clusterUUID,
			Logger:           params.Logger,
			DI:               di,
			TmpDir:           tmpDir,
			IsDebug:          params.IsDebug,
			Settings:         set,
			AdditionalParams: params.AdditionalParams,
		}

		newProvider := providersCache.Add(clusterUUID, metaConfig, cloud.NewProvider(p), params.Logger)

		params.Logger.LogDebugF("Cloud %s initialized and added in cache. Root dir is %s\n", newProvider.String(), newProvider.RootDir())

		return newProvider, nil
	}
}
