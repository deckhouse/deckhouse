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

	config, err = NewModuleConfig("test-module", map[interface{}]interface{}{"testModule": 1234})
	if err == nil {
		t.Errorf("Expected error, got ModuleConfig: %v", config)
	} else if !strings.HasPrefix(err.Error(), "required map or bool data") {
		t.Errorf("Got unexpected error: %s", err)
	}

	config, err = NewModuleConfig("test-module", map[interface{}]interface{}{"testModule": false})
	if err != nil {
		t.Error(err)
	}
	if config.IsEnabled {
		t.Errorf("Expected module to be disabled, got: %v", config)
	}

	config, err = NewModuleConfig("test-module", map[interface{}]interface{}{"testModule": true})
	if err != nil {
		t.Error(err)
	}
	if !config.IsEnabled {
		t.Errorf("Expected module to be enabled")
	}

	inputData := map[interface{}]interface{}{
		"testModule": map[interface{}]interface{}{
			"hello": "world", 4: "123", 5: 5,
			"aaa": map[interface{}]interface{}{"no": []interface{}{"one", "two", "three"}},
		},
	}
	expectedData := Values{
		"testModule": map[string]interface{}{
			"hello": "world", "4": "123", "5": 5.0,
			"aaa": map[string]interface{}{"no": []interface{}{"one", "two", "three"}},
		},
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

func TestNewModuleConfigByValuesYamlData(t *testing.T) {
	expectedData := Values{
		"testModule": map[string]interface{}{
			"a": 1.0, "b": 2.0,
		},
	}
	config, err := NewModuleConfigByValuesYamlData("test-module", []byte("testModule:\n  a: 1\n  b: 2"))
	if err != nil {
		t.Error(err)
	}
	if !config.IsEnabled {
		t.Errorf("Expected module to be enabled")
	}
	if !reflect.DeepEqual(config.Values, expectedData) {
		t.Errorf("Got unexpected config values: %+v", config.Values)
	}

	config, err = NewModuleConfigByValuesYamlData("test-module", []byte("testModule: false\n"))
	if err != nil {
		t.Error(err)
	}
	if config.IsEnabled {
		t.Errorf("Expected module to be disabled")
	}

	config, err = NewModuleConfigByValuesYamlData("test-module", []byte("testModule: falsee\n"))
	if !strings.HasPrefix(err.Error(), "required map or bool data") {
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
			Values{"a": []interface{}{2}},
		},
		{
			"map",
			Values{"a": map[string]interface{}{"a": 1, "b": 2}},
			Values{"a": map[string]interface{}{"b": 3, "c": 4}},
			Values{"a": map[string]interface{}{"a": 1, "b": 3, "c": 4}},
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
		{"module-1", "module1"},
		{"prometheus", "prometheus"},
		{"prometheus-operator", "prometheusOperator"},
		{"hello-world-module", "helloWorldModule"},
		{"cert-manager-crd", "certManagerCrd"},
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

func TestCompactValuesPatchOperations(t *testing.T) {
	expectations := []struct {
		testName           string
		operations         []*ValuesPatchOperation
		newOperations      []*ValuesPatchOperation
		expectedOperations []*ValuesPatchOperation
	}{
		{
			"path",
			[]*ValuesPatchOperation{
				{
					"add",
					"/a",
					"",
				},
			},
			[]*ValuesPatchOperation{
				{
					"add",
					"/a",
					"",
				},
			},
			nil,
		},
		{
			"subpath",
			[]*ValuesPatchOperation{
				{
					"add",
					"/a/b",
					"",
				},
			},
			[]*ValuesPatchOperation{
				{
					"add",
					"/a",
					"",
				},
			},
			nil,
		},
		{
			"different op",
			[]*ValuesPatchOperation{
				{
					"add",
					"/a",
					"",
				},
			},
			[]*ValuesPatchOperation{
				{
					"delete",
					"/a",
					"",
				},
			},
			[]*ValuesPatchOperation{
				{
					"add",
					"/a",
					"",
				},
			},
		},
		{
			"different path",
			[]*ValuesPatchOperation{
				{
					"add",
					"/a",
					"",
				},
			},
			[]*ValuesPatchOperation{
				{
					"add",
					"/b",
					"",
				},
			},
			[]*ValuesPatchOperation{
				{
					"add",
					"/a",
					"",
				},
			},
		},
		{
			"sample",
			[]*ValuesPatchOperation{
				{
					"add",
					"/a",
					"",
				},
				{
					"add",
					"/a/b",
					"",
				},
				{
					"add",
					"/b",
					"",
				},
				{
					"delete",
					"/c",
					"",
				},
			},
			[]*ValuesPatchOperation{
				{
					"add",
					"/a",
					"",
				},
				{
					"delete",
					"/c",
					"",
				},
				{
					"add",
					"/d",
					"",
				},
			},
			[]*ValuesPatchOperation{
				{
					"add",
					"/b",
					"",
				},
			},
		},
	}

	for _, expectation := range expectations {
		t.Run(expectation.testName, func(t *testing.T) {
			compactOperations := CompactValuesPatchOperations(expectation.operations, expectation.newOperations)

			if !reflect.DeepEqual(expectation.expectedOperations, compactOperations) {
				t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.expectedOperations, compactOperations)
			}
		})
	}
}

// TODO поправить после изменения алгоритма compact
func TestApplyPatch(t *testing.T) {
	t.SkipNow()
	expectations := []struct {
		testName              string
		operations            ValuesPatch
		values                Values
		expectedValues        Values
		expectedValuesChanged bool
	}{
		{
			"path",
			ValuesPatch{
				[]*ValuesPatchOperation{
					{
						"add",
						"/test_key_3",
						"baz",
					},
				},
			},
			Values{
				"test_key_1": "foo",
				"test_key_2": "bar",
			},
			Values{
				"test_key_1": "foo",
				"test_key_2": "bar",
				"test_key_3": "baz",
			},
			true,
		},
		{
			"path",
			ValuesPatch{
				[]*ValuesPatchOperation{
					{
						"remove",
						"/test_key_3",
						"baz",
					},
				},
			},
			Values{
				"test_key_1": "foo",
				"test_key_2": "bar",
			},
			Values{
				"test_key_1": "foo",
				"test_key_2": "bar",
			},
			false,
		},
	}

	for _, expectation := range expectations {
		t.Run(expectation.testName, func(t *testing.T) {
			newValues, changed, err := ApplyValuesPatch(expectation.values, expectation.operations)

			if err != nil {
				t.Errorf("ApplyValuesPatch error: %s", err)
				return
			}

			if !reflect.DeepEqual(expectation.expectedValues, newValues) {
				t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.expectedValues, newValues)
			}

			if changed != expectation.expectedValuesChanged {
				t.Errorf("\n[EXPECTED]: %#v\n[GOT]: %#v", expectation.expectedValuesChanged, changed)
			}
		})
	}
}
