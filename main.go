package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/romana/rlog"
)

var (
	lastScriptsDir    string
	lastModules       []map[string]string
	retryModulesQueue []map[string]string

	ModulesStatusDir string
	WorkingDir       string
	RunDir           string

	lastRunAt time.Time

	// Имя хоста совпадает с именем пода. Можно использовать для запросов API
	Hostname string
)

func main() {
	Init()
	Run()
}

func Init() {
	rlog.Info("Init")

	lastScriptsDir = ""
	lastModules = make([]map[string]string, 0)
	retryModulesQueue = make([]map[string]string, 0)

	WorkingDir, err := os.Getwd()
	if err != nil {
		rlog.Error("MAIN Fatal: Cannot determine antiopa working dir: %s", err)
		os.Exit(1)
	}

	RunDir = path.Join(WorkingDir, "antiopa-run")
	ModulesStatusDir = path.Join(RunDir, "module-status")

	Hostname, err = os.Hostname()
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot get pod name from hostname: %v", err)
		os.Exit(1)
	}

	InitKube()
	InitConfigManager()
	InitScriptsManager()
	InitRegistryManager()
}

func Run() {
	rlog.Info("Run")

	go RunConfigManager()
	go RunScriptsManager()
	go RunRegistryManager()

	retryModuleTicker := time.NewTicker(time.Duration(30) * time.Second)
	nightRunTicker := time.NewTicker(time.Duration(300) * time.Second)

	for {
		select {
		case modules := <-ModulesUpdated:
			lastModules = modules

			if lastScriptsDir != "" && len(lastModules) > 0 {
				runModules(lastScriptsDir, lastModules)
			}

		case <-retryModuleTicker.C:
			if lastScriptsDir != "" && len(retryModulesQueue) > 0 {
				retryModule := retryModulesQueue[0]
				retryModulesQueue = retryModulesQueue[1:]

				rlog.Infof("Retrying module %s", retryModule["name"])

				runModule(lastScriptsDir, retryModule)
			}

		case <-nightRunTicker.C:
			if lastScriptsDir != "" && len(lastModules) > 0 {
				now := time.Now()
				mskLocation, err := time.LoadLocation("Europe/Moscow")
				if err == nil {
					// Ежедневный запуск в 3:45 по московскому времени
					nightRunTime := time.Date(now.Year(), now.Month(), now.Day(), 3, 45, 0, 0, mskLocation)

					if lastRunAt.Before(nightRunTime) {
						rlog.Infof("Night run modules ...")
						runModules(lastScriptsDir, lastModules)
					}
				}
			}
		case newImageId := <-ImageUpdated:
			KubeUpdateDeployment(newImageId)
			// TODO На этом можно выйти из программы, т.к. прилетел новый образ
		}
	}
}

func runModules(scriptsDir string, modules []map[string]string) {
	// Сброс очереди на рестарт
	retryModulesQueue = make([]map[string]string, 0)

	for _, module := range modules {
		runModule(scriptsDir, module)
	}

	lastRunAt = time.Now()
}

func runModule(scriptsDir string, module map[string]string) {
	var baseArgs []string

	entrypoint := module["entrypoint"]
	if entrypoint == "" {
		entrypoint = "bash"
		baseArgs = append(baseArgs, "ctl.sh")
	}

	isFirstRun := (getModuleStatus(module["name"])["installed"] != "true")
	firstRunUserArgs, firstRunUserArgsExist := module["first_run_args"]

	if isFirstRun && firstRunUserArgsExist {
		args := append(baseArgs, strings.Fields(firstRunUserArgs)...)

		cmd := exec.Command(entrypoint, args...)
		cmd.Dir = filepath.Join(scriptsDir, "modules", module["name"])
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		rlog.Infof("Running module %s (first run) ...", module["name"])
		rlog.Debugf("Module %s command: `%s %s`", module["name"], entrypoint, strings.Join(args, " "))

		err := cmd.Run()
		if err == nil {
			setModuleStatus(module["name"], map[string]string{"installed": "true"})
			rlog.Infof("Module %s first run OK", module["name"])
		} else {
			retryModulesQueue = append(retryModulesQueue, module)
			rlog.Errorf("Module %s FAILED: %s", module["name"], err)
			return
		}
	}

	args := append(baseArgs, strings.Fields(module["args"])...)
	cmd := exec.Command(entrypoint, args...)

	cmd.Dir = filepath.Join(scriptsDir, "modules", module["name"])
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	rlog.Infof("Running module %s ...", module["name"])
	rlog.Debugf("Module %s command: `%s %s`", module["name"], entrypoint, strings.Join(args, " "))

	err := cmd.Run()
	if err == nil {
		rlog.Infof("Module %s OK", module["name"])
	} else {
		retryModulesQueue = append(retryModulesQueue, module)
		rlog.Errorf("Module %s FAILED: %s", module["name"], err)
	}

	return
}

func getModuleStatus(moduleName string) (res map[string]string) {
	p := path.Join(ModulesStatusDir, moduleName)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return make(map[string]string)
	}

	dat, err := ioutil.ReadFile(p)
	if err != nil {
		return make(map[string]string)
	}
	if len(dat) > 0 {
		dat = dat[:len(dat)-1]
	}

	if err := json.Unmarshal(dat, &res); err != nil {
		return make(map[string]string)
	}

	return
}

func setModuleStatus(moduleName string, moduleStatus map[string]string) error {
	dat, err := json.Marshal(moduleStatus)
	if err != nil {
		return err
	}

	dat = append(dat, []byte("\n")...)

	if _, err := os.Stat(ModulesStatusDir); os.IsNotExist(err) {
		if err = os.MkdirAll(ModulesStatusDir, 0777); err != nil {
			return err
		}
	}

	if err = ioutil.WriteFile(path.Join(ModulesStatusDir, moduleName), dat, 0644); err != nil {
		return err
	}

	return nil
}
