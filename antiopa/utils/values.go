package utils

import (
	"fmt"
)

type ModuleConfig struct {
	ModuleName string
	IsEnabled  bool
	Values     map[interface{}]interface{}
}

func NewModuleConfig(moduleName string, valuesData interface{}) (*ModuleConfig, error) {
	moduleConfig := &ModuleConfig{
		ModuleName: moduleName,
		IsEnabled:  true,
		Values:     make(map[interface{}]interface{}),
	}

	if moduleEnabled, isBool := valuesData.(bool); isBool {
		moduleConfig.IsEnabled = moduleEnabled
	} else {
		moduleValues, moduleValuesOk := valuesData.(map[interface{}]interface{})
		if !moduleValuesOk {
			return nil, fmt.Errorf("expected map or bool, got: %v", valuesData)
		}
		moduleConfig.Values = moduleValues
	}

	return moduleConfig, nil
}

func FormatValues(map[interface{}]interface{}) (map[interface{}]interface{}, error) { return nil, nil }
