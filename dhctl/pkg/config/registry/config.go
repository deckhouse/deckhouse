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
	init_config "github.com/deckhouse/deckhouse/go_lib/registry/models/initconfig"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"
)

func errDuplicateConfig() error {
	return fmt.Errorf("duplicate registry configuration detected: " +
		"registry is configured in both 'initConfiguration.deckhouse' " +
		"and 'moduleConfig/deckhouse.spec.settings.registry'. " +
		"Please specify registry settings in only one location.")
}

func errUnsupportedCRI(cri constant.CRIType) error {
	return fmt.Errorf(
		"registry module cannot be started with defaultCRI '%s'. "+
			"Please either configure registry in 'initConfiguration.deckhouse', "+
			"or use a supported defaultCRI type with the existing configuration in "+
			"'moduleConfig/deckhouse.spec.settings.registry'. Supported CRI types: %v",
		cri,
		constant.SupportedCRI,
	)
}

func errNonStaticClusterMode(mode constant.ModeType) error {
	return fmt.Errorf(
		"bootstrap with registry mode '%s' is supported only in static cluster. "+
			"Please use one of the supported bootstrap modes for non-static cluster: %v",
		mode,
		[]constant.ModeType{
			constant.ModeUnmanaged,
			constant.ModeDirect,
		},
	)
}

// IsLocalBootstrapMode returns true when the bootstrap registry mode is Local.
// It is used only for preliminary registry information retrieval.
func IsLocalBootstrapMode(
	initConfig *init_config.Config,
	deckhouseSettings *module_config.DeckhouseSettings,
) (bool, error) {
	switch {
	case initConfig != nil && deckhouseSettings != nil:
		return false, errDuplicateConfig()

	case deckhouseSettings != nil:
		return deckhouseSettings.Mode == constant.ModeLocal, nil
	}
	return false, nil
}

// BootstrapRemoteData returns the remote registry Data derived from the provided configuration.
// It is used only for preliminary registry information retrieval.
func BootstrapRemoteData(
	initConfig *init_config.Config,
	deckhouseSettings *module_config.DeckhouseSettings,
) (Data, error) {
	var config Config

	switch {
	case initConfig != nil && deckhouseSettings != nil:
		return Data{}, errDuplicateConfig()

	case deckhouseSettings != nil:
		if err := config.useDeckhouseSettings(*deckhouseSettings); err != nil {
			return Data{}, fmt.Errorf("get registry settings from 'moduleConfig/deckhouse': %w", err)
		}

	case initConfig != nil:
		if err := config.useInitConfig(*initConfig); err != nil {
			return Data{}, fmt.Errorf("get registry settings from 'initConfiguration': %w", err)
		}

	default:
		// criSupported=false selects legacy Unmanaged mode with default registry parameters.
		if err := config.useDefault(false); err != nil {
			return Data{}, fmt.Errorf("get default registry settings: %w", err)
		}
	}
	return config.Settings.RemoteData, nil
}

// BootstrapConfig builds a full registry Config from the provided configuration sources.
func BootstrapConfig(
	initConfig *init_config.Config,
	deckhouseSettings *module_config.DeckhouseSettings,
	defaultCRI constant.CRIType,
	isStatic bool,
) (Config, error) {
	criSupported := constant.IsCRISupported(defaultCRI)

	switch {
	case initConfig != nil && deckhouseSettings != nil:
		return Config{}, errDuplicateConfig()

	case deckhouseSettings != nil:
		if !criSupported {
			return Config{}, errUnsupportedCRI(defaultCRI)
		}

		switch deckhouseSettings.Mode {
		case constant.ModeProxy, constant.ModeLocal:
			if !isStatic {
				return Config{}, errNonStaticClusterMode(deckhouseSettings.Mode)
			}
		}

		config := Config{}

		if err := config.useDeckhouseSettings(*deckhouseSettings); err != nil {
			return config, fmt.Errorf("get registry settings from 'moduleConfig/deckhouse': %w", err)
		}
		return config, nil

	case initConfig != nil:
		config := Config{}

		if err := config.useInitConfig(*initConfig); err != nil {
			return config, fmt.Errorf("get registry settings from 'initConfiguration': %w", err)
		}
		return config, nil

	default:
		config := Config{}

		if err := config.useDefault(criSupported); err != nil {
			return config, fmt.Errorf("get default registry settings: %w", err)
		}
		return config, nil
	}
}

type Config struct {
	Settings          ModeSettings
	DeckhouseSettings module_config.DeckhouseSettings
	LegacyMode        bool
}

// useDefault configures the registry with default CE settings.
// When no registry configuration is provided:
// - If Direct mode is supported, uses Direct mode
// - Otherwise, falls back to Unmanaged mode
// - All parameters are populated with default values for the CE registry
func (c *Config) useDefault(criSupported bool) error {
	var settings module_config.DeckhouseSettings

	if criSupported {
		settings = module_config.New(constant.ModeDirect)
	} else {
		settings = module_config.New(constant.ModeUnmanaged)
	}
	return c.Process(settings, !criSupported)
}

// useInitConfig configures registry using legacy initConfiguration.
// Note: This method maintains backward compatibility and only supports Unmanaged legacy mode.
func (c *Config) useInitConfig(userConfig init_config.Config) error {
	// Prepare config
	initConfig := init_config.
		New().
		Merge(&userConfig)

	// Convert to registry settings
	registrySettings, err := initConfig.ToRegistrySettings()
	if err != nil {
		return fmt.Errorf("get registry settings: %w", err)
	}

	settings := module_config.
		New(constant.ModeUnmanaged).
		Merge(&module_config.DeckhouseSettings{
			Mode:      constant.ModeUnmanaged,
			Unmanaged: &registrySettings,
		})
	return c.Process(settings, true)
}

// useDeckhouseSettings configures registry using deckhouse ModuleConfig settings.
// The operation mode (Direct/Unmanaged) is determined from the user configuration.
func (c *Config) useDeckhouseSettings(userSettings module_config.DeckhouseSettings) error {
	settings := module_config.
		New(userSettings.Mode).
		Merge(&userSettings)
	return c.Process(settings, false)
}

func (c *Config) Process(deckhouseSettings module_config.DeckhouseSettings, legacyMode bool) error {
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
	modeSettings, err := newModeSettings(deckhouseSettings)
	if err != nil {
		return fmt.Errorf("get registry mode settings: %w", err)
	}

	*c = Config{
		Settings:          modeSettings,
		DeckhouseSettings: deckhouseSettings,
		LegacyMode:        legacyMode,
	}

	return nil
}

// Manifest creates a ManifestBuilder instance for generating configuration manifests.
func (c *Config) Manifest() *ManifestBuilder {
	return newManifestBuilder(c.Settings.ToModel(), c.LegacyMode)
}

// DeepCopyInto copies the receiver into out.
func (c *Config) DeepCopyInto(out *Config) {
	*out = *c
	c.Settings.DeepCopyInto(&out.Settings)
	c.DeckhouseSettings.DeepCopyInto(&out.DeckhouseSettings)
}

// DeepCopy returns a deep copy of the receiver.
func (c *Config) DeepCopy() *Config {
	if c == nil {
		return nil
	}
	out := new(Config)
	c.DeepCopyInto(out)
	return out
}
