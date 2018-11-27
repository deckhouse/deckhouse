package utils

import (
	"fmt"

	"github.com/go-yaml/yaml"
)

type ModuleConfig struct {
	ModuleName string
	IsEnabled  bool
	Values     Values
	IsUpdated  bool
}

func (mc ModuleConfig) String() string {
	return fmt.Sprintf("Module(Name=%s IsEnabled=%v IsUpdated=%v Values:\n%s)", mc.ModuleName, mc.IsEnabled, mc.IsUpdated, ValuesToString(mc.Values))
}

func NewModuleConfig(moduleName string) *ModuleConfig {
	return &ModuleConfig{
		ModuleName: moduleName,
		IsEnabled:  true,
		Values:     make(Values),
	}
}

func (mc *ModuleConfig) WithEnabled(v bool) *ModuleConfig {
	mc.IsEnabled = v
	return mc
}

func (mc *ModuleConfig) WithUpdated(v bool) *ModuleConfig {
	mc.IsUpdated = v
	return mc
}

// WithValues load module config from a map.
//
// Values for module in `values` map are addressed by a key.
// This key should be produced with ModuleNameToValuesKey.
//
// Module is enabled if key not exists in values.
func (mc *ModuleConfig) WithValues(values map[interface{}]interface{}) (*ModuleConfig, error) {
	moduleValuesKey := ModuleNameToValuesKey(mc.ModuleName)

	if moduleValuesData, hasModuleData := values[moduleValuesKey]; hasModuleData {
		switch v := moduleValuesData.(type) {
		case bool:
			mc.IsEnabled = v
		case map[interface{}]interface{}, []interface{}:
			data := map[interface{}]interface{}{moduleValuesKey: v}

			values, err := NewValues(data)
			if err != nil {
				return nil, err
			}
			mc.IsEnabled = true
			mc.Values = values

		default:
			return nil, fmt.Errorf("module config should be bool, array or map, got: %#v", moduleValuesData)
		}
	} else {
		mc.IsEnabled = true
	}

	return mc, nil
}

// FromYaml load module config from a yaml string.
func (mc *ModuleConfig) FromYaml(yamlString []byte) (*ModuleConfig, error) {
	var values map[interface{}]interface{}

	err := yaml.Unmarshal(yamlString, &values)
	if err != nil {
		return nil, fmt.Errorf("module %s has errors in yaml: %s\n%s", mc.ModuleName, err, string(yamlString))
	}

	return mc.WithValues(values)
}
