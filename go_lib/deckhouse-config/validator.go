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
	IsConverted bool
	Settings    map[string]interface{}
	Version     int

	// true if spec.settings can be converted and validated.
	HasVersionedSettings bool

	ValidateCRWarning string
	ValidateCRError   string
	ConversionError   string
	ValidationError   string
}

func (v ValidationResult) HasError() bool {
	return v.ConversionError != "" || v.ValidateCRError != "" || v.ValidationError != ""
}

func (v ValidationResult) Error() string {
	errs := make([]string, 0)
	if v.ValidateCRError != "" {
		errs = append(errs, v.ValidateCRError)
	}
	if v.ConversionError != "" {
		errs = append(errs, v.ConversionError)
	}
	if v.ValidationError != "" {
		errs = append(errs, v.ValidationError)
	}
	return strings.Join(errs, ", ")
}

// ValidateCR checks if ModuleConfig resource is well-formed.
func (c *ConfigValidator) ValidateCR(cfg *d8cfg_v1alpha1.ModuleConfig) ValidationResult {
	result := ValidationResult{}

	if cfg.Spec.Version == 0 {
		// Resource is not valid when spec.settings are specified without version.
		if cfg.Spec.Settings != nil {
			result.ValidateCRError = "spec.version is required when spec.settings are specified"
		}
		// Resource is valid without spec.version and spec.settings.
		return result
	}

	// Can run conversions and validations if spec.version and spec.settings are specified.
	if cfg.Spec.Settings == nil {
		// Warn about spec.version without spec.settings.
		result.ValidateCRWarning = "spec.version is redundant without spec.settings"
	} else {
		// Resource has both spec.settings and spec.version — it is ok to convert and validate with OpenAPI schema.
		result.HasVersionedSettings = true
	}

	// Check if there is registered conversion for the version and if the version is the latest.
	chain := conversion.Registry().Chain(cfg.GetName())
	latestVer := chain.LatestVersion()

	// Check if version is unknown.
	if !chain.IsKnownVersion(cfg.Spec.Version) {
		previousVersions := concatIntList(chain.PreviousVersionsList())
		if previousVersions != "" {
			previousVersions = fmt.Sprintf(", or one of previous versions: %s", previousVersions)
		}

		msg := fmt.Sprintf("spec.version=%d is unsupported. Use latest version %d%s", cfg.Spec.Version, latestVer, previousVersions)
		if result.HasVersionedSettings {
			// Error if spec.settings are specified. Can't start conversions for such configuration.
			result.ValidateCRError = msg
		} else {
			// Warning if there are no spec.settings.
			result.ValidateCRWarning = msg
		}
		return result
	}

	// Warning if version is not the latest.
	versionMsg := ""
	if cfg.Spec.Version != latestVer {
		versionMsg = fmt.Sprintf("spec.version=%d is obsolete. Please migrate spec.settings to the latest version %d", cfg.Spec.Version, latestVer)
	}
	result.ValidateCRWarning = versionMsg
	return result
}

// ConvertToLatest checks if ModuleConfig resource is well-formed and runs conversions for spec.settings is needed.
func (c *ConfigValidator) ConvertToLatest(cfg *d8cfg_v1alpha1.ModuleConfig) ValidationResult {
	result := c.ValidateCR(cfg)
	if result.HasError() || !result.HasVersionedSettings {
		return result
	}

	// Run registered conversions if version is not the latest.
	result.Settings = cfg.Spec.Settings
	chain := conversion.Registry().Chain(cfg.GetName())
	if chain.LatestVersion() != cfg.Spec.Version {
		newVersion, newSettings, err := chain.ConvertToLatest(cfg.Spec.Version, cfg.Spec.Settings)
		if err != nil {
			result.ConversionError = fmt.Sprintf("spec.settings conversion from version %d to %d: %v", cfg.Spec.Version, chain.LatestVersion(), err)
			return result
		}
		// Clear settings and version if settings convert to an empty object.
		if len(newSettings) == 0 {
			newSettings = nil
			newVersion = 0
		}
		result.Settings = newSettings
		result.Version = newVersion
		result.IsConverted = true
	}

	return result
}

// Validate checks ModuleConfig resource:
// - check if resource is well-formed
// - runs conversions for spec.settings is needed
// - use OpenAPI schema defined in related config-values.yaml file to validate converted spec.settings.
func (c *ConfigValidator) Validate(cfg *d8cfg_v1alpha1.ModuleConfig) ValidationResult {
	result := c.ConvertToLatest(cfg)
	if result.HasError() {
		return result
	}

	err := c.validateSettings(cfg.GetName(), result.Settings)
	if err != nil {
		convMsg := ""
		if result.IsConverted {
			convMsg = fmt.Sprintf(" converted to %d", result.Version)
		}
		result.ValidationError = fmt.Sprintf("spec.settings are not valid (version %d%s): %v", cfg.Spec.Version, convMsg, cleanupMultilineError(err))
	}

	return result
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

func cleanupMultilineError(err error) string {
	if err == nil {
		return ""
	}
	parts := strings.Split(err.Error(), "\n")
	buf := strings.Builder{}
	for _, part := range parts {
		buf.WriteString(" ")
		buf.WriteString(strings.TrimSpace(part))
	}
	return buf.String()
}
