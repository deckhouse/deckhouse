package module_manager

import (
	"encoding/json"
	"fmt"
	"github.com/evanphx/json-patch"
	"github.com/gobwas/glob"
	"github.com/romana/rlog"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/deckhouse/antiopa/helm"
	"github.com/deckhouse/deckhouse/antiopa/kube"
	"github.com/deckhouse/deckhouse/antiopa/kube_values_manager"
	"github.com/deckhouse/deckhouse/antiopa/merge_values"
	"github.com/deckhouse/deckhouse/antiopa/utils"
)

type Module struct {
	Name          string
	DirectoryName string
	Path          string
}

type Hook struct {
	Name string
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

func RunModuleBeforeHelmHooks(moduleName string, valuesPath string) error {
	return runModuleHooks("before-helm", moduleName, valuesPath)
}

func RunModuleAfterHelmHooks(moduleName string, valuesPath string) error {
	return runModuleHooks("after-helm", moduleName, valuesPath)
}

func runModuleHooks(orderType string, moduleName string, valuesPath string) error {
	module, hasModule := modulesByName[moduleName]
	if !hasModule {
		return fmt.Errorf("no such module %s", moduleName)
	}

	hooksDir := filepath.Join(module.Path, "hooks", orderType)

	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil
	}

	hooksNames, err := readDirectoryExecutableFilesNames(hooksDir)
	if err != nil {
		return err
	}

	for _, hookName := range hooksNames {
		rlog.Infof("Running module %s %s hook %s ...", moduleName, orderType, hookName)

		var kubeModuleConfigValuesChanged bool

		configVJMV, configVJPV, dynamicVJMV, dynamicVJPV, err := runModuleHook(module.Path, hooksDir, hookName, valuesPath)

		if err != nil {
			return fmt.Errorf("%s hook %s FAILED: %s", orderType, hookName, err)
		}

		if kubeModulesConfigValues[module.Name], kubeModuleConfigValuesChanged, err = merge_values.ApplyJsonMergeAndPatch(kubeModulesConfigValues[module.Name], configVJMV, configVJPV); err != nil {
			return err
		}

		if kubeModuleConfigValuesChanged {
			rlog.Debugf("Updating module %s VALUES in ConfigMap:\n%s", module.Name, valuesToString(kubeModulesConfigValues[module.Name]))
			err = kube_values_manager.SetModuleKubeValues(module.Name, kubeModulesConfigValues[module.Name])
			if err != nil {
				return err
			}
		}

		if modulesDynamicValues[module.Name], _, err = merge_values.ApplyJsonMergeAndPatch(modulesDynamicValues[module.Name], dynamicVJMV, dynamicVJPV); err != nil {
			return err
		}
	}

	return nil
}

func runModuleHook(modulePath, hooksDir, hookName string, valuesPath string) (map[string]interface{}, *jsonpatch.Patch, map[string]interface{}, *jsonpatch.Patch, error) {
	cmd := makeCommand(modulePath, valuesPath, filepath.Join(hooksDir, hookName), []string{})
	return runHook(filepath.Join(TempDir, "values", "modules"), hookName, cmd)
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

		err := execCommand(makeCommand(module.Path, ValuesPath, "helm", []string{"upgrade", helmReleaseName, ".", "--install", "--namespace", helm.TillerNamespace, "--values", ValuesPath}))
		if err != nil {
			return fmt.Errorf("helm FAILED: %s", err)
		}
	} else {
		rlog.Debugf("No helm chart found for module %s in %s", moduleName, chartPath)
	}

	return nil
}

func runGlobalHook(hooksDir, hookName string, valuesPath string) (map[string]interface{}, *jsonpatch.Patch, map[string]interface{}, *jsonpatch.Patch, error) {
	cmd := makeCommand(WorkingDir, valuesPath, filepath.Join(hooksDir, hookName), []string{})
	return runHook(filepath.Join(TempDir, "values", "hooks"), hookName, cmd)
}

