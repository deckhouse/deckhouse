package utils

import (
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
