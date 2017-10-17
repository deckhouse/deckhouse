package main

import (
	"fmt"
	"github.com/romana/rlog"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

var (
	// список модулей, найденных в инсталляции
	modulesNames []string

	// values для всех модулей, для всех кластеров
	values map[string]interface{}
	// values для конкретного модуля, для всех кластеров
	modulesValues map[string]map[string]interface{}
	// values для всех модулей, для конкретного кластера
	kubeValues map[string]interface{}
	// values для конкретного модуля, для конкретного кластера
	kubeModulesValues map[string]map[string]interface{}

	retryModulesQueue []string

	WorkingDir string

	lastRunAt time.Time

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

	modulesNames, err = readModulesNames()
	if err != nil {
		rlog.Errorf("Cannot read antiopa modules: %s", err)
		os.Exit(1)
	}

	values, err = readValues()
	if err != nil {
		rlog.Errorf("Cannot read values: %s", err)
		os.Exit(1)
	}

	modulesValues, err = readModulesValues(modulesNames)
	if err != nil {
		rlog.Errorf("Cannot read modules values: %s", err)
		os.Exit(1)
	}

	retryModulesQueue = make([]string, 0)

	Hostname, err = os.Hostname()
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot get pod name from hostname: %v", err)
		os.Exit(1)
	}

	// TODO Пока для доступа к registry.flant.com передаётся временный токен через переменную среды
	GitlabToken := os.Getenv("GITLAB_TOKEN")
	DockerRegistryInfo["registry.flant.com"]["password"] = GitlabToken

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

	retryModuleTicker := time.NewTicker(time.Duration(30) * time.Second)
	nightRunTicker := time.NewTicker(time.Duration(300) * time.Second)

	retryModulesQueue = make([]string, 0)
	for _, moduleName := range modulesNames {
		vals, err := PrepareModuleValues(moduleName)
		if err != nil {
			rlog.Errorf("Cannot prepare values for module %s: %s", moduleName, err)
			continue
		}

		err = RunModule(moduleName, vals)
		if err != nil {
			rlog.Errorf("Module %s run failed: %s", moduleName, err)
			retryModulesQueue = append(retryModulesQueue, moduleName)
			continue
		}
	}
	lastRunAt = time.Now()

	for {
		select {
		case newKubevalues := <-KubeValuesUpdated:
			kubeValues = newKubevalues
			// runModules()

		case moduleValuesUpdate := <-KubeModuleValuesUpdated:
			kubeModulesValues[moduleValuesUpdate.ModuleName] = moduleValuesUpdate.Values
			// runModules()

		case <-retryModuleTicker.C:
			if len(retryModulesQueue) > 0 {
				retryModuleName := retryModulesQueue[0]
				retryModulesQueue = retryModulesQueue[1:]

				rlog.Infof("Retrying module %s", retryModuleName)

				// runModule(retryModuleName)
			}

		case <-nightRunTicker.C:
			if len(modulesNames) > 0 {
				now := time.Now()
				mskLocation, err := time.LoadLocation("Europe/Moscow")
				if err == nil {
					// Ежедневный запуск в 3:45 по московскому времени
					nightRunTime := time.Date(now.Year(), now.Month(), now.Day(), 3, 45, 0, 0, mskLocation)

					if lastRunAt.Before(nightRunTime) {
						rlog.Infof("Night run modules ...")
						// runModules()
					}
				}
			}
		case newImageId := <-ImageUpdated:
			KubeUpdateDeployment(newImageId)
			// TODO На этом можно выйти из программы, т.к. прилетел новый образ
		}
	}
}

func PrepareModuleValues(ModuleName string) (map[string]interface{}, error) {
	valuesShPath := filepath.Join(WorkingDir, "modules", ModuleName, "values.sh")

	if _, err := os.Stat(valuesShPath); os.IsExist(err) {
		generatedValues, err := runValuesSh(valuesShPath)
		if err != nil {
			return nil, err
		}

		newModuleValues := MergeValues(generatedValues, kubeModulesValues[ModuleName])
		err = SetModuleKubeValues(ModuleName, newModuleValues)
		if err != nil {
			return nil, err
		}
		kubeModulesValues[ModuleName] = newModuleValues
	}

	return MergeValues(values, modulesValues[ModuleName], kubeValues, kubeModulesValues[ModuleName]), nil
}

func runValuesSh(ValuesShPath string) (map[string]interface{}, error) {
	// TODO
	return make(map[string]interface{}), nil
}

