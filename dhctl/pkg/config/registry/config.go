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
	"encoding/json"
	"fmt"
	"slices"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry/types"
)

const (
	CRIContainerdV1   CRIType = "Containerd"
	CRIContainerdV2   CRIType = "ContainerdV2"
	defaultImagesRepo         = "registry.deckhouse.io/deckhouse/ce"
)

var (
	defaultScheme = types.SchemeHTTPS
	SupportedCRI  = []CRIType{CRIContainerdV1, CRIContainerdV2}
)

type CRIType = string

type Config struct {
	RegistryMode
	deckhouseSettings types.DeckhouseSettings
	moduleEnabled     bool
}

func NewConfig(
	deckhouseSettings *types.DeckhouseSettings,
	initConfig *types.InitConfig,
	defaultCRI string,
) (Config, error) {
	// if deckhouseSettings != nil && initConfig != nil {
	// 	return Config{}, fmt.Errorf("conflicting registry settings: specify either 'initConfig' or 'deckhouseSettings', not both")
	// }

	var config Config

	switch {
	case deckhouseSettings != nil:
		config.deckhouseSettings = *deckhouseSettings
	case initConfig != nil:
		registrySettings, err := initConfig.ToDeckhouseRegistrySettings()
		if err != nil {
			return Config{}, fmt.Errorf("failed to get registry settings from initConfig: %w", err)
		}
		config.deckhouseSettings = types.DeckhouseSettings{
			Mode: registry_const.ModeUnmanaged,
			Unmanaged: &types.UnmanagedModeSettings{
				RegistrySettings: registrySettings,
			},
		}
	default:
		config.deckhouseSettings = types.DeckhouseSettings{
			Mode: registry_const.ModeUnmanaged,
			Unmanaged: &types.UnmanagedModeSettings{
				RegistrySettings: types.RegistrySettings{
					ImagesRepo: defaultImagesRepo,
					Scheme:     defaultScheme,
				},
			},
		}
	}

	if err := config.deckhouseSettings.Validate(); err != nil {
		return Config{}, fmt.Errorf("failed to validate registry settings: %w", err)
	}

	registryMode, err := registryModeFromDeckhouse(&config.deckhouseSettings)
	if err != nil {
		return Config{}, err
	}
	config.RegistryMode = registryMode

	config.moduleEnabled = slices.Contains(SupportedCRI, defaultCRI)
	if config.IsModuleRequired() && !config.moduleEnabled {
		return Config{}, fmt.Errorf(
			"registry module is required for mode '%s', but defaultCRI='%s' is not supported; supported: %v",
			config.Mode(),
			defaultCRI,
			SupportedCRI,
		)
	}
	return config, nil
}

func (config *Config) DeckhouseSettings() (bool, map[string]interface{}, error) {
	if !config.moduleEnabled {
		return false, nil, nil
	}

	data, err := json.Marshal(config.deckhouseSettings)
	if err != nil {
		return true, nil, fmt.Errorf("failed to marshal deckhouse registry settings: %w", err)
	}

	var ret map[string]interface{}
	if err := json.Unmarshal(data, &ret); err != nil {
		return true, nil, fmt.Errorf("failed to unmarshal deckhouse registry settings: %w", err)
	}

	return true, ret, nil
}

func (config *Config) ConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		moduleEnabled: config.moduleEnabled,
		registryMode:  config.RegistryMode,
	}
}

func registryModeFromDeckhouse(ds *types.DeckhouseSettings) (RegistryMode, error) {
	switch ds.Mode {
	case registry_const.ModeDirect:
		if ds.Direct == nil {
			return nil, fmt.Errorf("field 'direct' is required when mode is 'Direct'")
		}
		m := &DirectMode{}
		m.Remote.FromDeckhouseRegistrySettings(ds.Direct.RegistrySettings)
		return m, nil

	case registry_const.ModeUnmanaged:
		if ds.Unmanaged == nil {
			return nil, fmt.Errorf("field 'unmanaged' is required when mode is 'Unmanaged'")
		}
		m := &UnmanagedMode{}
		m.Remote.FromDeckhouseRegistrySettings(ds.Unmanaged.RegistrySettings)
		return m, nil

	default:
		return nil, ErrUnknownMode
	}
}
