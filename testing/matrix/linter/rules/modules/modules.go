package modules

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
)

const (
	ValuesConfigFilename       = "values_matrix_test.yaml"
	defaultDeckhouseModulesDir = "/deckhouse/modules"
)

var (
	toHelmignore = []string{"hooks", "crds", "enabled", "candi"}
)

func skipModuleIfNeeded(name string) bool {
	switch name {
	case "helm_lib",
		"340-monitoring-kubernetes-control-plane",
		"360-istio",
		"400-nginx-ingress",
		"500-dashboard":
		return true
	}
	return false
}

func namespaceModuleRule(name, path string) (string, errors.LintRuleError) {
	content, err := ioutil.ReadFile(path + "/.namespace")
	if err != nil {
		return "", errors.NewLintRuleError(
			"MODULE002",
			"module = "+name,
			nil,
			"Module does not contain \".namespace\" file, module will be ignored",
		)
	}
	return strings.TrimRight(string(content), " \t\n"), errors.EmptyRuleError
}

func chartModuleRule(name, path string) (string, errors.LintRuleError) {
	lintError := errors.NewLintRuleError(
		"MODULE002",
		"module = "+name,
		nil,
		"Module does not contain valid \"Chart.yaml\" file, module will be ignored",
	)

	yamlFile, err := ioutil.ReadFile(path + "/Chart.yaml")
	if err != nil {
		return "", lintError
	}

	var chart struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(yamlFile, &chart)
	if err != nil {
		return "", lintError
	}

	if !isExistsOnFilesystem(path, ValuesConfigFilename) {
		return "", errors.NewLintRuleError(
			"MODULE002",
			"module = "+name,
			nil,
			"Module does not contain %q file, module will be ignored", ValuesConfigFilename,
		)
	}

	return chart.Name, errors.EmptyRuleError
}

func helmignoreModuleRule(name, path string) errors.LintRuleError {
	var existedFiles []string
	for _, file := range toHelmignore {
		if isExistsOnFilesystem(path, file) {
			existedFiles = append(existedFiles, file)
		}
	}
	if len(existedFiles) == 0 {
		return errors.EmptyRuleError
	}

	contentBytes, err := ioutil.ReadFile(path + "/.helmignore")
	if err != nil {
		return errors.NewLintRuleError(
			"MODULE001",
			"module = "+name,
			nil,
			"Module does not contain \".helmignore\" file",
		)
	}

	var moduleErrors []string
	content := string(contentBytes)
	for _, existedFile := range existedFiles {
		if strings.Contains(content, existedFile) {
			continue
		}
		moduleErrors = append(moduleErrors, existedFile)
	}

	if len(moduleErrors) > 0 {
		return errors.NewLintRuleError(
			"MODULE001",
			"module = "+name,
			strings.Join(moduleErrors, ", "),
			"module does not have desired entries in \".helmignore\" file",
		)
	}
	return errors.EmptyRuleError
}

func GetDeckhouseModulesWithValuesMatrixTests() ([]types.Module, error) {
	var modules []types.Module

	modulesDir, ok := os.LookupEnv("MODULES_DIR")
	if !ok {
		modulesDir = defaultDeckhouseModulesDir
	}

	var lintRuleErrorsList errors.LintRuleErrorsList
	_ = filepath.Walk(modulesDir, func(path string, info os.FileInfo, _ error) error {
		if !isExistsOnFilesystem(path, "/Chart.yaml") {
			return nil
		}

		parts := strings.Split(path, string(os.PathSeparator))
		moduleName := parts[len(parts)-1]

		if skipModuleIfNeeded(moduleName) {
			return nil
		}

		lintRuleErrorsList.Add(helmignoreModuleRule(moduleName, path))

		name, lintError := chartModuleRule(moduleName, path)
		lintRuleErrorsList.Add(lintError)
		if name == "" {
			return nil
		}

		namespace, lintError := namespaceModuleRule(moduleName, path)
		lintRuleErrorsList.Add(lintError)
		if namespace == "" {
			return nil
		}

		if isExistsOnFilesystem(path, "/crds") {
			lintRuleErrorsList.Merge(crdsModuleRule(moduleName, path+"/crds"))
		}

		modules = append(modules, types.Module{Name: name, Path: path, Namespace: namespace})
		return nil
	})
	return modules, lintRuleErrorsList.ConvertToError()
}

func isExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(strings.Join(parts, string(os.PathSeparator)))
	return err == nil
}
