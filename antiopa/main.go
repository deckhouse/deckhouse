package main

import (
	"fmt"
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

	globalValues, err = readModulesValues()
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

func valuesToString(Values map[interface{}]interface{}) string {
	valuesYaml, err := yaml.Marshal(&Values)
	if err != nil {
		return fmt.Sprintf("%v", Values)
	}
	return string(valuesYaml)
}
