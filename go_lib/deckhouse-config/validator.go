/*
Copyright 2022 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package deckhouse_config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/flant/addon-operator/pkg/utils"

	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
	d8cfg_v1alpha1 "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/v1alpha1"
)

// ConfigValidator is a validator for values in ModuleConfig.
type ConfigValidator struct {
	valuesValidator ValuesValidator
}

func NewConfigValidator(valuesValidator ValuesValidator) *ConfigValidator {
	return &ConfigValidator{
		valuesValidator: valuesValidator,
	}
}

// ValuesValidator is a part of ValuesValidator from addon-operator with needed
// methods to validate config values.
type ValuesValidator interface {
	ValidateGlobalConfigValues(values utils.Values) error
	ValidateModuleConfigValues(moduleName string, values utils.Values) error
}

type ValidationResult struct {
	IsConverted    bool
	Settings       map[string]interface{}
	Version        int
	VersionWarning string
}

// Validate checks version value if spec.settings value is not empty,
// then converts spec.settings to the latest version and finally validates
// spec.settings using OpenAPI schema defined in related config-values.yaml file.
func (c *ConfigValidator) Validate(cfg *d8cfg_v1alpha1.ModuleConfig) (ValidationResult, error) {
	result := ValidationResult{}

	// It is ok if resource spec is empty or if it has enabled field only.
	if len(cfg.Spec.Settings) == 0 && cfg.Spec.Version == 0 {
		return result, nil
	}
	if cfg.Spec.Settings == nil && cfg.Spec.Version > 0 {
		return result, fmt.Errorf("spec.version is forbidden when settings are not specified")
	}

	// Settings are present, check version using conversion chain.
	chain := conversion.Registry().Chain(cfg.GetName())
	latestVer := chain.LatestVersion()

	if !chain.IsKnownVersion(cfg.Spec.Version) {
		previousVersions := concatIntList(chain.PreviousVersionsList())
		if previousVersions != "" {
			previousVersions = fmt.Sprintf(", or one of previous versions: %s", previousVersions)
		}

		return result, fmt.Errorf("spec.version=%d is invalid. Use latest version %d%s", cfg.Spec.Version, latestVer, previousVersions)
	}

	versionMsg := ""
	if cfg.Spec.Version != latestVer {
		versionMsg = fmt.Sprintf("spec.version=%d is obsolete. Please migrate settings to the latest version %d", cfg.Spec.Version, latestVer)
	}
	result.VersionWarning = versionMsg

	// Run registered conversions if version is not the latest.
	convMsg := ""
	settings := cfg.Spec.Settings
	if chain.LatestVersion() != cfg.Spec.Version {
		newVersion, newSettings, err := chain.ConvertToLatest(cfg.Spec.Version, cfg.Spec.Settings)
		if err != nil {
			return result, fmt.Errorf("convert '%s' settings from version %d to %d: %v", cfg.GetName(), cfg.Spec.Version, chain.LatestVersion(), err)
		}
		// Clear settings and version if settings convert to an empty object.
		if len(newSettings) == 0 {
			newSettings = nil
			newVersion = 0
		}
		settings = newSettings
		result.Settings = newSettings
		result.Version = newVersion
		result.IsConverted = true
		convMsg = fmt.Sprintf(" converted to %d", newVersion)
	}

	// Ignore validation for empty settings.
	if len(settings) > 0 {
		err := c.validateSettings(cfg.GetName(), settings)
		if err != nil {
			return result, fmt.Errorf("%s settings are not valid (version %d%s): %v", cfg.GetName(), cfg.Spec.Version, convMsg, err)
		}
	}

	return result, nil
}

// validateSettings uses ValuesValidator from ModuleManager instance to validate spec.settings.
// cfgName arg is a kebab-cased name of the ModuleConfig resource.
// cfgSettings is a content of spec.settings.
// (Note: cfgValues map is a map with 'plain values', i.e. without camelCased module name as a root key).
func (c *ConfigValidator) validateSettings(cfgName string, cfgSettings map[string]interface{}) error {
	// Ignore empty validator.
	if c.valuesValidator == nil {
		return nil
	}

	valuesKey := valuesKeyFromObjectName(cfgName)
	values := map[string]interface{}{
		valuesKey: cfgSettings,
	}

	if cfgName == "global" {
		return c.valuesValidator.ValidateGlobalConfigValues(values)
	}

	return c.valuesValidator.ValidateModuleConfigValues(valuesKey, values)
}

func valuesKeyFromObjectName(name string) string {
	if name == "global" {
		return name
	}
	return utils.ModuleNameToValuesKey(name)
}

func concatIntList(items []int) string {
	var buf strings.Builder
	for i, item := range items {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(strconv.Itoa(item))
	}
	return buf.String()
}
