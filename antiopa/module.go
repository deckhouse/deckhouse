package main

import (
	"bytes"
	"fmt"
	"github.com/gobwas/glob"
	"github.com/romana/rlog"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type Module struct {
	Name          string
	DirectoryName string
	Path          string
}

func RunModules() {
	retryModulesNamesQueue = make([]string, 0)
	for _, moduleName := range modulesOrder {
		RunModule(moduleName)
	}
}

func RunModule(moduleName string) {
	vals, err := PrepareModuleValues(moduleName)
	if err != nil {
		rlog.Errorf("Cannot prepare values for module %s: %s", moduleName, err)
		retryModulesNamesQueue = append(retryModulesNamesQueue, moduleName)
		return
	}
	rlog.Debugf("Prepared module %s VALUES:\n%s", moduleName, valuesToString(vals))

	valuesPath, err := dumpModuleValuesYaml(moduleName, vals)
	if err != nil {
		rlog.Errorf("Cannot dump values yaml for module %s: %s", moduleName, err)
		retryModulesNamesQueue = append(retryModulesNamesQueue, moduleName)
		return
	}

	err = RunModuleBeforeHelmHooks(moduleName, valuesPath)
	if err != nil {
		rlog.Errorf("Module %s before-helm hooks error: %s", moduleName, err)
		retryModulesNamesQueue = append(retryModulesNamesQueue, moduleName)
		return
	}

	err = RunModuleHelm(moduleName, valuesPath)
	if err != nil {
		rlog.Errorf("Module %s run error: %s", moduleName, err)
		retryModulesNamesQueue = append(retryModulesNamesQueue, moduleName)
	}

	err = RunModuleAfterHelmHooks(moduleName, valuesPath)
	if err != nil {
		rlog.Errorf("Module %s after-helm hooks error: %s", moduleName, err)
		retryModulesNamesQueue = append(retryModulesNamesQueue, moduleName)
		return
	}
}

func RunModuleBeforeHelmHooks(moduleName string, ValuesPath string) error {
	module, hasModule := modulesByName[moduleName]
	if !hasModule {
		return fmt.Errorf("no such module %s", moduleName)
	}

	hooksDir := filepath.Join(module.Path, "hooks", "before-helm")

	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil
	}

	hooksNames, err := readDirectoryExecutableFilesNames(hooksDir)
	if err != nil {
		return err
	}

	for _, hookName := range hooksNames {
		rlog.Infof("Running module %s before-helm hook %s ...", moduleName, hookName)

		err := execCommand(makeModuleCommand(module.Path, ValuesPath, filepath.Join(hooksDir, hookName), []string{}))
		if err != nil {
			return fmt.Errorf("before-helm hook %s FAILED: %s", hookName, err)
		}
	}

	return nil
}

func RunModuleAfterHelmHooks(moduleName string, ValuesPath string) error {
	module, hasModule := modulesByName[moduleName]
	if !hasModule {
		return fmt.Errorf("no such module %s", moduleName)
	}

	hooksDir := filepath.Join(module.Path, "hooks", "after-helm")

	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil
	}

	hooksNames, err := readDirectoryExecutableFilesNames(hooksDir)
	if err != nil {
		return err
	}

	for _, hookName := range hooksNames {
		rlog.Infof("Running module %s after-helm hook %s ...", moduleName, hookName)

		err := execCommand(makeModuleCommand(module.Path, ValuesPath, filepath.Join(hooksDir, hookName), []string{}))
		if err != nil {
			return fmt.Errorf("after-helm hook %s FAILED: %s", hookName, err)
		}
	}

	return nil
}

func RunModuleHelm(moduleName string, ValuesPath string) error {
	module, hasModule := modulesByName[moduleName]
	if !hasModule {
		return fmt.Errorf("no such module %s", moduleName)
	}

	chartPath := filepath.Join(module.Path, "Chart.yaml")

	if _, err := os.Stat(chartPath); !os.IsNotExist(err) {
		rlog.Infof("Running module %s helm ...", moduleName)

		helmReleaseName := moduleName

		err := execCommand(makeModuleCommand(module.Path, ValuesPath, "helm", []string{"upgrade", helmReleaseName, ".", "--install", "--namespace", HelmTillerNamespace(), "--values", ValuesPath}))
		if err != nil {
			return fmt.Errorf("helm FAILED: %s", err)
		}
	} else {
		rlog.Debugf("No helm chart found for module %s in %s", moduleName, chartPath)
	}

	return nil
}

