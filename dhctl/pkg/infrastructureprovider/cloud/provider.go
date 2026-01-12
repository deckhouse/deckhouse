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

package cloud

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type ProviderAdditionalParams struct {
	// empty
	// reserve for future usage
}

type SettingsProvider interface {
	GetSettings(ctx context.Context, provider string, additionalParams ProviderAdditionalParams) (settings.ProviderSettings, error)
}

type Version struct {
	Version string
	Arch    string
}

func (v Version) String() string {
	return fmt.Sprintf("version %s arch %s", v.Version, v.Arch)
}

type InfrastructureUtilProviderParams struct {
	Version
}

type InfrastructureUtilProvider interface {
	DownloadTerraform(ctx context.Context, params InfrastructureUtilProviderParams, destination string) error
	DownloadOpenTofu(ctx context.Context, params InfrastructureUtilProviderParams, destination string) error
}

type InfrastructurePluginProviderParams struct {
	Version
	Settings settings.ProviderSettings
}

type InfrastructurePluginProvider interface {
	DownloadPlugin(ctx context.Context, params InfrastructurePluginProviderParams, destination string) error
}

type ModulesParams struct {
	Settings settings.ProviderSettings
}

type DownloadModulesParams struct {
	ModulesParams
}

type DownloadSpecsParams struct {
	ModulesParams
}

type ProviderModulesProvider interface {
	// DownloadModules
	// destination is dir which filled with next structures (should contain)
	//  layouts/
	// optional (if layouts do not use common modules)
	//  terraform-modules/
	DownloadModules(ctx context.Context, params DownloadModulesParams, destination string) error

	// DownloadSpecs
	// destination is dir which filled with next structures (should contain)
	//  cluster_configuration.yaml
	//  cloud_discovery_data.yaml
	DownloadSpecs(ctx context.Context, params DownloadSpecsParams, destination string) error
}

type VersionContentProvider func(ctx context.Context, settings settings.ProviderSettings, metaConfig *config.MetaConfig, logger log.Logger) ([]byte, string, error)
