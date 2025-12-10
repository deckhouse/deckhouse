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

type settingsData = map[string]any

type Config struct {
	Settings          ModeSettings
	DeckhouseSettings module_config.DeckhouseSettings
	LegacyMode        bool
}

// UseDefault configures the registry with default CE settings.
// When no registry configuration is provided:
// - If Direct mode is supported, uses Direct mode
// - Otherwise, falls back to Unmanaged mode
// - All parameters are populated with default values for the CE registry
func (c *Config) UseDefault(criSupported bool) error {
	if criSupported {
		settings := module_config.DeckhouseSettings{
			Mode:   constant.ModeDirect,
			Direct: &module_config.RegistrySettings{},
		}
		return c.process(settings, false)
	}

	settings := module_config.DeckhouseSettings{
		Mode:      constant.ModeUnmanaged,
		Unmanaged: &module_config.RegistrySettings{},
	}
	return c.process(settings, true)
}

// UseInitConfig configures registry using legacy initConfiguration.
// Note: This method maintains backward compatibility and only supports Unmanaged legacy mode.
func (c *Config) UseInitConfig(initConfig init_config.Config) error {
	registrySettings, err := initConfig.ToRegistrySettings()
	if err != nil {
		return fmt.Errorf("get registry settings: %w", err)
	}

	settings := module_config.DeckhouseSettings{
		Mode:      constant.ModeUnmanaged,
		Unmanaged: &registrySettings,
	}
	return c.process(settings, true)
}

// UseDeckhouseSettings configures registry using deckhouse ModuleConfig settings.
// The operation mode (Direct/Unmanaged) is determined from the user configuration.
func (c *Config) UseDeckhouseSettings(userSettings module_config.DeckhouseSettings) error {
	return c.process(userSettings, false)
}

func (c *Config) process(userSettings module_config.DeckhouseSettings, legacyMode bool) error {
	// Prepare settings
	var deckhouseSettings module_config.DeckhouseSettings
	deckhouseSettings.ApplySettings(userSettings)

	// Validate
	if err := deckhouseSettings.Validate(); err != nil {
		return fmt.Errorf("validate registry settings: %w", err)
	}

	// This error checks whether the registry can be started in legacy mode.
	// The error is needed to check the tests of the UseInitConfig and UseDefault methods.
	if legacyMode && constant.ModuleRequired(deckhouseSettings.Mode) {
		return fmt.Errorf(
			"internal error: cannot run registry in legacy mode with registry mode: '%s'.",
			deckhouseSettings.Mode,
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
		LegacyMode:        legacyMode,
	}

	return nil
}

func (c *Config) Manifest() *ManifestBuilder {
	return newManifestBuilder(c.Settings.ToModel(), c.LegacyMode)
}

func (c *Config) DeckhouseSettingsToMap() (exist bool, settings settingsData, err error) {
	if c.LegacyMode {
		return false, nil, nil
	}

	ret, err := c.DeckhouseSettings.ToMap()
	return true, ret, err
}
