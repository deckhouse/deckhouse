package modules

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
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
	toHelmignore  = []string{"hooks", "crds", "enabled"}
	regexPatterns = map[string]string{
		`$BASE_ALPINE`:         imageRegexp(`alpine:[\d.]+`),
		`$BASE_DEBIAN`:         imageRegexp(`debian:[\d.]+`),
		`$BASE_GOLANG_ALPINE`:  imageRegexp(`golang:[\d.]+-alpine`),
		`$BASE_GOLANG_BUSTER`:  imageRegexp(`golang:[\d.]+-buster`),
		`$BASE_NGINX_ALPINE`:   imageRegexp(`nginx:[\d.]+-alpine`),
		`$BASE_PYTHON_ALPINE`:  imageRegexp(`python:[\d.]+-alpine`),
		`$BASE_SHELL_OPERATOR`: imageRegexp(`shell-operator:v[\d.]+`),
		`$BASE_UBUNTU`:         imageRegexp(`ubuntu:[\d.]+`),
	}
)

func skipModuleImageNameIfNeeded(filePath string) bool {
	// Kube-apiserver 1.15 needs golang 1.12 to build, so we don't use $BASE_GOLANG_ALPINE image for building
	return filePath == "/deckhouse/modules/040-control-plane-manager/images/kube-apiserver-1-15/Dockerfile"
}

