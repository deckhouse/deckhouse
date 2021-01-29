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
	var dirs []string
	openAPIDirs, err := filepath.Glob("/deckhouse/modules/*/openapi")
	if err != nil {
		return nil, err
	}
	// TODO - Global scheme currently not supported
	// openAPIDirs = append(openAPIDirs, "/deckhouse/global-hooks/openapi")
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
