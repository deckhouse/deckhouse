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
	"reflect"
	"regexp"
	"strings"
	"time"
)

type Module struct {
	Name          string
	DirectoryName string
	Path          string
}

var (
	// список модулей, найденных в инсталляции
	modulesByName map[string]Module
	// список имен модулей в порядке вызова
	modulesOrder []string

	// values для всех модулей, для всех кластеров
	globalValues map[interface{}]interface{}
	// values, которые генерируют хуки (on-kube-node-change)
	hooksValues map[interface{}]interface{}
	// values для конкретного модуля, для всех кластеров
	globalModulesValues map[string]map[interface{}]interface{}
	// values для всех модулей, для конкретного кластера
	kubeValues map[interface{}]interface{}
	// values для конкретного модуля, для конкретного кластера
	kubeModulesValues map[string]map[interface{}]interface{}

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

	hooksValues = make(map[interface{}]interface{})

	globalValues, err = readValues()
	if err != nil {
		rlog.Errorf("Cannot read values: %s", err)
		os.Exit(1)
	}
	rlog.Debugf("Read global VALUES:\n%s", valuesToString(globalValues))

	globalModulesValues = make(map[string]map[interface{}]interface{})
	for _, module := range modulesByName {
		values, err := readModuleValues(module)
		if err != nil {
			rlog.Errorf("Cannot read module %s global values: %s", module.Name, err)
			os.Exit(1)
		}
		if values != nil {
			globalModulesValues[module.Name] = values
			rlog.Debugf("Read module %s global VALUES:\n%s", module.Name, valuesToString(values))
		}
	}

	res, err := InitKubeValuesManager()
	if err != nil {
		rlog.Errorf("Cannot initialize kube values manager: %s", err)
		os.Exit(1)
	}
	kubeValues = res.Values
	kubeModulesValues = res.ModulesValues
	rlog.Debugf("Read kube VALUES:\n%s", valuesToString(kubeValues))
	for moduleName, kubeModuleValues := range kubeModulesValues {
		rlog.Debugf("Read module %s kube VALUES:\n%s", moduleName, valuesToString(kubeModuleValues))
	}

	InitKubeNodeManager()

	err = InitRegistryManager()
	if err != nil {
		rlog.Errorf("Cannot initialize registry manager: %s", err)
		os.Exit(1)
	}
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
			kubeValues = newKubevalues

			rlog.Infof("Kube values has been updated, rerun all modules ...")

			RunModules()

		case moduleValuesUpdate := <-KubeModuleValuesUpdated:
			kubeModulesValues[moduleValuesUpdate.ModuleName] = moduleValuesUpdate.Values

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
	values, err := RunOnKubeNodeChangedHooks()

	if err != nil {
		retryAll = true
		rlog.Errorf("on-kube-node-change hooks error: %s", err)
		return
	}

	hooksValues = values

	RunModules()
}

func OnKubeNodeChanged() {
	rlog.Infof("Kube node change detected")

	values, err := RunOnKubeNodeChangedHooks()
	if err != nil {
		rlog.Errorf("on-kube-node-change hooks error: %s", err)
		return
	}

	newHooksValues := MergeValues(hooksValues, values)

	if !reflect.DeepEqual(hooksValues, newHooksValues) {
		hooksValues = newHooksValues

		RunModules()
	}
}

// Вызов хуков при изменении опций узлов и самих узлов.
// Таким образом можно подтюнить узлы кластера.
// см. `/global-hooks/on-kube-node-change/*`
func RunOnKubeNodeChangedHooks() (map[interface{}]interface{}, error) {
	hooksDir := filepath.Join(WorkingDir, "global-hooks", "on-kube-node-change")

	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil, nil
	}

	hooksNames, err := readDirectoryExecutableFilesNames(hooksDir)
	if err != nil {
		return nil, err
	}

	var resValues map[interface{}]interface{}

	for _, hookName := range hooksNames {
		rlog.Infof("Running global on-kube-node-change hook %s ...", hookName)

		returnValuesPath := filepath.Join(TempDir, "global.on-kube-node-change.yaml")
		returnValuesFile, err := os.OpenFile(returnValuesPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			return nil, err
		}
		returnValuesFile.Close()

		cmd := exec.Command(filepath.Join(hooksDir, hookName))
		cmd.Env = append(cmd.Env, os.Environ()...)
		cmd.Env = append(
			cmd.Env,
			fmt.Sprintf("RETURN_VALUES_PATH=%s", returnValuesPath),
			fmt.Sprintf("TILLER_NAMESPACE=%s", HelmTillerNamespace()),
		)

		cmd.Dir = WorkingDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err = execCommand(cmd)
		if err != nil {
			return nil, fmt.Errorf("%s FAILED: %s", hookName, err)
		}

		values, err := readValuesYamlFile(returnValuesPath)
		if err != nil {
			return nil, fmt.Errorf("Got bad values yaml from hook %s: %s", hookName, err)
		}

		resValues = MergeValues(resValues, values)
	}

	return resValues, nil
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

func readValuesYamlFile(Path string) (map[interface{}]interface{}, error) {
	valuesYaml, err := ioutil.ReadFile(Path)
	if err != nil {
		return nil, fmt.Errorf("Cannot read %s: %s", Path, err)
	}

	var res map[interface{}]interface{}

	err = yaml.Unmarshal(valuesYaml, &res)
	if err != nil {
		return nil, fmt.Errorf("Bad %s: %s", Path, err)
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

func readValues() (map[interface{}]interface{}, error) {
	path := filepath.Join(WorkingDir, "modules", "values.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return make(map[interface{}]interface{}), nil
	}

	return readValuesYamlFile(path)
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

func valuesToString(Values map[interface{}]interface{}) string {
	valuesYaml, err := yaml.Marshal(&Values)
	if err != nil {
		return fmt.Sprintf("%v", Values)
	}
	return string(valuesYaml)
}