func shouldSkipModule(name string) bool {
	switch name {
	case "helm_lib",
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

const commonTestGoContent = `package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}
`

func commonTestGoForHooks(name, path string) errors.LintRuleError {
	if !isExistsOnFilesystem(path + "/hooks") {
		return errors.EmptyRuleError
	}

	if matches, _ := filepath.Glob(path + "/hooks/*.go"); len(matches) == 0 {
		return errors.EmptyRuleError
	}

	commonTestPath := filepath.Join(path, "hooks", "common_test.go")

	if !isExistsOnFilesystem(commonTestPath) {
		return errors.NewLintRuleError(
			"MODULE001",
			"module = "+name,
			nil,
			"Module does not contain %q file", commonTestPath,
		)
	}

	contentBytes, err := ioutil.ReadFile(commonTestPath)
	if err != nil {
		return errors.NewLintRuleError(
			"MODULE001",
			"module = "+name,
			nil,
			"Module does not contain %q file", commonTestPath,
		)
	}

	if string(contentBytes) != commonTestGoContent {
		return errors.NewLintRuleError(
			"MODULE001",
			"module = "+name,
			nil,
			"Module content of %q file is different from default\nContent should be equal to:\n%s",
			commonTestPath, commonTestGoContent,
		)
	}

	return errors.EmptyRuleError
}

func imageRegexp(s string) string {
	return fmt.Sprintf("^(from:|FROM)(\\s+)(%s)", s)
}

func isImageNameUnacceptable(imageName string) (bool, string) {
	for ciVariable, pattern := range regexPatterns {
		matched, _ := regexp.MatchString(pattern, imageName)
		if matched {
			return true, ciVariable
		}
	}
	return false, ""
}

func checkImageNamesInDockerAndWerfFiles(name, path string, lintRuleErrorsList *errors.LintRuleErrorsList) {
	var filePaths []string
	imagesPath := filepath.Join(path, "images")

	if !isExistsOnFilesystem(imagesPath) {
		return
	}

	err := filepath.Walk(imagesPath, func(fullPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		switch filepath.Base(fullPath) {
		case "werf.inc.yaml",
			"Dockerfile":
			filePaths = append(filePaths, fullPath)
		}
		return nil
	})

	if err != nil {
		lintRuleErrorsList.Add(errors.NewLintRuleError(
			"MODULE001",
			"module = "+name,
			imagesPath,
			"Cannot read directory structure:%s",
			err,
		))
		return
	}
	for _, filePath := range filePaths {
		if skipModuleImageNameIfNeeded(filePath) {
			continue
		}
		file, err := os.Open(filePath)
		if err != nil {
			lintRuleErrorsList.Add(errors.NewLintRuleError(
				"MODULE001",
				"module = "+name,
				filePath,
				"Error opening file:%s",
				err,
			))
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		linePos := 0
		relativeFilePath, err := filepath.Rel(imagesPath, filePath)
		if err != nil {
			lintRuleErrorsList.Add(errors.NewLintRuleError(
				"MODULE001",
				"module = "+name,
				filePath,
				"Error calculating relative file path:%s",
				err,
			))
			continue
		}

		for scanner.Scan() {
			line := scanner.Text()
			linePos++
			result, ciVariable := isImageNameUnacceptable(line)
			if result {
				lintRuleErrorsList.Add(errors.NewLintRuleError(
					"MODULE001",
					fmt.Sprintf("module = %s, image = %s, line = %d", name, relativeFilePath, linePos),
					line,
					"Please use %s as an image name", ciVariable,
				))
			}
		}
	}
}

func GetDeckhouseModulesWithValuesMatrixTests() ([]types.Module, error) {
	var modules []types.Module

	modulesDir, ok := os.LookupEnv("MODULES_DIR")
	if !ok {
		modulesDir = defaultDeckhouseModulesDir
	}

	modulePaths, err := getModulePaths(modulesDir)
	if err != nil {
		return modules, fmt.Errorf("search modules with Chart.yaml: %v", err)
	}

	var lintRuleErrorsList errors.LintRuleErrorsList
	for _, modulePath := range modulePaths {
		module, ok := lintModuleStructure(lintRuleErrorsList, modulePath)
		if !ok {
			continue
		}
		modules = append(modules, module)
	}

	return modules, lintRuleErrorsList.ConvertToError()
}

func isExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

// getModulePaths returns all paths with Chart.yaml
// modulesDir can be a module directory or a directory that contains modules in subdirectories.
func getModulePaths(modulesDir string) ([]string, error) {
	var chartDirs = make([]string, 0)

	if isExistsOnFilesystem(modulesDir, "Chart.yaml") {
		chartDirs = append(chartDirs, modulesDir)
		return chartDirs, nil
	}

	// Here we find all dirs and check for Chart.yaml in them.
	err := filepath.Walk(modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error access '%s': %v", path, err)
		}

		// Ignore root path and non-dirs
		if path == modulesDir || !info.IsDir() {
			return nil
		}

		if shouldSkipModule(filepath.Base(path)) {
			return filepath.SkipDir
		}

		// Check if first level subdirectory has a Chart.yaml
		if isExistsOnFilesystem(path, "Chart.yaml") {
			chartDirs = append(chartDirs, path)
		}
		return filepath.SkipDir
	})

	if err != nil {
		return nil, err
	}
	return chartDirs, nil
}

// lintModuleStructure collects linting errors
// for helmignore, hooks, docker and werf files, namespace, and CRDs
func lintModuleStructure(lintRuleErrorsList errors.LintRuleErrorsList, modulePath string) (types.Module, bool) {
	moduleName := filepath.Base(modulePath)

	lintRuleErrorsList.Add(helmignoreModuleRule(moduleName, modulePath))
	lintRuleErrorsList.Add(commonTestGoForHooks(moduleName, modulePath))
	checkImageNamesInDockerAndWerfFiles(moduleName, modulePath, &lintRuleErrorsList)

	name, lintError := chartModuleRule(moduleName, modulePath)
	lintRuleErrorsList.Add(lintError)
	if name == "" {
		return types.Module{}, false
	}

	namespace, lintError := namespaceModuleRule(moduleName, modulePath)
	lintRuleErrorsList.Add(lintError)
	if namespace == "" {
		return types.Module{}, false
	}

	if isExistsOnFilesystem(modulePath, "crds") {
		lintRuleErrorsList.Merge(crdsModuleRule(moduleName, modulePath+"/crds"))
	}

	module := types.Module{Name: name, Path: modulePath, Namespace: namespace}
	return module, true
}
