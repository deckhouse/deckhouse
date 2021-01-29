package openapi_cases

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager"
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
	openAPIDirs, err := GetAllOpenAPIDirs()
	It("Should find some directories with openapi-case-tests.yaml file", func() {
		Expect(err).ToNot(HaveOccurred())
		Expect(openAPIDirs).ToNot(HaveLen(0))
	})
	for _, dir := range openAPIDirs {
		testDir := dir
		It(fmt.Sprintf("Openapi test cases should pass in %s", testDir), func() {
			ExecuteTestCasesInDir(testDir)
		})
	}
})

func ExecuteTestCasesInDir(dir string) {
	// Silence addon-operator logger. (Validation, ModuleManager)
	log.SetOutput(ioutil.Discard)

	var testCases TestCases

	modulePath, _ := filepath.Split(dir)
	moduleName := filepath.Base(modulePath)
	By("Parse openAPI test cases file")
	testCases, err := ParseCasesTestFile(filepath.Join(dir, "openapi-case-tests.yaml"))
	Expect(err).NotTo(HaveOccurred())

	By("Read openAPI schemas")
	configBytes, valuesBytes, err := module_manager.ReadOpenAPIFiles(dir)
	Expect(err).NotTo(HaveOccurred())

	By("Parse openAPI schemas")
	validator := validation.NewValuesValidator()
	err = validator.SchemaStorage.AddModuleValuesSchemas(moduleName, configBytes, valuesBytes)
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

func PositiveCasesTest(validator *validation.ValuesValidator, moduleName string, testCases TestCases) error {
	for _, testCase := range testCases.Positive.ConfigValues {
		err := ValidateCase(validator, moduleName, validation.ConfigValuesSchema, testCase)
		if err != nil {
			return err
		}
	}
	for _, testCase := range testCases.Positive.Values {
		err := ValidateCase(validator, moduleName, validation.ValuesSchema, testCase)
		if err != nil {
			return err
		}
	}
	return nil
}

func NegativeCasesTest(validator *validation.ValuesValidator, moduleName string, testCases TestCases) error {
	for _, testCase := range testCases.Negative.ConfigValues {
		err := ValidateCase(validator, moduleName, validation.ConfigValuesSchema, testCase)
		if err == nil {
			return fmt.Errorf("negative case for config values: %s", string(testCase))
		}
	}
	for _, testCase := range testCases.Negative.Values {
		err := ValidateCase(validator, moduleName, validation.ValuesSchema, testCase)
		if err == nil {
			return fmt.Errorf("negative case for config values: %s", string(testCase))
		}
	}
	return nil
}
