package utils

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/evanphx/json-patch"
	ghodssyaml "github.com/ghodss/yaml"
	"github.com/go-yaml/yaml"
	"github.com/segmentio/go-camelcase"
)

type Values map[string]interface{}

type ModuleConfig struct {
	ModuleName string
	IsEnabled  bool
	Values     Values
}

func ModuleNameToValuesKey(moduleName string) string {
	return camelcase.Camelcase(moduleName)
}

func ModuleNameFromValuesKey(moduleValuesKey string) string {
	b := make([]byte, 0, 64)
	l := len(moduleValuesKey)
	i := 0

	for i < l {
		c := moduleValuesKey[i]

		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				// append dash module name parts delimiter
				b = append(b, '-')
			}
			// append lowercased symbol
			b = append(b, c+('a'-'A'))
		} else if c >= '0' && c <= '9' {
			if i > 0 {
				// append dash module name parts delimiter
				b = append(b, '-')
			}
			b = append(b, c)
		} else {
			b = append(b, c)
		}

		i++
	}

	return string(b)
}

func NewModuleConfigByValuesYamlData(moduleName string, data []byte) (*ModuleConfig, error) {
	var values map[interface{}]interface{}

	err := yaml.Unmarshal(data, &values)
	if err != nil {
		return nil, fmt.Errorf("bad module %s values data: %s\n%s", moduleName, err, string(data))
	}

	return NewModuleConfig(moduleName, values)
}

func NewModuleConfigByModuleValuesYamlData(moduleName string, moduleData []byte) (*ModuleConfig, error) {
	var valuesAtModuleKey interface{}

	err := yaml.Unmarshal(moduleData, &valuesAtModuleKey)
	if err != nil {
		return nil, fmt.Errorf("bad module %s configmap values data: %s\n%s", moduleName, err, string(moduleData))
	}

	moduleValues := map[interface{}]interface{}{ModuleNameToValuesKey(moduleName): valuesAtModuleKey}

	return NewModuleConfig(moduleName, moduleValues)
}

func NewModuleConfig(moduleName string, data map[interface{}]interface{}) (*ModuleConfig, error) {
	moduleConfig := &ModuleConfig{
		ModuleName: moduleName,
		IsEnabled:  true,
		Values:     make(Values),
	}

	moduleValuesKey := ModuleNameToValuesKey(moduleName)

	if moduleValuesData, hasModuleData := data[moduleValuesKey]; hasModuleData {
		if moduleEnabled, isBool := moduleValuesData.(bool); isBool {
			moduleConfig.IsEnabled = moduleEnabled
		} else {
			moduleValues, moduleValuesOk := moduleValuesData.(map[interface{}]interface{})
			if !moduleValuesOk {
				return nil, fmt.Errorf("required map or bool data, got: %#v", moduleValuesData)
			}

			values := map[interface{}]interface{}{moduleValuesKey: moduleValues}

			formattedValues, err := FormatValues(values)
			if err != nil {
				panic(err)
			}
			moduleConfig.Values = formattedValues
		}
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

	resValues, err := FormatValues(res)
	if err != nil {
		panic(err)
	}

	return resValues
}

func valuesToDeepMergeArg(values Values) map[interface{}]interface{} {
	arg := make(map[interface{}]interface{})
	for key, value := range values {
		arg[key] = value
	}
	return arg
}

func ValuesToString(values Values) string {
	return YamlToString(values)
}
