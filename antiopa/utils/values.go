package utils

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/evanphx/json-patch"
	ghodssyaml "github.com/ghodss/yaml"
	"github.com/go-yaml/yaml"
	"strconv"
	"strings"
)

type Values map[string]interface{}

type ModuleConfig struct {
	ModuleName string
	IsEnabled  bool
	Values     Values
}

func NewModuleConfigByYamlData(moduleName string, data []byte) (*ModuleConfig, error) {
	b, err := strconv.ParseBool(strings.TrimSpace(string(data)))
	if err != nil {
		var res map[interface{}]interface{}
		err = yaml.Unmarshal(data, &res)

		if err != nil {
			return nil, fmt.Errorf("unsupported value '%s': %s", string(data), err)
		}

		return NewModuleConfig(moduleName, res)
	} else {
		return NewModuleConfig(moduleName, b)
	}
}

func NewModuleConfig(moduleName string, data interface{}) (*ModuleConfig, error) {
	moduleConfig := &ModuleConfig{
		ModuleName: moduleName,
		IsEnabled:  true,
		Values:     make(Values),
	}

	if moduleEnabled, isBool := data.(bool); isBool {
		moduleConfig.IsEnabled = moduleEnabled
	} else {
		moduleValues, moduleValuesOk := data.(map[interface{}]interface{})
		if !moduleValuesOk {
			return nil, fmt.Errorf("required map or bool data, got: %v", reflect.TypeOf(data))
		}

		formattedValues, err := FormatValues(moduleValues)
		if err != nil {
			return nil, err
		}
		moduleConfig.Values = formattedValues
	}

	return moduleConfig, nil
}

func FormatValues(someValues map[interface{}]interface{}) (Values, error) {
	yamlDoc, err := yaml.Marshal(someValues)
	if err != nil {
		return nil, err
	}

	jsonDoc, err := ghodssyaml.YAMLToJSON(yamlDoc)
	if err != nil {
		return nil, err
	}

	values := make(Values)
	if err := json.Unmarshal(jsonDoc, &values); err != nil {
		return nil, err
	}

	return values, nil
}

func ApplyJsonMergeAndPatch(values Values, valuesToMerge Values, patch *jsonpatch.Patch) (Values, bool, error) {
	var err error
	resValues := values

	if valuesToMerge != nil {
		resValues = MergeValues(resValues, valuesToMerge)
	}

	if patch != nil {
		if resValues, err = applyJsonPatch(resValues, patch); err != nil {
			return nil, false, err
		}
	}

	valuesChanged := !reflect.DeepEqual(values, resValues)

	return resValues, valuesChanged, nil
}

func applyJsonPatch(values Values, patch *jsonpatch.Patch) (Values, error) {
	jsonDoc, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	resJsonDoc, err := patch.Apply(jsonDoc)
	if err != nil {
		return nil, err
	}

	resValues := make(Values)
	if err = json.Unmarshal(resJsonDoc, &resValues); err != nil {
		return nil, err
	}

	return resValues, nil
}

func MergeValues(values ...Values) Values {
	var deepMergeArgs []map[interface{}]interface{}
	for _, v := range values {
		deepMergeArgs = append(deepMergeArgs, valuesToDeepMergeArg(v))
	}

	res := DeepMerge(deepMergeArgs...)
	resValues := deepMergeResToValues(res)

	return resValues
}

func valuesToDeepMergeArg(values Values) map[interface{}]interface{} {
	arg := make(map[interface{}]interface{})
	for key, value := range values {
		arg[key] = value
	}
	return arg
}

func deepMergeResToValues(res map[interface{}]interface{}) Values {
	values := make(Values)
	for key, value := range res {
		values[key.(string)] = value
	}
	return values
}
