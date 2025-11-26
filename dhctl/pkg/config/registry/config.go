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
	SupportedCRI = []types.CRIType{types.CRIContainerdV1, types.CRIContainerdV2}
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
	deckhouseSettings *types.DeckhouseSettings,
	initConfig *types.InitConfig,
	defaultCRI types.CRIType,
) (Config, error) {
	moduleEnabled := slices.Contains(SupportedCRI, defaultCRI)
	settings, err := prepareDeckhouseSettings(deckhouseSettings, initConfig)
	if err != nil {
		return Config{}, err
	}
	mode, err := NewModeSettings(settings)
	if err != nil {
		return Config{}, err
	}

	// Check module enable
	if mode.ToModel().ModuleRequired && !moduleEnabled {
		return Config{}, fmt.Errorf(
			"registry module is required for mode '%s', but defaultCRI='%s' is not supported; supported: %v",
			mode.Mode,
			defaultCRI,
			SupportedCRI,
		)
	}

	return Config{
		Settings:          mode,
		DeckhouseSettings: settings,
		ModuleEnabled:     moduleEnabled,
	}, nil
}

func prepareDeckhouseSettings(
	deckhouseSettings *types.DeckhouseSettings,
	initConfig *types.InitConfig,
) (types.DeckhouseSettings, error) {
	// Use deckhouse settings if available
	if deckhouseSettings != nil {
		settings := *deckhouseSettings
		settings.Correct()
		if err := settings.Validate(); err != nil {
			return types.DeckhouseSettings{}, fmt.Errorf("validate deckhouse settings: %w", err)
		}
		return settings, nil
	}

	// Build registry settings from init config or use defaults
	registrySettings := types.RegistrySettings{
		ImagesRepo: types.CEImagesRepo,
		Scheme:     types.CEScheme,
	}

	if initConfig != nil {
		var err error
		registrySettings, err = initConfig.ToRegistrySettings()
		if err != nil {
			return types.DeckhouseSettings{}, fmt.Errorf("get registry settings from init config: %w", err)
		}
	}

	settings := types.DeckhouseSettings{
		Mode: registry_const.ModeUnmanaged,
		Unmanaged: &types.UnmanagedModeSettings{
			RegistrySettings: registrySettings,
		},
	}
	settings.Correct()
	if err := settings.Validate(); err != nil {
		return settings, fmt.Errorf("validate deckhouse settings: %w", err)
	}
	return settings, nil
}
