package main

import (
	"encoding/json"
	"fmt"
	"github.com/evanphx/json-patch"
	notordinaryyaml "github.com/ghodss/yaml"
	"github.com/romana/rlog"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

var (
	// список модулей, найденных в инсталляции
	modulesByName map[string]Module
	// список имен модулей в порядке вызова
	modulesOrder []string

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

	valuesChanged        bool
	modulesValuesChanged map[string]bool

	retryModulesNamesQueue []string
	retryAll               bool

	WorkingDir string
	TempDir    string

	// Имя хоста совпадает с именем пода. Можно использовать для запросов API
	Hostname string
)

func main() {
	Init()
	Run()
}

func Init() {
	rlog.Debug("Init")

	var err error

	WorkingDir, err = os.Getwd()
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot determine antiopa working dir: %s", err)
		os.Exit(1)
	}

	TempDir, err = ioutil.TempDir("", "antiopa-")
	if err != nil {
		rlog.Errorf("MAIN Fatal: cannot create antiopa temporary dir: %s", err)
		os.Exit(1)
	}

	retryModulesNamesQueue = make([]string, 0)
	retryAll = false

	Hostname, err = os.Hostname()
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot get pod name from hostname: %v", err)
		os.Exit(1)
	}

	InitKube()
	InitHelm()

	// Initialize global enabled-modules index with descriptors
	modules, err := getEnabledModules()
	if err != nil {
		rlog.Errorf("Cannot detect enabled antiopa modules: %s", err)
		os.Exit(1)
	}
	if len(modules) == 0 {
		rlog.Warnf("No modules enabled")
	}
	modulesOrder = make([]string, 0)
	modulesByName = make(map[string]Module)
	for _, module := range modules {
		modulesByName[module.Name] = module
		modulesOrder = append(modulesOrder, module.Name)
		rlog.Debugf("Using module %s", module.Name)
	}

	globalConfigValues, err = readModulesValues()
	if err != nil {
		rlog.Errorf("Cannot read values: %s", err)
		os.Exit(1)
	}
	rlog.Debugf("Read global VALUES:\n%s", valuesToString(globalConfigValues))

	globalModulesConfigValues = make(map[string]map[interface{}]interface{})
	for _, module := range modulesByName {
		values, err := readModuleValues(module)
		if err != nil {
			rlog.Errorf("Cannot read module %s global values: %s", module.Name, err)
			os.Exit(1)
		}
		if values != nil {
			globalModulesConfigValues[module.Name] = values
			rlog.Debugf("Read module %s global VALUES:\n%s", module.Name, valuesToString(values))
		}
	}

	res, err := InitKubeValuesManager()
	if err != nil {
		rlog.Errorf("Cannot initialize kube values manager: %s", err)
		os.Exit(1)
	}
	kubeConfigValues = res.Values
	kubeModulesConfigValues = res.ModulesValues
	rlog.Debugf("Read kube VALUES:\n%s", valuesToString(kubeConfigValues))
	for moduleName, kubeModuleValues := range kubeModulesConfigValues {
		rlog.Debugf("Read module %s kube VALUES:\n%s", moduleName, valuesToString(kubeModuleValues))
	}

	InitKubeNodeManager()

	err = InitRegistryManager()
	if err != nil {
		rlog.Errorf("Cannot initialize registry manager: %s", err)
		os.Exit(1)
	}

	dynamicValues = make(map[interface{}]interface{})
	modulesDynamicValues = make(map[string]map[interface{}]interface{})
	modulesValuesChanged = make(map[string]bool)
}

func Run() {
	rlog.Debug("Run")

	go RunKubeValuesManager()
	go RunKubeNodeManager()
	go RunRegistryManager()

	RunAll()

	retryTicker := time.NewTicker(time.Duration(30) * time.Second)

	for {
		select {
		case newKubevalues := <-KubeValuesUpdated:
			kubeConfigValues = newKubevalues

			rlog.Infof("Kube values has been updated, rerun all modules ...")

			RunModules()

		case moduleValuesUpdate := <-KubeModuleValuesUpdated:
			kubeModulesConfigValues[moduleValuesUpdate.ModuleName] = moduleValuesUpdate.Values

			rlog.Infof("Module %s kube values has been updated, rerun ...")

			RunModule(moduleValuesUpdate.ModuleName)

		case <-KubeNodeChanged:
			OnKubeNodeChanged()

		case <-retryTicker.C:
			if retryAll {
				retryAll = false

				rlog.Infof("Retrying all modules ...")

				RunAll()
			} else if len(retryModulesNamesQueue) > 0 {
				retryModuleName := retryModulesNamesQueue[0]
				retryModulesNamesQueue = retryModulesNamesQueue[1:]

				rlog.Infof("Retrying module %s ...", retryModuleName)

				RunModule(retryModuleName)
			}

		case newImageId := <-ImageUpdated:
			err := KubeUpdateDeployment(newImageId)
			if err == nil {
				rlog.Infof("KUBE deployment update successful, exiting ...")
				os.Exit(1)
			} else {
				rlog.Errorf("KUBE deployment update error: %s", err)
			}
		}
	}
}

func RunAll() {
	if err := RunOnKubeNodeChangedHooks(); err != nil {
		retryAll = true
		rlog.Errorf("on-kube-node-change hooks error: %s", err)
		return
	}

	RunModules()
}