func PrepareModuleValues(moduleName string) (map[interface{}]interface{}, error) {
	if _, hasModule := modulesByName[moduleName]; !hasModule {
		return nil, fmt.Errorf("no such module %s", moduleName)
	}
	return merge_values.MergeValues(globalConfigValues, globalModulesConfigValues[moduleName], kubeConfigValues, kubeModulesConfigValues[moduleName], dynamicValues, modulesDynamicValues[moduleName]), nil
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

	cm, err := kube.GetConfigMap()
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
		return nil, fmt.Errorf("cannot list modules directory %s: %s", modulesDir, err)
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

func dumpModuleValuesYaml(moduleName string, values map[interface{}]interface{}) (string, error) {
	return dumpValuesYaml(fmt.Sprintf("%s.yaml", moduleName), values)
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

func readValuesYamlFile(filePath string) (map[interface{}]interface{}, error) {
	valuesYaml, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	var res map[interface{}]interface{}

	err = yaml.Unmarshal(valuesYaml, &res)
	if err != nil {
		return nil, fmt.Errorf("bad %s: %s", filePath, err)
	}

	return res, nil
}

func makeCommand(dir string, valuesPath string, entrypoint string, args []string) *exec.Cmd {
	envs := make([]string, 0)
	envs = append(envs, os.Environ()...)
	envs = append(envs, helm.CommandEnv()...)
	envs = append(envs, fmt.Sprintf("VALUES_PATH=%s", valuesPath))

	return utils.MakeCommand(dir, entrypoint, args, envs)
}

func runHook(tmpDir, hookName string, cmd *exec.Cmd) (map[string]interface{}, *jsonpatch.Patch, map[string]interface{}, *jsonpatch.Patch, error) {
	configValuesJsonMergePath := filepath.Join(tmpDir, hookName, "config_values_json_merge.json")
	if err := createResultFile(configValuesJsonMergePath); err != nil {
		return nil, nil, nil, nil, err
	}

	configValuesJsonPatchPath := filepath.Join(tmpDir, hookName, "config_values_json_patch.json")
	if err := createResultFile(configValuesJsonPatchPath); err != nil {
		return nil, nil, nil, nil, err
	}

	dynamicValuesJsonMergePath := filepath.Join(tmpDir, hookName, "dynamic_values_json_merge.json")
	if err := createResultFile(dynamicValuesJsonMergePath); err != nil {
		return nil, nil, nil, nil, err
	}

	dynamicValuesJsonPatchPath := filepath.Join(tmpDir, hookName, "dynamic_values_json_patch.json")
	if err := createResultFile(dynamicValuesJsonPatchPath); err != nil {
		return nil, nil, nil, nil, err
	}

	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("CONFIG_VALUES_JSON_MERGE_PATH=%s", configValuesJsonMergePath),
		fmt.Sprintf("CONFIG_VALUES_JSON_PATCH_PATH=%s", configValuesJsonPatchPath),
		fmt.Sprintf("DYNAMIC_VALUES_JSON_MERGE_PATH=%s", dynamicValuesJsonMergePath),
		fmt.Sprintf("DYNAMIC_VALUES_JSON_PATCH_PATH=%s", dynamicValuesJsonPatchPath),
	)

	err := execCommand(cmd)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s FAILED: %s", hookName, err)
	}

	configValuesJsonMergeValues, err := readValuesJsonFile(configValuesJsonMergePath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("got bad values json from hook %s: %s", hookName, err)
	}

	configValuesJsonPatchValues, err := readJsonPatchFile(configValuesJsonPatchPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("got bad values json from hook %s: %s", hookName, err)
	}

	dynamicValuesJsonMergeValues, err := readValuesJsonFile(dynamicValuesJsonMergePath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("got bad values json from hook %s: %s", hookName, err)
	}

	dynamicValuesJsonPatchValues, err := readJsonPatchFile(dynamicValuesJsonPatchPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("got bad values json from hook %s: %s", hookName, err)
	}

	return configValuesJsonMergeValues, configValuesJsonPatchValues, dynamicValuesJsonMergeValues, dynamicValuesJsonPatchValues, nil
}

func readDirectoryExecutableFilesNames(Dir string) ([]string, error) {
	files, err := ioutil.ReadDir(Dir)
	if err != nil {
		return nil, err
	}

	res := make([]string, 0)
	for _, file := range files {
		// Тупой тест, что файл executable.
		// Т.к. antiopa всегда работает под root, то этого достаточно.
		isExecutable := !file.IsDir() && (file.Mode()&0111 != 0)

		if isExecutable {
			res = append(res, file.Name())
		} else {
			rlog.Warnf("Ignoring non executable file %s", filepath.Join(Dir, file.Name()))
		}
	}

	return res, nil
}

func valuesToString(values map[interface{}]interface{}) string {
	valuesYaml, err := yaml.Marshal(&values)
	if err != nil {
		return fmt.Sprintf("%v", values)
	}
	return string(valuesYaml)
}

func execCommand(cmd *exec.Cmd) error {
	rlog.Debugf("Executing command in %s: `%s`", cmd.Dir, strings.Join(cmd.Args, " "))
	return cmd.Run()
}

func dumpValuesYaml(fileName string, values map[interface{}]interface{}) (string, error) {
	valuesYaml, err := yaml.Marshal(&values)
	if err != nil {
		return "", err
	}

	filePath := filepath.Join(TempDir, fileName)

	err = ioutil.WriteFile(filePath, valuesYaml, 0644)
	if err != nil {
		return "", err
	}

	return filePath, nil
}
func createResultFile(filePath string) error {
	os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil
	}

	file.Close()
	return nil
}

func readValuesJsonFile(filePath string) (map[string]interface{}, error) {
	valuesJson, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	if len(valuesJson) == 0 {
		return make(map[string]interface{}), nil
	}

	var res map[string]interface{}

	err = json.Unmarshal(valuesJson, &res)
	if err != nil {
		return nil, fmt.Errorf("bad %s: %s", filePath, err)
	}

	return res, nil
}
func readJsonPatchFile(filePath string) (*jsonpatch.Patch, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	patch, err := jsonpatch.DecodePatch(data)
	if err != nil {
		return nil, fmt.Errorf("bad %s: %s", filePath, err)
	}

	return &patch, nil
}
func dumpGlobalHooksValuesYaml() (string, error) {
	return dumpValuesYaml("global-hooks.yaml", prepareGlobalValues())
}
func prepareGlobalValues() map[interface{}]interface{} {
	return merge_values.MergeValues(globalConfigValues, kubeConfigValues, dynamicValues)
}