func PrepareModuleValues(moduleName string) (map[interface{}]interface{}, error) {
	module, hasModule := modulesByName[moduleName]
	if !hasModule {
		return nil, fmt.Errorf("no such module %s", moduleName)
	}

	valuesShPath := filepath.Join(module.Path, "initial_values")

	if statRes, err := os.Stat(valuesShPath); !os.IsNotExist(err) {
		// Тупой тест, что файл executable.
		// Т.к. antiopa всегда работает под root, то этого достаточно.
		if statRes.Mode()&0111 != 0 {
			rlog.Debugf("Running values generator %s ...", valuesShPath)

			var valuesYamlBuffer bytes.Buffer
			cmd := exec.Command(valuesShPath)
			cmd.Env = append(cmd.Env, os.Environ()...)
			cmd.Dir = module.Path
			cmd.Stdout = &valuesYamlBuffer
			err := execCommand(cmd)
			if err != nil {
				return nil, fmt.Errorf("Values generator %s error: %s", valuesShPath, err)
			}

			var generatedValues map[interface{}]interface{}
			err = yaml.Unmarshal(valuesYamlBuffer.Bytes(), &generatedValues)
			if err != nil {
				return nil, fmt.Errorf("Got bad yaml from values generator %s: %s", valuesShPath, err)
			}
			rlog.Debugf("got VALUES from initial_values:\n%s", valuesToString(generatedValues))

			newModuleValues := MergeValues(generatedValues, kubeModulesValues[moduleName])

			rlog.Debugf("Updating module %s VALUES in ConfigMap:\n%s", moduleName, valuesToString(newModuleValues))

			err = SetModuleKubeValues(moduleName, newModuleValues)
			if err != nil {
				return nil, err
			}
			kubeModulesValues[moduleName] = newModuleValues
		} else {
			rlog.Warnf("Ignoring non executable file %s", valuesShPath)
		}

	}

	return MergeValues(globalValues, hooksValues, globalModulesValues[moduleName], kubeValues, kubeModulesValues[moduleName]), nil
}

func makeModuleCommand(ModuleDir string, ValuesPath string, Entrypoint string, Args []string) *exec.Cmd {
	cmd := exec.Command(Entrypoint, Args...)
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("VALUES_PATH=%s", ValuesPath),
		fmt.Sprintf("TILLER_NAMESPACE=%s", HelmTillerNamespace()),
	)

	cmd.Dir = ModuleDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd
}

func matchesGlob(value string, globPattern string) bool {
	g, err := glob.Compile(globPattern)
	if err != nil {
		return false
	}
	return g.Match(value)
}

func getEnabledModules() ([]Module, error) {
	allModules, err := readModules()
	if err != nil {
		return nil, err
	}

	cm, err := GetConfigMap()
	if err != nil {
		return nil, err
	}

	var disabledModulesNames []string
	for _, configKey := range []string{"disable-modules", "disabled-modules"} {
		if _, hasKey := cm.Data[configKey]; hasKey {
			disabledModulesNames = make([]string, 0)
			for _, moduleName := range strings.Split(cm.Data[configKey], ",") {
				disabledModulesNames = append(disabledModulesNames, strings.TrimSpace(moduleName))
			}
		}
	}

	for _, disabledModuleName := range disabledModulesNames {
		found := false
		for _, module := range allModules {
			if matchesGlob(module.Name, disabledModuleName) {
				found = true
				break
			}
		}

		if !found {
			rlog.Warnf("Bad value '%s' in antiopa ConfigMap disabled-modules: does not match any module", disabledModuleName)
		}
	}

	res := make([]Module, 0)
	for _, module := range allModules {
		isEnabled := true

		for _, disabledModuleName := range disabledModulesNames {
			if matchesGlob(module.Name, disabledModuleName) {
				isEnabled = false
				break
			}
		}

		if isEnabled {
			res = append(res, module)
		}
	}

	return res, nil
}

func readModules() ([]Module, error) {
	modulesDir := filepath.Join(WorkingDir, "modules")

	files, err := ioutil.ReadDir(modulesDir)
	if err != nil {
		return nil, fmt.Errorf("Cannot list modules directory %s: %s", modulesDir, err)
	}

	var validmoduleName = regexp.MustCompile(`^[0-9][0-9][0-9]-(.*)$`)

	res := make([]Module, 0)
	badModulesDirs := make([]string, 0)

	for _, file := range files {
		if file.IsDir() {
			matchRes := validmoduleName.FindStringSubmatch(file.Name())
			if matchRes != nil {
				module := Module{
					Name:          matchRes[1],
					DirectoryName: file.Name(),
					Path:          filepath.Join(modulesDir, file.Name()),
				}
				res = append(res, module)
			} else {
				badModulesDirs = append(badModulesDirs, filepath.Join(modulesDir, file.Name()))
			}
		}
	}

	if len(badModulesDirs) > 0 {
		return nil, fmt.Errorf("bad module directory names, must match regex `%s`: %s", validmoduleName, strings.Join(badModulesDirs, ", "))
	}

	return res, nil
}

func dumpModuleValuesYaml(moduleName string, Values map[interface{}]interface{}) (string, error) {
	return dumpValuesYaml(fmt.Sprintf("%s.yaml", moduleName), Values)
}

func dumpValuesYaml(FileName string, Values map[interface{}]interface{}) (string, error) {
	valuesYaml, err := yaml.Marshal(&Values)
	if err != nil {
		return "", err
	}

	filePath := filepath.Join(TempDir, FileName)

	err = ioutil.WriteFile(filePath, valuesYaml, 0644)
	if err != nil {
		return "", err
	}

	return filePath, nil
}

func readModuleValues(module Module) (map[interface{}]interface{}, error) {
	path := filepath.Join(module.Path, "values.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	values, err := readValuesYamlFile(path)
	if err != nil {
		return nil, err
	}
	return values, nil
}

func readModulesValues() (map[interface{}]interface{}, error) {
	path := filepath.Join(WorkingDir, "modules", "values.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return make(map[interface{}]interface{}), nil
	}

	return readValuesYamlFile(path)
}
