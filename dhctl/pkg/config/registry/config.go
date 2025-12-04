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
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/module-config"
)

type Config struct {
	Settings          ModeSettings
	DeckhouseSettings module_config.DeckhouseSettings
	ModuleEnabled     bool
}

func (c *Config) FromDefault(
	cri constant.CRIType,
) error {
	userRegistrySettings := module_config.RegistrySettings{}
	return c.FromRegistrySettings(
		userRegistrySettings,
		cri,
	)
}

func (c *Config) FromRegistrySettings(
	userRegistrySettings module_config.RegistrySettings,
	cri constant.CRIType,
) error {
	userSettings := module_config.DeckhouseSettings{
		Mode:      constant.ModeUnmanaged,
		Unmanaged: &userRegistrySettings,
	}

	moduleEnabled := constant.ModuleEnabled(cri)
	if moduleEnabled {
		userSettings = module_config.DeckhouseSettings{
			Mode:   constant.ModeDirect,
			Direct: &userRegistrySettings,
		}
	}
	return c.FromDeckhouseSettings(
		userSettings,
		cri,
	)
}

func (c *Config) FromDeckhouseSettings(
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
