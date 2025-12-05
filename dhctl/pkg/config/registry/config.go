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

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	init_config "github.com/deckhouse/deckhouse/go_lib/registry/models/init-config"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/module-config"
)

type Config struct {
	Settings          ModeSettings
	DeckhouseSettings module_config.DeckhouseSettings
	ModuleEnabled     bool
}

// UseDefault configures the registry with default CE settings.
// When no registry configuration is provided:
// - If Direct mode is supported (based on CRI type), uses Direct mode
// - Otherwise, falls back to Unmanaged mode
// - All parameters are populated with default values for the CE registry
func (c *Config) UseDefault(
	cri constant.CRIType,
) error {
	userSettings := module_config.DeckhouseSettings{
		Mode:      constant.ModeUnmanaged,
		Unmanaged: &module_config.RegistrySettings{},
	}
	if constant.ModuleEnabled(cri) {
		userSettings = module_config.DeckhouseSettings{
			Mode:   constant.ModeDirect,
			Direct: &module_config.RegistrySettings{},
		}
	}
	return c.UseDeckhouseSettings(
		userSettings,
		cri,
	)
}

// UseInitConfig configures registry using legacy initConfiguration.
// Note: This method maintains backward compatibility and only supports Unmanaged mode.
func (c *Config) UseInitConfig(
	initConfig init_config.Config,
	cri constant.CRIType,
) error {
	userRegistrySettings, err := initConfig.ToRegistrySettings()
	if err != nil {
		return fmt.Errorf("get registry settings from 'initConfiguration': %w", err)
	}
	userSettings := module_config.DeckhouseSettings{
		Mode:      constant.ModeUnmanaged,
		Unmanaged: &userRegistrySettings,
	}
	return c.UseDeckhouseSettings(
		userSettings,
		cri,
	)
}

// UseDeckhouseSettings configures registry using deckhouse ModuleConfig settings.
// The operation mode (Direct/Unmanaged) is determined from the user configuration.
func (c *Config) UseDeckhouseSettings(
	userSettings module_config.DeckhouseSettings,
	cri constant.CRIType,
) error {
	// Prepare settings
	deckhouseSettings := module_config.DeckhouseSettings{}
	deckhouseSettings.ApplySettings(userSettings)

	// Validate
	if err := deckhouseSettings.Validate(); err != nil {
		return fmt.Errorf("validate registry settings: %w", err)
	}

	// Check if module can be enabled with current CRI
	moduleEnabled := constant.ModuleEnabled(cri)
	moduleRequired := constant.ModuleRequired(deckhouseSettings.Mode)
	if moduleRequired && !moduleEnabled {
		return fmt.Errorf(
			"registry mode '%s' is not supported with defaultCRI:'%s'. "+
				"Please switch to 'Unmanaged' registry mode or use one of defaultCRI: %v",
			deckhouseSettings.Mode,
			cri,
			constant.ModuleEnabledCRI,
		)
	}

	// Prepare mode settings
	settings, err := newModeSettings(deckhouseSettings)
	if err != nil {
		return fmt.Errorf("get registry mode settings: %w", err)
	}

	*c = Config{
		Settings:          settings,
		DeckhouseSettings: deckhouseSettings,
		ModuleEnabled:     moduleEnabled,
	}
	return nil
}

func (c *Config) Manifest() *ManifestBuilder {
	return newManifestBuilder(c.Settings.ToModel(), c.ModuleEnabled)
}

func (c *Config) DeckhouseSettingsToMap() (bool, map[string]interface{}, error) {
	if !c.ModuleEnabled {
		return false, nil, nil
	}
	ret, err := c.DeckhouseSettings.ToMap()
	return true, ret, err
}
