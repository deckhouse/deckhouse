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
	registrySettings := module_config.RegistrySettings{}
	registrySettings.Correct()
	return c.FromRegistrySettings(registrySettings, cri)
}

func (c *Config) FromRegistrySettings(
	registrySettings module_config.RegistrySettings,
	cri constant.CRIType,
) error {
	// TODO:
	// moduleEnabled := moduleEnabled(cri)
	// if moduleEnabled {
	// 	return c.fromDeckhouseSettings(module_config.DeckhouseSettings{
	// 		Mode:   constant.ModeDirect,
	// 		Direct: &registrySettings,
	// 	}, cri)
	// }
	return c.FromDeckhouseSettings(module_config.DeckhouseSettings{
		Mode:      constant.ModeUnmanaged,
		Unmanaged: &registrySettings,
	}, cri)
}

func (c *Config) FromDeckhouseSettings(
	deckhouseSettings module_config.DeckhouseSettings,
	cri constant.CRIType,
) error {
	// Check if module can be enabled with current CRI
	moduleEnabled := moduleEnabled(cri)
	moduleRequired := moduleRequired(deckhouseSettings.Mode)
	if moduleRequired && !moduleEnabled {
		return fmt.Errorf(
			"registry mode '%s' is not supported with defaultCRI:'%s'. "+
				"Please switch to 'Unmanaged' registry mode or use one of defaultCRI: %v",
			deckhouseSettings.Mode,
			cri,
			constant.ModuleEnabledCRI,
		)
	}

	deckhouseSettings.Correct()
	if err := deckhouseSettings.Validate(); err != nil {
		return fmt.Errorf("validate registry settings: %w", err)
	}
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

func moduleEnabled(cri constant.CRIType) bool {
	return slices.Contains(constant.ModuleEnabledCRI, cri)
}

func moduleRequired(mode constant.ModeType) bool {
	return slices.Contains(constant.ModesRequiringModule, mode)
}
