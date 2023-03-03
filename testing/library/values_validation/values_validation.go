/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package values_validation

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/flant/addon-operator/pkg/module_manager"
	"github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/values/validation"
	"helm.sh/helm/v3/pkg/chartutil"
	"sigs.k8s.io/yaml"
)

func LoadOpenAPISchemas(validator *validation.ValuesValidator, moduleName, modulePath string) error {
	openAPIDir := filepath.Join("/deckhouse", "global-hooks", "openapi")
	configBytes, valuesBytes, err := module_manager.ReadOpenAPIFiles(openAPIDir)
	if err != nil {
		return fmt.Errorf("read global openAPI schemas: %v", err)
	}
	err = validator.SchemaStorage.AddGlobalValuesSchemas(configBytes, valuesBytes)
	if err != nil {
		return fmt.Errorf("parse global openAPI schemas: %v", err)
	}

	if moduleName == "" || modulePath == "" {
		return nil
	}

	valuesKey := utils.ModuleNameToValuesKey(moduleName)
	openAPIPath := filepath.Join(modulePath, "openapi")
	configBytes, valuesBytes, err = module_manager.ReadOpenAPIFiles(openAPIPath)
	if err != nil {
		return fmt.Errorf("module '%s' read openAPI schemas: %v", moduleName, err)
	}

	err = validator.SchemaStorage.AddModuleValuesSchemas(valuesKey, configBytes, valuesBytes)
	if err != nil {
		return fmt.Errorf("parse global openAPI schemas: %v", err)
	}

	return nil
}

// ValidateValues is an adapter between JSONRepr and Values
func ValidateValues(validator *validation.ValuesValidator, moduleName string, values chartutil.Values) error {
	obj := values["Values"].(map[string]interface{})

	err := validator.ValidateGlobalValues(obj)
	if err != nil {
		return err
	}

	valuesKey := utils.ModuleNameToValuesKey(moduleName)
	err = validator.ValidateModuleValues(valuesKey, obj)
	if err != nil {
		return err
	}
	return nil
}

// ValidateValues is an adapter between JSONRepr and Values
func ValidateHelmValues(validator *validation.ValuesValidator, moduleName, values string) error {
	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(values), &obj)
	if err != nil {
		return err
	}

	err = validator.ValidateGlobalValues(obj)
	if err != nil {
		return err
	}

	valuesKey := utils.ModuleNameToValuesKey(moduleName)
	err = validator.ValidateModuleHelmValues(valuesKey, obj)
	if err != nil {
		return err
	}
	return nil
}

func ValidateJSONValues(validator *validation.ValuesValidator, moduleName string, values []byte, configValues bool) error {
	obj := map[string]interface{}{}
	err := json.Unmarshal(values, &obj)
	if err != nil {
		return err
	}

	err = validator.ValidateGlobalValues(obj)
	if err != nil {
		return err
	}

	valuesKey := utils.ModuleNameToValuesKey(moduleName)

	if configValues {
		err = validator.ValidateModuleConfigValues("config", obj)
	} else {
		err = validator.ValidateModuleValues(valuesKey, obj)
	}

	if err != nil {
		return err
	}
	return nil
}
