package utils

import (
	"encoding/json"
	"fmt"
	ghodssyaml "github.com/ghodss/yaml"
	"github.com/go-yaml/yaml"
)

type ModuleConfig struct {
	ModuleName string
	IsEnabled  bool
	Values     map[interface{}]interface{}
}

func NewModuleConfig(moduleName string, data interface{}) (*ModuleConfig, error) {
	moduleConfig := &ModuleConfig{
		ModuleName: moduleName,
		IsEnabled:  true,
		Values:     make(map[interface{}]interface{}),
	}

	if moduleEnabled, isBool := data.(bool); isBool {
		moduleConfig.IsEnabled = moduleEnabled
	} else {
		moduleValues, moduleValuesOk := data.(map[interface{}]interface{})
		if !moduleValuesOk {
			return nil, fmt.Errorf("required map or bool data, got: %v", data)
		}

		formattedValues, err := FormatValues(moduleValues)
		if err != nil {
			return nil, err
		}
		moduleConfig.Values = formattedValues
	}

	return moduleConfig, nil
}

func FormatValues(values map[interface{}]interface{}) (map[interface{}]interface{}, error) {
	yamlDoc, err := yaml.Marshal(values)
	if err != nil {
		return nil, err
	}

	jsonDoc, err := ghodssyaml.YAMLToJSON(yamlDoc)
	if err != nil {
		return nil, err
	}

	jsonValues := make(map[string]interface{})
	if err := json.Unmarshal(jsonDoc, &jsonValues); err != nil {
		return nil, err
	}

	resValues := JsonValuesToValues(jsonValues)

	return resValues, nil
}

func JsonValuesToValues(jsonValues map[string]interface{}) map[interface{}]interface{} {
	values := make(map[interface{}]interface{})
	for key, value := range jsonValues {
		values[key] = value
	}
	return values
}

func ValuesToJsonValues(values map[interface{}]interface{}) (map[string]interface{}, error) {
	jsonValues := make(map[string]interface{})
	for key, value := range values {
		stringKey, ok := key.(string)
		if ok {
			jsonValues[stringKey] = value
		} else {
			return nil, fmt.Errorf("function ValuesToJsonValues failed: unexpected key `%v`", key)
		}
	}
	return jsonValues, nil
}
