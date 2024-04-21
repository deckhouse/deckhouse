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
	"io"
	"path/filepath"
	"testing"

	"github.com/deckhouse/deckhouse/testing/library/values_validation"

	"github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/values/validation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
)

func TestOpenAPICases(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("OpenAPI case tests", func() {
	var err error

	openAPIDirs, err := GetAllOpenAPIDirs()
	It("Should find some directories with openapi-case-tests.yaml file", func() {
		Expect(err).ToNot(HaveOccurred())
		Expect(openAPIDirs).ToNot(HaveLen(0))
	})

	allTestCases := make([]*TestCases, 0)
	var testCases *TestCases
	for _, dir := range openAPIDirs {
		testCases, err = TestCasesFromFile(filepath.Join(dir, "openapi-case-tests.yaml"))
		if err != nil {
			break
		}
		testCases.dir = dir
		if testCases.hasFocused {
			allTestCases = []*TestCases{testCases}
			break
		}
		allTestCases = append(allTestCases, testCases)
	}
	Expect(err).NotTo(HaveOccurred(), "All openapi test cases should be parsed")

	for _, item := range allTestCases {
		// Need copy for proper carrying in test function.
		testCases := item
		It(fmt.Sprintf("Openapi test cases should pass in %s", testCases.dir), func() {
			ExecuteTestCases(testCases)
		})
	}
})

func ExecuteTestCases(testCases *TestCases) {
	// Silence addon-operator logger. (Validation, moduleManager)
	log.SetOutput(io.Discard)

	modulePath, _ := filepath.Split(testCases.dir)
	moduleName := filepath.Base(modulePath)

	By("Read openAPI schemas")
	configBytes, valuesBytes, err := utils.ReadOpenAPIFiles(testCases.dir)
	Expect(err).NotTo(HaveOccurred())

	By("Parse openAPI schemas")
	validator, err := values_validation.NewValuesValidator(moduleName, testCases.dir)
	Expect(err).NotTo(HaveOccurred())

	By("Check if test cases for config values are present and openapi/config-values.yaml is loaded")
	if configBytes == nil && testCases.HaveConfigValuesCases() {
		Expect(fmt.Errorf("found positive or negative config values cases in '%s', but there is no openapi/config-values.yaml schema", moduleName)).ShouldNot(HaveOccurred())
	}

	By("Check if test cases for values are present and openapi/values.yaml is loaded")
	if valuesBytes == nil && testCases.HaveValuesCases() {
		Expect(fmt.Errorf("found positive or negative values cases in '%s', but there is no openapi/values.yaml schema", moduleName)).ShouldNot(HaveOccurred())
	}

	By("Test schema with positive test cases")
	err = PositiveCasesTest(validator, moduleName, testCases)
	Expect(err).NotTo(HaveOccurred())

	By("Test schema with negative test cases")
	err = NegativeCasesTest(validator, moduleName, testCases)
	Expect(err).NotTo(HaveOccurred())
}

func PositiveCasesTest(validator *values_validation.ValuesValidator, moduleName string, testCases *TestCases) error {
	for _, testCase := range testCases.Positive.ConfigValues {
		err := ValidatePositiveCase(validator, moduleName, validation.ConfigValuesSchema, testCase, testCases.hasFocused)
		if err != nil {
			return err
		}
	}
	for _, testCase := range testCases.Positive.Values {
		err := ValidatePositiveCase(validator, moduleName, validation.ValuesSchema, testCase, testCases.hasFocused)
		if err != nil {
			return err
		}
	}
	for _, testCase := range testCases.Positive.HelmValues {
		err := ValidatePositiveCase(validator, moduleName, validation.HelmValuesSchema, testCase, testCases.hasFocused)
		if err != nil {
			return err
		}
	}
	return nil
}

func NegativeCasesTest(validator *values_validation.ValuesValidator, moduleName string, testCases *TestCases) error {
	for _, testCase := range testCases.Negative.ConfigValues {
		err := ValidateNegativeCase(validator, moduleName, validation.ConfigValuesSchema, testCase, testCases.hasFocused)
		if err != nil {
			return err
		}
	}
	for _, testCase := range testCases.Negative.Values {
		err := ValidateNegativeCase(validator, moduleName, validation.ValuesSchema, testCase, testCases.hasFocused)
		if err != nil {
			return err
		}
	}
	for _, testCase := range testCases.Negative.HelmValues {
		err := ValidateNegativeCase(validator, moduleName, validation.HelmValuesSchema, testCase, testCases.hasFocused)
		if err != nil {
			return err
		}
	}
	return nil
}
