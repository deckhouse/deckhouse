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

	config, err = NewModuleConfig("test-module", map[interface{}]interface{}{"hello": "world"})
	if err != nil {
		t.Error(err)
	}
	if !config.IsEnabled {
		t.Errorf("Expected module to be enabled")
	}
	if !reflect.DeepEqual(config.Values, map[interface{}]interface{}{"hello": "world"}) {
		t.Errorf("Got unexpected config values: %v", config.Values)
	}
}
