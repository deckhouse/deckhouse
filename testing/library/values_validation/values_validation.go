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

	"github.com/flant/addon-operator/pkg/values/validation"

	"github.com/flant/addon-operator/pkg/utils"
	log "github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/chartutil"
	"sigs.k8s.io/yaml"
)

type ValuesValidator struct {
	GlobalSchemaStorage  *validation.SchemaStorage
	ModuleSchemaStorages map[string]*validation.SchemaStorage
}

func NewValuesValidator(moduleName, modulePath string) (*ValuesValidator, error) {
	openAPIDir := filepath.Join("/deckhouse", "global-hooks", "openapi")
	configBytes, valuesBytes, err := utils.ReadOpenAPIFiles(openAPIDir)
	if err != nil {
		return nil, fmt.Errorf("read global openAPI schemas: %v", err)
	}

	globalSchemaStorage, err := validation.NewSchemaStorage(configBytes, valuesBytes)
	if err != nil {
		return nil, fmt.Errorf("parse global openAPI schemas: %v", err)
	}

	if moduleName == "" || modulePath == "" {
		return &ValuesValidator{GlobalSchemaStorage: globalSchemaStorage}, nil
	}

	openAPIPath := filepath.Join(modulePath, "openapi")
	configBytes, valuesBytes, err = utils.ReadOpenAPIFiles(openAPIPath)
	if err != nil {
		return nil, fmt.Errorf("module '%s' read openAPI schemas: %v", moduleName, err)
	}

	moduleSchemaStorage, err := validation.NewSchemaStorage(configBytes, valuesBytes)
	if err != nil {
		return nil, fmt.Errorf("parse module openAPI schemas: %v", err)
	}

	return &ValuesValidator{
		GlobalSchemaStorage: globalSchemaStorage,
		ModuleSchemaStorages: map[string]*validation.SchemaStorage{
			moduleName: moduleSchemaStorage,
		},
	}, nil
}

// ValidateValues is an adapter between JSONRepr and Values
func (vv *ValuesValidator) ValidateValues(moduleName string, values chartutil.Values) error {
	obj := values["Values"].(map[string]interface{})

	err := vv.ValidateGlobalValues(obj)
	if err != nil {
		return err
	}

	valuesKey := utils.ModuleNameToValuesKey(moduleName)
	err = vv.ValidateModuleValues(valuesKey, obj)
	if err != nil {
		return err
	}
	return nil
}

func (vv *ValuesValidator) ValidateHelmValues(moduleName string, values string) error {
	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(values), &obj)
	if err != nil {
		return err
	}

	err = vv.ValidateGlobalValues(obj)
	if err != nil {
		return err
	}

	valuesKey := utils.ModuleNameToValuesKey(moduleName)
	err = vv.ValidateModuleHelmValues(valuesKey, obj)
	if err != nil {
		return err
	}
	return nil
}

func (vv *ValuesValidator) ValidateJSONValues(moduleName string, values []byte, configValues bool) error {
	obj := map[string]interface{}{}
	err := json.Unmarshal(values, &obj)
	if err != nil {
		return err
	}

	err = vv.ValidateGlobalValues(obj)
	if err != nil {
		return err
	}

	valuesKey := utils.ModuleNameToValuesKey(moduleName)

	if configValues {
		err = vv.ValidateConfigValues("config", obj)
	} else {
		err = vv.ValidateModuleValues(valuesKey, obj)
	}

	if err != nil {
		return err
	}
	return nil
}

func (vv *ValuesValidator) ValidateGlobalValues(obj utils.Values) error {
	return vv.GlobalSchemaStorage.ValidateValues(utils.GlobalValuesKey, obj)
}

func (vv *ValuesValidator) ValidateModuleValues(moduleName string, obj utils.Values) error {
	ss := vv.ModuleSchemaStorages[moduleName]
	if ss == nil {
		log.Warnf("schema storage for '%s' is not found", moduleName)
		return nil
	}

	return vv.ModuleSchemaStorages[moduleName].ValidateValues(moduleName, obj)
}

func (vv *ValuesValidator) ValidateModuleHelmValues(moduleName string, obj utils.Values) error {
	ss := vv.ModuleSchemaStorages[moduleName]
	if ss == nil {
		log.Warnf("schema storage for '%s' is not found", moduleName)
		return nil
	}

	return vv.ModuleSchemaStorages[moduleName].ValidateModuleHelmValues(moduleName, obj)
}

func (vv *ValuesValidator) ValidateConfigValues(moduleName string, obj utils.Values) error {
	ss := vv.ModuleSchemaStorages[moduleName]
	if ss == nil {
		log.Warnf("schema storage for '%s' is not found", moduleName)
		return nil
	}

	return vv.ModuleSchemaStorages[moduleName].ValidateConfigValues(moduleName, obj)
}
