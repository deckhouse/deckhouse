package main

import (
	"bytes"
	"fmt"
	"github.com/romana/rlog"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	// список модулей, найденных в инсталляции
	modulesNames []string

	// values для всех модулей, для всех кластеров
	globalValues map[string]interface{}
	// values для конкретного модуля, для всех кластеров
	globalModulesValues map[string]map[string]interface{}
	// values для всех модулей, для конкретного кластера
	kubeValues map[string]interface{}
	// values для конкретного модуля, для конкретного кластера
	kubeModulesValues map[string]map[string]interface{}

	retryModulesQueue []string

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

	modulesNames, err = readModulesNames()
	if err != nil {
		rlog.Errorf("Cannot read antiopa modules: %s", err)
		os.Exit(1)
	}
	for _, moduleName := range modulesNames {
		rlog.Debugf("Found module %s", moduleName)
	}

	globalValues, err = readValues()
	if err != nil {
		rlog.Errorf("Cannot read values: %s", err)
		os.Exit(1)
	}
	rlog.Debugf("Initialized global VALUES: %s", globalValues)

	globalModulesValues, err = readModulesValues(modulesNames)
	if err != nil {
		rlog.Errorf("Cannot read modules values: %s", err)
		os.Exit(1)
	}
	rlog.Debugf("Initialized global modules VALUES: %s", globalModulesValues)

	retryModulesQueue = make([]string, 0)

	Hostname, err = os.Hostname()
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot get pod name from hostname: %v", err)
		os.Exit(1)
	}

	InitKube()

	res, err := InitKubeValuesManager()
	if err != nil {
		rlog.Errorf("Cannot initialize kube values manager: %s", err)
		os.Exit(1)
	}
	kubeValues = res.Values
	kubeModulesValues = res.ModulesValues

	InitKubeNodeManager()
	InitRegistryManager()
}

