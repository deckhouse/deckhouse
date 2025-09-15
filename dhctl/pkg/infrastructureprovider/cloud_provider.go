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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
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

	return func(ctx context.Context, metaConfig *config.MetaConfig) (infrastructure.CloudProvider, error) {
		if metaConfig == nil {
			return nil, fmt.Errorf("Cannot get CloudProvider. metaConfig must not be nil")
		}

		if metaConfig.ProviderName == "" {
			params.Logger.LogDebugLn("Returns DummyCloudProvider because provider name is empty. Probably it is static cluster")
			return infrastructure.NewDummyCloudProvider(params.Logger), nil
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

		di := fs.GetDi(params.Logger)

		p := cloud.ProviderParams{
			AdditionalParams: params.AdditionalParams,
		}

		provider := cloud.NewProvider(metaConfig, clusterUUID, di, p, tmpDir, params.Logger)

		params.Logger.LogDebugF("Cloud provider %s initialized for cluster %s with uuid. Root dir is %s\n",
			provider.Name(), clusterUUID, provider.RootDir())

		return provider, nil
	}
}
