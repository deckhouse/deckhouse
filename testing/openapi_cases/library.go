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
	"fmt"
	"os"
	"path/filepath"

	"github.com/deckhouse/deckhouse/testing/library/values_validation"

	"github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/values/validation"
	"sigs.k8s.io/yaml"
)

const FocusFieldName = "x-test-focus"

type TestCase struct {
	ConfigValues []map[string]interface{}
	Values       []map[string]interface{}
	HelmValues   []map[string]interface{}
}

func (tc TestCase) HasFocused() bool {
	for _, values := range tc.ConfigValues {
		if _, hasFocus := values[FocusFieldName]; hasFocus {
			return true
		}
	}
	for _, values := range tc.Values {
		if _, hasFocus := values[FocusFieldName]; hasFocus {
			return true
		}
	}
	for _, values := range tc.HelmValues {
		if _, hasFocus := values[FocusFieldName]; hasFocus {
			return true
		}
	}
	return false
}

type TestCases struct {
	Positive TestCase
	Negative TestCase

	dir        string
	hasFocused bool
}

func (t *TestCases) HaveConfigValuesCases() bool {
	return len(t.Positive.ConfigValues) > 0 || len(t.Negative.ConfigValues) > 0
}

func (t *TestCases) HaveValuesCases() bool {
	return len(t.Positive.Values) > 0 || len(t.Negative.Values) > 0
}

func (t *TestCases) HaveHelmValuesCases() bool {
	return len(t.Positive.HelmValues) > 0 || len(t.Negative.HelmValues) > 0
}

type edition struct {
	Name       string `yaml:"name,omitempty"`
	ModulesDir string `yaml:"modulesDir,omitempty"`
}

type editions struct {
	Editions []edition `yaml:"editions,omitempty"`
}

func getPossiblePathToModules() []string {
	content, err := os.ReadFile("/deckhouse/editions.yaml")
	if err != nil {
		panic(fmt.Sprintf("cannot read editions file: %v", err))
	}

	e := editions{}
	err = yaml.Unmarshal(content, &e)
	if err != nil {
		panic(fmt.Errorf("cannot unmarshal editions file: %v", err))
	}

	modulesDir := make([]string, 0)
	for i, ed := range e.Editions {
		if ed.Name == "" {
			panic(fmt.Sprintf("name for %d index is empty", i))
		}
		modulesDir = append(modulesDir, fmt.Sprintf("/deckhouse/%s/*/openapi", ed.ModulesDir))
	}

	return modulesDir
}

func GetAllOpenAPIDirs() ([]string, error) {
	var (
		dirs        []string
		openAPIDirs []string
	)

	for _, possibleDir := range getPossiblePathToModules() {
		globDirs, err := filepath.Glob(possibleDir)
		if err != nil {
			return nil, err
		}

		openAPIDirs = append(openAPIDirs, globDirs...)
	}

	openAPIDirs = append(openAPIDirs, "/deckhouse/global-hooks/openapi")
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

func TestCasesFromFile(filename string) (*TestCases, error) {
	var testCases TestCases
	yamlFile, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(yamlFile, &testCases)
	if err != nil {
		return nil, err
	}
	testCases.hasFocused = testCases.Positive.HasFocused() || testCases.Negative.HasFocused()
	return &testCases, nil
}

func ValidatePositiveCase(validator *values_validation.ValuesValidator, moduleName string, schema validation.SchemaType, testValues map[string]interface{}, runFocused bool) error {
	if _, hasFocus := testValues[FocusFieldName]; !hasFocus && runFocused {
		return nil
	}
	delete(testValues, FocusFieldName)
	return validator.ModuleSchemaStorage.Validate(schema, moduleName, utils.Values{moduleName: testValues})
}

func ValidateNegativeCase(validator *values_validation.ValuesValidator, moduleName string, schema validation.SchemaType, testValues map[string]interface{}, runFocused bool) error {
	_, hasFocus := testValues[FocusFieldName]
	if !hasFocus && runFocused {
		return nil
	}
	delete(testValues, FocusFieldName)
	err := validator.ModuleSchemaStorage.Validate(schema, moduleName, utils.Values{moduleName: testValues})
	if err == nil {
		return fmt.Errorf("negative case error for %s values: test case should not pass validation: %+v", schema, ValuesToString(testValues))
	}
	// Focusing is a debugging tool, so print hidden error.
	if hasFocus {
		fmt.Printf("Debug: expected error for negative case: %v\n", err)
	}
	return nil
}

func ValuesToString(v map[string]interface{}) string {
	b, _ := yaml.Marshal(v)
	return string(b)
}