func readModulesNames() ([]string, error) {
	modulesDir := filepath.Join(WorkingDir, "modules")

	files, err := ioutil.ReadDir(modulesDir)
	if err != nil {
		return nil, fmt.Errorf("Cannot list modules directory %s: %s", modulesDir, err)
	}

	res := make([]string, 0)
	for _, file := range files {
		res = append(res, file.Name())
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

func readValues() (map[string]interface{}, error) {
	return readValuesYamlFile(filepath.Join(WorkingDir, "values.yaml"))
}

func readModulesValues(ModulesNames []string) (map[string]map[string]interface{}, error) {
	modulesDir := filepath.Join(WorkingDir, "modules")

	res := make(map[string]map[string]interface{})

	var err error

	for _, moduleName := range ModulesNames {
		values, err = readValuesYamlFile(filepath.Join(modulesDir, moduleName, "values.yaml"))
		if err != nil {
			return nil, err
		}
		res[moduleName] = values
	}

	return res, nil
}

// func runModules(scriptsDir string, modules []map[string]string) {
// 	// Сброс очереди на рестарт
// 	retryModulesQueue = make([]map[string]string, 0)

// 	for _, module := range modules {
// 		runModule(scriptsDir, module)
// 	}

// 	lastRunAt = time.Now()
// }

// func runModule(scriptsDir string, module map[string]string) {
// 	var baseArgs []string

// 	entrypoint := module["entrypoint"]
// 	if entrypoint == "" {
// 		entrypoint = "bash"
// 		baseArgs = append(baseArgs, "ctl.sh")
// 	}

// 	isFirstRun := (getModuleStatus(module["name"])["installed"] != "true")
// 	firstRunUserArgs, firstRunUserArgsExist := module["first_run_args"]

// 	if isFirstRun && firstRunUserArgsExist {
// 		args := append(baseArgs, strings.Fields(firstRunUserArgs)...)

// 		cmd := exec.Command(entrypoint, args...)
// 		cmd.Dir = filepath.Join(scriptsDir, "modules", module["name"])
// 		cmd.Stdout = os.Stdout
// 		cmd.Stderr = os.Stderr

// 		rlog.Infof("Running module %s (first run) ...", module["name"])
// 		rlog.Debugf("Module %s command: `%s %s`", module["name"], entrypoint, strings.Join(args, " "))

// 		err := cmd.Run()
// 		if err == nil {
// 			setModuleStatus(module["name"], map[string]string{"installed": "true"})
// 			rlog.Infof("Module %s first run OK", module["name"])
// 		} else {
// 			retryModulesQueue = append(retryModulesQueue, module)
// 			rlog.Errorf("Module %s FAILED: %s", module["name"], err)
// 			return
// 		}
// 	}

// 	args := append(baseArgs, strings.Fields(module["args"])...)
// 	cmd := exec.Command(entrypoint, args...)

// 	cmd.Dir = filepath.Join(scriptsDir, "modules", module["name"])
// 	cmd.Stdout = os.Stdout
// 	cmd.Stderr = os.Stderr

// 	rlog.Infof("Running module %s ...", module["name"])
// 	rlog.Debugf("Module %s command: `%s %s`", module["name"], entrypoint, strings.Join(args, " "))

// 	err := cmd.Run()
// 	if err == nil {
// 		rlog.Infof("Module %s OK", module["name"])
// 	} else {
// 		retryModulesQueue = append(retryModulesQueue, module)
// 		rlog.Errorf("Module %s FAILED: %s", module["name"], err)
// 	}

// 	return
// }

// func getModuleStatus(moduleName string) (res map[string]string) {
// 	p := path.Join(ModulesStatusDir, moduleName)
// 	if _, err := os.Stat(p); os.IsNotExist(err) {
// 		return make(map[string]string)
// 	}

// 	dat, err := ioutil.ReadFile(p)
// 	if err != nil {
// 		return make(map[string]string)
// 	}
// 	if len(dat) > 0 {
// 		dat = dat[:len(dat)-1]
// 	}

// 	if err := json.Unmarshal(dat, &res); err != nil {
// 		return make(map[string]string)
// 	}

// 	return
// }

// func setModuleStatus(moduleName string, moduleStatus map[string]string) error {
// 	dat, err := json.Marshal(moduleStatus)
// 	if err != nil {
// 		return err
// 	}

// 	dat = append(dat, []byte("\n")...)

// 	if _, err := os.Stat(ModulesStatusDir); os.IsNotExist(err) {
// 		if err = os.MkdirAll(ModulesStatusDir, 0777); err != nil {
// 			return err
// 		}
// 	}

// 	if err = ioutil.WriteFile(path.Join(ModulesStatusDir, moduleName), dat, 0644); err != nil {
// 		return err
// 	}

// 	return nil
// }
