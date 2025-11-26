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

package registry

import (
	"fmt"
	"slices"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry/types"
)

const (
	CRIContainerdV1   CRIType = "Containerd"
	CRIContainerdV2   CRIType = "ContainerdV2"
	DefaultImagesRepo         = "registry.deckhouse.io/deckhouse/ce"
)

var (
	DefaultScheme = types.SchemeHTTPS
	SupportedCRI  = []CRIType{CRIContainerdV1, CRIContainerdV2}
)

type CRIType = string

type Config struct {
	Mode          Mode
	ModuleEnabled bool
	Settings      types.DeckhouseSettings
}

func (cfg *Config) ConfigBuilder() *ConfigBuilder {
	return NewConfigBuilder(cfg.Mode, cfg.ModuleEnabled)
}

func (cfg *Config) DeckhouseSettings() (bool, map[string]interface{}, error) {
	if !cfg.ModuleEnabled {
		return false, nil, nil
	}
	ret, err := cfg.Settings.ToMap()
	return true, ret, err
}

func NewConfig(
	deckhouseSettings *types.DeckhouseSettings,
	initConfig *types.InitConfig,
	defaultCRI string,
) (Config, error) {
	// if deckhouseSettings != nil && initConfig != nil {
	// 	return Config{}, fmt.Errorf("conflicting registry settings: specify either 'initConfig' or 'deckhouseSettings', not both")
	// }

	var settings types.DeckhouseSettings
	var mode Mode
	moduleEnabled := false

	// Prepare deckhouse settings
	switch {
	case deckhouseSettings != nil:
		settings = *deckhouseSettings
	case initConfig != nil:
		registrySettings, err := initConfig.ToRegistrySettings()
		if err != nil {
			return Config{}, fmt.Errorf("failed to get registry settings from initConfig: %w", err)
		}
		settings = types.DeckhouseSettings{
			Mode: registry_const.ModeUnmanaged,
			Unmanaged: &types.UnmanagedModeSettings{
				RegistrySettings: registrySettings,
			},
		}
	default:
		settings = types.DeckhouseSettings{
			Mode: registry_const.ModeUnmanaged,
			Unmanaged: &types.UnmanagedModeSettings{
				RegistrySettings: types.RegistrySettings{
					ImagesRepo: DefaultImagesRepo,
					Scheme:     DefaultScheme,
				},
			},
		}
	}

	settings.Correct()
	if err := settings.Validate(); err != nil {
		return Config{}, fmt.Errorf("failed to validate registry settings: %w", err)
	}

	// Prepare mode settings
	switch {
	case settings.Direct != nil:
		remote := types.Data{}
		remote.FromRegistrySettings(settings.Direct.RegistrySettings)
		mode = &DirectMode{
			Remote: remote,
		}
	case settings.Unmanaged != nil:
		remote := types.Data{}
		remote.FromRegistrySettings(settings.Unmanaged.RegistrySettings)
		mode = &UnmanagedMode{
			Remote: remote,
		}
	default:
		return Config{}, ErrUnknownMode
	}

	// Check module enable
	moduleEnabled = slices.Contains(SupportedCRI, defaultCRI)
	if mode.IsModuleRequired() && !moduleEnabled {
		return Config{}, fmt.Errorf(
			"registry module is required for mode '%s', but defaultCRI='%s' is not supported; supported: %v",
			mode.Mode(),
			defaultCRI,
			SupportedCRI,
		)
	}

	return Config{
		Mode:          mode,
		ModuleEnabled: moduleEnabled,
		Settings:      settings,
	}, nil
}
