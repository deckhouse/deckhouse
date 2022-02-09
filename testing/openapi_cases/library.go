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

package openapi_cases

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/values/validation"
	"sigs.k8s.io/yaml"
)

type TestCase struct {
	ConfigValues []json.RawMessage
	Values       []json.RawMessage
}

type TestCases struct {
	Positive TestCase
	Negative TestCase
}

func (t *TestCases) HaveConfigValuesCases() bool {
	return len(t.Positive.ConfigValues) > 0 || len(t.Negative.ConfigValues) > 0
}

func (t *TestCases) HaveValuesCases() bool {
	return len(t.Positive.Values) > 0 || len(t.Negative.Values) > 0
}

func GetAllOpenAPIDirs() ([]string, error) {
	var (
		dirs        []string
		openAPIDirs []string
	)

	for _, possibleDir := range []string{
		os.Getenv("DECKHOUSE_ROOT") + "/deckhouse/modules/*/openapi",
		os.Getenv("DECKHOUSE_ROOT") + "/deckhouse/ee/modules/*/openapi",
		os.Getenv("DECKHOUSE_ROOT") + "/deckhouse/ee/fe/modules/*/openapi",
	} {
		globDirs, err := filepath.Glob(possibleDir)
		if err != nil {
			return nil, err
		}

		openAPIDirs = append(openAPIDirs, globDirs...)
	}

	openAPIDirs = append(openAPIDirs, os.Getenv("DECKHOUSE_ROOT")+"/deckhouse/global-hooks/openapi")
	for _, openAPIDir := range openAPIDirs {
		info, err := os.Stat(openAPIDir)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			continue
		}
		dirs = append(dirs, openAPIDir)
	}
	return dirs, nil
}

func ParseCasesTestFile(filename string) (TestCases, error) {
	var testCases TestCases
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return testCases, err
	}
	err = yaml.Unmarshal(yamlFile, &testCases)
	if err != nil {
		return testCases, err
	}
	return testCases, nil
}

func ValidateCase(validator *validation.ValuesValidator, moduleName string, schema validation.SchemaType, testCase json.RawMessage) error {
	var values map[string]interface{}
	err := json.Unmarshal(testCase, &values)
	if err != nil {
		return err
	}
	err = validator.ValidateValues(validation.ModuleSchema, schema, moduleName, utils.Values{moduleName: values})
	if err != nil {
		return err
	}
	return nil
}