func OnKubeNodeChanged() {
	rlog.Infof("Kube node change detected")

	if err := RunOnKubeNodeChangedHooks(); err != nil {
		rlog.Errorf("on-kube-node-change hooks error: %s", err)
		return
	}

	if valuesChanged {
		rlog.Debug("Global values changed: run all modules")
		RunModules()
	} else {
		for _, moduleName := range modulesOrder {
			if changed, exist := modulesValuesChanged[moduleName]; exist && changed {
				rlog.Debugf("Module `%s` values changed: run module", moduleName)
				RunModule(moduleName)
			}
		}
	}

	valuesChanged = false
	modulesValuesChanged = make(map[string]bool)
}

// Вызов хуков при изменении опций узлов и самих узлов.
// Таким образом можно подтюнить узлы кластера.
// см. `/global-hooks/on-kube-node-change/*`
func RunOnKubeNodeChangedHooks() error {
	globalHooksValuesPath, err := dumpGlobalHooksValuesYaml()
	if err != nil {
		return err
	}

	hooksDir := filepath.Join(WorkingDir, "global-hooks", "on-kube-node-change")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil
	}

	hooksNames, err := readDirectoryExecutableFilesNames(hooksDir)
	if err != nil {
		return err
	}

	for _, hookName := range hooksNames {
		rlog.Infof("Running global on-kube-node-change hook %s ...", hookName)

		configVJMV, configVJPV, dynamicVJMV, dynamicVJPV, err := runGlobalHook(hooksDir, hookName, globalHooksValuesPath)
		if err != nil {
			return err
		}

		var kubeConfigValuesChanged, globalConfigValuesChanged bool

		if kubeConfigValues, kubeConfigValuesChanged, err = applyJsonMergeAndPatch(kubeConfigValues, configVJMV, configVJPV); err != nil {
			return err
		}

		if err := SetKubeValues(kubeConfigValues); err != nil {
			return err
		}

		if dynamicValues, globalConfigValuesChanged, err = applyJsonMergeAndPatch(globalConfigValues, dynamicVJMV, dynamicVJPV); err != nil {
			return err
		}

		valuesChanged = kubeConfigValuesChanged || globalConfigValuesChanged
	}

	return nil
}

func dumpGlobalHooksValuesYaml() (string, error) {
	return dumpValuesYaml("global-hooks.yaml", prepareGlobalValues())
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

func prepareGlobalValues() map[interface{}]interface{} {
	return MergeValues(globalConfigValues, kubeConfigValues, dynamicValues)
}

func execCommand(cmd *exec.Cmd) error {
	rlog.Debugf("Executing command in %s: `%s`", cmd.Dir, strings.Join(cmd.Args, " "))
	return cmd.Run()
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

func applyJsonMergeAndPatch(values map[interface{}]interface{}, mergeJsonValues map[string]interface{}, patch *jsonpatch.Patch) (map[interface{}]interface{}, bool, error) {
	var resValues map[interface{}]interface{}
	var err error

	if mergeJsonValues != nil {
		resValues = MergeValues(values, jsonValuesToValues(mergeJsonValues))
	}

	if patch != nil {
		if resValues, err = applyJsonPatch(resValues, patch); err != nil {
			return nil, false, err
		}
	}

	valuesChanged := !reflect.DeepEqual(values, resValues)

	return resValues, valuesChanged, nil
}

func jsonValuesToValues(jsonValues map[string]interface{}) map[interface{}]interface{} {
	values := make(map[interface{}]interface{})
	for key, value := range jsonValues {
		values[key] = value
	}
	return values
}

func applyJsonPatch(values map[interface{}]interface{}, patch *jsonpatch.Patch) (map[interface{}]interface{}, error) {
	yamlDoc, err := yaml.Marshal(values)
	if err != nil {
		return nil, err
	}

	jsonDoc, err := notordinaryyaml.YAMLToJSON(yamlDoc)
	if err != nil {
		return nil, err
	}

	resJsonDoc, err := patch.Apply(jsonDoc)
	if err != nil {
		return nil, err
	}

	resJsonValues := make(map[string]interface{})
	if err = json.Unmarshal(resJsonDoc, &resJsonValues); err != nil {
		return nil, err
	}

	resValues := jsonValuesToValues(resJsonValues)

	return resValues, nil
}

func SetKubeValues(_ map[interface{}]interface{}) error {
	return nil
}

func runGlobalHook(hooksDir, hookName string, valuesPath string) (map[string]interface{}, *jsonpatch.Patch, map[string]interface{}, *jsonpatch.Patch, error) {
	cmd := makeCommand(WorkingDir, valuesPath, filepath.Join(hooksDir, hookName), []string{})
	return runHook(filepath.Join(TempDir, "values", "hooks"), hookName, cmd)
}

func makeCommand(dir string, valuesPath string, entrypoint string, args []string) *exec.Cmd {
	cmd := exec.Command(entrypoint, args...)
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("VALUES_PATH=%s", valuesPath),
		fmt.Sprintf("TILLER_NAMESPACE=%s", HelmTillerNamespace()),
	)

	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd
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

func valuesToString(values map[interface{}]interface{}) string {
	valuesYaml, err := yaml.Marshal(&values)
	if err != nil {
		return fmt.Sprintf("%v", values)
	}
	return string(valuesYaml)
}
