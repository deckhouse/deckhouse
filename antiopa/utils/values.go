package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"

	"github.com/evanphx/json-patch"
	ghodssyaml "github.com/ghodss/yaml"
	"github.com/go-yaml/yaml"
	"github.com/peterbourgon/mergemap"
	"github.com/segmentio/go-camelcase"
)

type Values map[string]interface{}

type ValuesPatch struct {
	JsonPatch  jsonpatch.Patch
	Operations []*ValuesPatchOperation
}

type ValuesPatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

func (op *ValuesPatchOperation) ToString() string {
	data, err := json.Marshal(op)
	if err != nil {
		panic(err)
	}
	return string(data)
}

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

func MustValuesPatch(res *ValuesPatch, err error) *ValuesPatch {
	if err != nil {
		panic(err)
	}
	return res
}

func ValuesPatchFromBytes(data []byte) (*ValuesPatch, error) {
	patch, err := jsonpatch.DecodePatch(data)
	if err != nil {
		return nil, fmt.Errorf("bad json-patch data: %s\n%s", err, string(data))
	}

	var operations []*ValuesPatchOperation
	if err := json.Unmarshal(data, &operations); err != nil {
		return nil, fmt.Errorf("bad json-patch data: %s\n%s", err, string(data))
	}

	return &ValuesPatch{JsonPatch: patch, Operations: operations}, nil
}

func ValuesPatchFromFile(filePath string) (*ValuesPatch, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	return ValuesPatchFromBytes(data)
}

func AppendValuesPatch(valuesPatches []ValuesPatch, newValuesPatch ValuesPatch) []ValuesPatch {
	// FIXME: patches compaction
	return append(valuesPatches, newValuesPatch)
}

func ApplyValuesPatch(values Values, valuesPatch ValuesPatch) (Values, bool, error) {
	var err error
	resValues := values

	if resValues, err = ApplyJsonPatchToValues(resValues, valuesPatch.JsonPatch); err != nil {
		return nil, false, err
	}

	valuesChanged := !reflect.DeepEqual(values, resValues)

	return resValues, valuesChanged, nil
}

func ApplyJsonPatchToValues(values Values, patch jsonpatch.Patch) (Values, error) {
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
	res := make(Values)

	for _, v := range values {
		res = mergemap.Merge(res, v)
	}

	return res
}

func ValuesToString(values Values) string {
	return YamlToString(values)
}

func MustDump(data []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return data
}

func DumpValuesYaml(values Values) ([]byte, error) {
	return yaml.Marshal(values)
}

func DumpValuesJson(values Values) ([]byte, error) {
	return json.Marshal(values)
}
