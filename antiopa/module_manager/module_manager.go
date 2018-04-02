package module_manager

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	retryModulesNamesQueue []string
	retryAll               bool

	// список модулей, найденных в инсталляции
	modulesByName map[string]Module
	// список имен модулей в порядке вызова
	modulesOrder []string

	hooksByName                  map[string]Hook
	beforeHelmHooksOrderByModule map[string][]string
	afterHelmHooksOrderByModule  map[string][]string

	// values для всех модулей, для всех кластеров
	globalConfigValues map[interface{}]interface{}
	// values для конкретного модуля, для всех кластеров
	globalModulesConfigValues map[string]map[interface{}]interface{}
	// values для всех модулей, для конкретного кластера
	kubeConfigValues map[interface{}]interface{}
	// values для конкретного модуля, для конкретного кластера
	kubeModulesConfigValues map[string]map[interface{}]interface{}
	// dynamic-values для всех модулей, для всех кластеров
	dynamicValues map[interface{}]interface{}
	// dynamic-values для конкретного модуля, для всех кластеров
	modulesDynamicValues map[string]map[interface{}]interface{}

	valuesChanged bool

	WorkingDir string
	TempDir    string
)

func Init(workingDir string, tempDir string) error {
	TempDir = tempDir
	WorkingDir = workingDir
	modulesDir := filepath.Join(WorkingDir, "modules")

	files, err := ioutil.ReadDir(modulesDir)
	if err != nil {
		return fmt.Errorf("cannot list modules directory %s: %s", modulesDir, err)
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

				isEnabled, err := isModuleEnabled(module)
				if err != nil {
					return err
				}
				if isEnabled {
					res = append(res, module)
				}
			} else {
				badModulesDirs = append(badModulesDirs, filepath.Join(modulesDir, file.Name()))
			}
		}
	}

	if len(badModulesDirs) > 0 {
		return fmt.Errorf("bad module directory names, must match regex `%s`: %s", validmoduleName, strings.Join(badModulesDirs, ", "))
	}

	return nil
}

func isModuleEnabled(module Module) (bool, error) {
	enabledScriptPath := filepath.Join(module.DirectoryName, "enabled")

	_, err := os.Stat(enabledScriptPath)
	if os.IsNotExist(err) {
		return true, nil
	} else if err != nil {
		return false, err
	}

	return true, nil

	// TODO
	// cmd := exec.Command(enabledScriptPath, args...)
	// cmd.Env = append(cmd.Env, os.Environ()...)
	// cmd.Env = append(
	// 	cmd.Env,
	// 	fmt.Sprintf("TILLER_NAMESPACE=%s", HelmTillerNamespace()),
	// )
	// cmd.Dir = dir
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
}