func Run() {
	rlog.Debug("Run")

	go RunKubeValuesManager()
	go RunKubeNodeManager()
	go RunRegistryManager()

	RunModules()

	retryModuleTicker := time.NewTicker(time.Duration(30) * time.Second)

	for {
		select {
		case newKubevalues := <-KubeValuesUpdated:
			kubeValues = newKubevalues
			RunModules()

		case moduleValuesUpdate := <-KubeModuleValuesUpdated:
			kubeModulesValues[moduleValuesUpdate.ModuleName] = moduleValuesUpdate.Values

			rlog.Infof("Module %s kube values has been updated, rerun ...")

			RunModule(moduleValuesUpdate.ModuleName)

		case <-retryModuleTicker.C:
			if len(retryModulesQueue) > 0 {
				retryModuleName := retryModulesQueue[0]
				retryModulesQueue = retryModulesQueue[1:]

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

func RunModules() {
	retryModulesQueue = make([]string, 0)
	for _, moduleName := range modulesNames {
		RunModule(moduleName)
	}
}

func RunModule(ModuleName string) {
	vals, err := PrepareModuleValues(ModuleName)
	if err != nil {
		rlog.Errorf("Cannot prepare values for module %s: %s", ModuleName, err)
		retryModulesQueue = append(retryModulesQueue, ModuleName)
		return
	}
	rlog.Debugf("Prepared module %s VALUES: %v", ModuleName, vals)

	valuesPath, err := dumpModuleValuesYaml(ModuleName, vals)
	if err != nil {
		rlog.Errorf("Cannot dump values yaml for module %s: %s", ModuleName, err)
		retryModulesQueue = append(retryModulesQueue, ModuleName)
		return
	}

	err = RunModuleBeforeHelmHooks(ModuleName, valuesPath)
	if err != nil {
		rlog.Errorf("Module %s before-helm hooks error: %s", ModuleName, err)
		retryModulesQueue = append(retryModulesQueue, ModuleName)
		return
	}

	err = RunModuleHelmOrEntrypoint(ModuleName, valuesPath)
	if err != nil {
		rlog.Errorf("Module %s run error: %s", ModuleName, err)
		retryModulesQueue = append(retryModulesQueue, ModuleName)
	}

	err = RunModuleAfterHelmHooks(ModuleName, valuesPath)
	if err != nil {
		rlog.Errorf("Module %s after-helm hooks error: %s", ModuleName, err)
		retryModulesQueue = append(retryModulesQueue, ModuleName)
		return
	}
}

func RunModuleBeforeHelmHooks(ModuleName string, ValuesPath string) error {
	moduleDir := filepath.Join(WorkingDir, "modules", ModuleName)
	hooksDir := filepath.Join(moduleDir, "before-helm")

	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil
	}

	hooksNames, err := readDirectoryFilesNames(hooksDir)
	if err != nil {
		return err
	}

	for _, hookName := range hooksNames {
		rlog.Infof("Running module %s before-helm hook %s ...", ModuleName, hookName)

		err := execCommand(makeModuleCommand(moduleDir, ValuesPath, "bin/bash", []string{filepath.Join(hooksDir, hookName)}))
		if err != nil {
			return fmt.Errorf("before-helm hook %s FAILED: %s", hookName, err)
		}
	}

	return nil
}

func RunModuleAfterHelmHooks(ModuleName string, ValuesPath string) error {
	moduleDir := filepath.Join(WorkingDir, "modules", ModuleName)
	hooksDir := filepath.Join(moduleDir, "after-helm")

	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil
	}

	hooksNames, err := readDirectoryFilesNames(hooksDir)
	if err != nil {
		return err
	}

	for _, hookName := range hooksNames {
		rlog.Infof("Running module %s after-helm hook %s ...", ModuleName, hookName)

		err := execCommand(makeModuleCommand(moduleDir, ValuesPath, "bin/bash", []string{filepath.Join(hooksDir, hookName)}))
		if err != nil {
			return fmt.Errorf("after-helm hook %s FAILED: %s", hookName, err)
		}
	}

	return nil
}

func RunModuleHelmOrEntrypoint(ModuleName string, ValuesPath string) error {
	moduleDir := filepath.Join(WorkingDir, "modules", ModuleName)

	rlog.Debugf("moduleDir = %s", moduleDir)

	if _, err := os.Stat(filepath.Join(moduleDir, "Chart.yaml")); !os.IsNotExist(err) {
		rlog.Infof("Running module %s helm ...", ModuleName)

		helmReleaseName := fmt.Sprintf("antiopa-%s", ModuleName)

		err := execCommand(makeModuleCommand(moduleDir, ValuesPath, "helm", []string{"upgrade", helmReleaseName, ".", "--install", "--values", ValuesPath}))
		if err != nil {
			return fmt.Errorf("helm FAILED: %s", err)
		}
	} else if _, err := os.Stat(filepath.Join(moduleDir, "ctl.sh")); !os.IsNotExist(err) {
		rlog.Infof("Running module %s ctl.sh ...", ModuleName)

		err := execCommand(makeModuleCommand(moduleDir, ValuesPath, "/bin/bash", []string{"ctl.sh"}))
		if err != nil {
			return fmt.Errorf("ctl.sh FAILED: %s", err)
		}
	} else {
		rlog.Warnf("No helm chart or ctl.sh found for module %s", ModuleName)
	}

	return nil
}

func PrepareModuleValues(ModuleName string) (map[string]interface{}, error) {
	moduleDir := filepath.Join(WorkingDir, "modules", ModuleName)
	valuesShPath := filepath.Join(moduleDir, "values.sh")

	if _, err := os.Stat(valuesShPath); !os.IsNotExist(err) {
		rlog.Debugf("Running values generator %s ...", valuesShPath)

		var valuesYamlBuffer bytes.Buffer
		cmd := exec.Command("/bin/bash", []string{valuesShPath}...)
		cmd.Env = append(cmd.Env, os.Environ()...)
		cmd.Dir = moduleDir
		cmd.Stdout = &valuesYamlBuffer
		err := execCommand(cmd)
		if err != nil {
			return nil, fmt.Errorf("Values generator %s error: %s", valuesShPath, err)
		}

		var generatedValues map[string]interface{}
		err = yaml.Unmarshal(valuesYamlBuffer.Bytes(), &generatedValues)
		if err != nil {
			return nil, fmt.Errorf("Got bad yaml from values generator %s: %s", valuesShPath, err)
		}
		rlog.Debugf("got VALUES from values.sh: %v", generatedValues)

		newModuleValues := MergeValues(generatedValues, kubeModulesValues[ModuleName])

		rlog.Debugf("Setting module %s values in ConfigMap: %v", ModuleName, newModuleValues)

		err = SetModuleKubeValues(ModuleName, newModuleValues)
		if err != nil {
			return nil, err
		}
		kubeModulesValues[ModuleName] = newModuleValues
	}

	return MergeValues(globalValues, globalModulesValues[ModuleName], kubeValues, kubeModulesValues[ModuleName]), nil
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

func readDirectoryFilesNames(Dir string) ([]string, error) {
	files, err := ioutil.ReadDir(Dir)
	if err != nil {
		return nil, err
	}

	res := make([]string, 0)
	for _, file := range files {
		if !file.IsDir() {
			res = append(res, file.Name())
		}
	}

	return res, nil
}

func readModulesNames() ([]string, error) {
	modulesDir := filepath.Join(WorkingDir, "modules")

	files, err := ioutil.ReadDir(modulesDir)
	if err != nil {
		return nil, fmt.Errorf("Cannot list modules directory %s: %s", modulesDir, err)
	}

	res := make([]string, 0)
	for _, file := range files {
		if file.IsDir() {
			res = append(res, file.Name())
		}
	}

	return res, nil
}

func readValuesYamlFile(Path string) (map[string]interface{}, error) {
	valuesYaml, err := ioutil.ReadFile(Path)
	if err != nil {
		return nil, fmt.Errorf("Cannot read %s: %s", Path, err)
	}

	var res map[string]interface{}

	err = yaml.Unmarshal(valuesYaml, &res)
	if err != nil {
		return nil, fmt.Errorf("Bad %s: %s", Path, err)
	}

	return res, nil
}

func dumpModuleValuesYaml(ModuleName string, Values map[string]interface{}) (string, error) {
	return dumpValuesYaml(fmt.Sprintf("%s.yaml", ModuleName), Values)
}

func dumpValuesYaml(FileName string, Values map[string]interface{}) (string, error) {
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

func readValues() (map[string]interface{}, error) {
	path := filepath.Join(WorkingDir, "modules", "values.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return make(map[string]interface{}), nil
	}

	return readValuesYamlFile(path)
}

func readModulesValues(ModulesNames []string) (map[string]map[string]interface{}, error) {
	modulesDir := filepath.Join(WorkingDir, "modules")

	res := make(map[string]map[string]interface{})

	for _, moduleName := range ModulesNames {
		path := filepath.Join(modulesDir, moduleName, "values.yaml")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		values, err := readValuesYamlFile(path)
		if err != nil {
			return nil, err
		}
		res[moduleName] = values
	}

	return res, nil
}
