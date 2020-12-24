package common

import (
	"fmt"
	"path/filepath"

	"github.com/flant/addon-operator/pkg/module_manager"
	"github.com/flant/addon-operator/pkg/values/validation"
	"github.com/go-openapi/spec"
	"github.com/iancoleman/strcase"
	"sigs.k8s.io/yaml"
)

func LoadOpenAPISchemas(moduleName, modulePath string) error {
	openAPIDir := filepath.Join("/deckhouse", "global-hooks", "openapi")
	configBytes, valuesBytes, err := module_manager.ReadOpenAPISchemas(openAPIDir)
	if err != nil {
		return fmt.Errorf("read global openAPI schemas: %v", err)
	}
	if configBytes != nil {
		err = validation.AddGlobalValuesSchema("config", configBytes)
		if err != nil {
			return fmt.Errorf("parse global config openAPI: %v", err)
		}
	}
	if valuesBytes != nil {
		err = validation.AddGlobalValuesSchema("memory", valuesBytes)
		if err != nil {
			return fmt.Errorf("parse global values openAPI: %v", err)
		}
	}

	if moduleName == "" || modulePath == "" {
		return nil
	}

	openAPIPath := filepath.Join(modulePath, "openapi")
	configBytes, valuesBytes, err = module_manager.ReadOpenAPISchemas(openAPIPath)
	if err != nil {
		return fmt.Errorf("module '%s' read openAPI schemas: %v", moduleName, err)
	}
	if configBytes != nil {
		err = validation.AddModuleValuesSchema(moduleName, "config", configBytes)
		if err != nil {
			return fmt.Errorf("module '%s' parse config openAPI: %v", moduleName, err)
		}
	}
	if valuesBytes != nil {
		err = validation.AddModuleValuesSchema(moduleName, "memory", valuesBytes)
		if err != nil {
			return fmt.Errorf("module '%s' parse config openAPI: %v", moduleName, err)
		}
	}

	return nil
}

func ValidateValues(moduleName, values string) error {
	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(values), &obj)
	if err != nil {
		return err
	}

	valuesKey := strcase.ToLowerCamel(moduleName)
	schemaValidation := []struct {
		schema *spec.Schema
		key    string
	}{
		{
			schema: validation.GetGlobalValuesSchema("memory"),
			key:    "global",
		},
		{
			schema: validation.GetGlobalValuesSchema("config"),
			key:    "global",
		},
		{
			schema: validation.GetModuleValuesSchema(moduleName, "memory"),
			key:    valuesKey,
		},
		{
			schema: func() *spec.Schema {
				s := validation.GetModuleValuesSchema(moduleName, "config")
				if s == nil {
					return s
				}
				// Do not validate internal values with config schema
				s.Properties["internal"] = spec.Schema{}
				return s
			}(),
			key: valuesKey,
		},
	}

	for _, ss := range schemaValidation {
		if ss.schema == nil {
			continue
		}

		err = validation.ValidateObject(obj[ss.key], ss.schema, ss.key)
		if err != nil {
			return err
		}
	}
	return nil
}
