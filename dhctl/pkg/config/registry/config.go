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

var (
	ModuleEnabledCRI = []types.CRIType{types.CRIContainerdV1, types.CRIContainerdV2}
)

type Config struct {
	Settings          ModeSettings
	DeckhouseSettings types.DeckhouseSettings
	ModuleEnabled     bool
}

func (c *Config) Manifest() *ManifestBuilder {
	return NewManifestBuilder(c.Settings.ToModel(), c.ModuleEnabled)
}

func (c *Config) DeckhouseSettingsToMap() (bool, map[string]interface{}, error) {
	if !c.ModuleEnabled {
		return false, nil, nil
	}
	mapSettings, err := c.DeckhouseSettings.ToMap()
	return true, mapSettings, err
}

func NewConfig(
	deckhouse *types.DeckhouseSettings,
	initConfig *types.InitConfig,
	cri types.CRIType,
) (Config, error) {
	moduleEnabled := slices.Contains(ModuleEnabledCRI, cri)

	dekhouseSettings, err := NewDeckhouseSettings(deckhouse, initConfig)
	if err != nil {
		return Config{}, fmt.Errorf("failed to get registry settings: %w", err)
	}

	settings, err := NewModeSettings(dekhouseSettings)
	if err != nil {
		return Config{}, fmt.Errorf("failed to get registry mode settings: %w", err)
	}

	// Check if module can be enabled with current CRI
	if settings.ToModel().ModuleRequired && !moduleEnabled {
		return Config{}, fmt.Errorf(
			"registry mode '%s' is not supported with defaultCRI:'%s'. "+
				"Please switch to 'Unmanaged' registry mode or use one of defaultCRI: %v",
			settings.Mode,
			cri,
			ModuleEnabledCRI,
		)
	}

	return Config{
		Settings:          settings,
		DeckhouseSettings: dekhouseSettings,
		ModuleEnabled:     moduleEnabled,
	}, nil
}

func NewDeckhouseSettings(
	deckhouse *types.DeckhouseSettings,
	initConfig *types.InitConfig,
) (types.DeckhouseSettings, error) {
	if deckhouse != nil && initConfig != nil {
		return types.DeckhouseSettings{}, fmt.Errorf(
			"duplicate registry configuration detected in initConfiguration.deckhouse " +
				"and moduleConfig/deckhouse.spec.settings.registry. Please specify registry settings in only one location.")
	}

	// Use deckhouse settings if available
	if deckhouse != nil {
		deckhouseSettings := *deckhouse
		deckhouseSettings.CorrectWithDefault()
		if err := deckhouseSettings.Validate(); err != nil {
			return types.DeckhouseSettings{}, fmt.Errorf("validate registry settings: %w", err)
		}
		return deckhouseSettings, nil
	}

	// Build registry settings from init config or use defaults
	var registrySettings types.RegistrySettings
	if initConfig != nil {
		var err error
		registrySettings, err = initConfig.ToRegistrySettings()
		if err != nil {
			return types.DeckhouseSettings{}, fmt.Errorf("get registry settings from init config: %w", err)
		}
	}
	deckhouseSettings := types.DeckhouseSettings{
		Mode:      registry_const.ModeUnmanaged,
		Unmanaged: &registrySettings,
	}
	deckhouseSettings.CorrectWithDefault()
	if err := deckhouseSettings.Validate(); err != nil {
		return deckhouseSettings, fmt.Errorf("validate registry settings: %w", err)
	}
	return deckhouseSettings, nil
}
