package utils

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestModuleConfig(t *testing.T) {
	var config *ModuleConfig
	var err error

	config, err = NewModuleConfig("test-module", 1234)
	if err == nil {
		t.Errorf("Expected error, got ModuleConfig: %v", config)
	} else if !strings.HasPrefix(err.Error(), "required map or bool data") {
		t.Errorf("Got unexpected error: %s", err)
	}

	config, err = NewModuleConfig("test-module", false)
	if err != nil {
		t.Error(err)
	}
	if config.IsEnabled {
		t.Errorf("Expected module to be disabled, got: %v", config)
	}

	config, err = NewModuleConfig("test-module", true)
	if err != nil {
		t.Error(err)
	}
	if !config.IsEnabled {
		t.Errorf("Expected module to be enabled")
	}

	inputData := map[interface{}]interface{}{
		"hello": "world", 4: "123", 5: 5,
		"aaa": map[interface{}]interface{}{"no": []interface{}{"one", "two", "three"}},
	}
	expectedData := Values{
		"hello": "world", "4": "123", "5": 5.0,
		"aaa": map[string]interface{}{"no": []interface{}{"one", "two", "three"}},
	}

	config, err = NewModuleConfig("test-module", inputData)
	if err != nil {
		t.Error(err)
	}
	if !config.IsEnabled {
		t.Errorf("Expected module to be enabled")
	}

	if !reflect.DeepEqual(config.Values, expectedData) {
		t.Errorf("Got unexpected config values: %+v", config.Values)
	}
}

func TestNewModuleConfigByYamlData(t *testing.T) {
	expectedData := Values{"a": 1.0, "b": 2.0}
	config, err := NewModuleConfigByYamlData("test-module", []byte("a: 1\nb: 2"))
	if err != nil {
		t.Error(err)
	}
	if !config.IsEnabled {
		t.Errorf("Expected module to be enabled")
	}
	if !reflect.DeepEqual(config.Values, expectedData) {
		t.Errorf("Got unexpected config values: %+v", config.Values)
	}

	config, err = NewModuleConfigByYamlData("test-module", []byte("false\t\n"))
	if err != nil {
		t.Error(err)
	}
	if config.IsEnabled {
		t.Errorf("Expected module to be disabled")
	}

	config, err = NewModuleConfigByYamlData("test-module", []byte("falsee"))
	if !strings.HasPrefix(err.Error(), "unsupported value") {
		t.Errorf("Got unexpected error: %s", err.Error())
	}
}

func TestMergeValues(t *testing.T) {
	expectations := []struct {
		testName       string
		values1        Values
		values2        Values
		expectedValues Values
	}{
		{
			"simple",
			Values{"a": 1, "b": 2},
			Values{"b": 3, "c": 4},
			Values{"a": 1, "b": 3, "c": 4},
		},
		{
			"array",
			Values{"a": []interface{}{1}},
			Values{"a": []interface{}{2}},
			Values{"a": []interface{}{1, 2}},
		},
		{
			"map",
			Values{"a": map[interface{}]interface{}{"a": 1, "b": 2}},
			Values{"a": map[interface{}]interface{}{"b": 3, "c": 4}},
			Values{"a": map[interface{}]interface{}{"a": 1, "b": 3, "c": 4}},
		},
		{
			"mixed-map",
			Values{"a": map[interface{}]interface{}{1: "a", 2: "b"}},
			Values{"a": map[interface{}]interface{}{"1": "c"}},
			Values{"a": map[interface{}]interface{}{1: "a", 2: "b", "1": "c"}},
		},
	}

	for _, expectation := range expectations {
		t.Run(expectation.testName, func(t *testing.T) {
			values := MergeValues(expectation.values1, expectation.values2)

			if !reflect.DeepEqual(expectation.expectedValues, values) {
				t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.expectedValues, values)
			}
		})
	}
}

func expectStringToEqual(str string, expected string) error {
	if str != expected {
		return fmt.Errorf("Expected '%s' string, got '%s'", expected, str)
	}
	return nil
}

func TestModuleNameConversions(t *testing.T) {
	var err error

	for _, strs := range [][]string{
		[]string{"module-1", "module1"},
		[]string{"prometheus", "prometheus"},
		[]string{"prometheus-operator", "prometheusOperator"},
		[]string{"hello-world-module", "helloWorldModule"},
		[]string{"cert-manager-crd", "certManagerCrd"},
	} {
		moduleName := strs[0]
		moduleValuesKey := strs[1]

		err = expectStringToEqual(ModuleNameToValuesKey(moduleName), moduleValuesKey)
		if err != nil {
			t.Error(err)
		}

		err = expectStringToEqual(ModuleNameFromValuesKey(moduleValuesKey), moduleName)
		if err != nil {
			t.Error(err)
		}
	}
}
