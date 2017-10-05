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
		rlog.Error("Cannot determine antiopa working dir: %s", err)
		os.Exit(1)
	}

	RunDir = path.Join(WorkingDir, "antiopa-run")
	ModulesStatusDir = path.Join(RunDir, "module-status")

	InitKube()
	InitConfigManager()
	InitScriptsManager()
}

func RunModule(scriptsDir string, module map[string]string) error {
	var args []string

	entrypoint := module["entrypoint"]
	if entrypoint == "" {
		entrypoint = "bash"
		args = append(args, "ctl.sh")
	}

	isFirstRun := (getModuleStatus(module["name"])["installed"] != "true")
	var userArgs string
	if isFirstRun && module["first_run_args"] != "" {
		userArgs = module["first_run_args"]
	} else {
		userArgs = module["args"]
	}
	args = append(args, strings.Fields(userArgs)...)

	cmd := exec.Command(entrypoint, args...)
	cmd.Dir = filepath.Join(scriptsDir, "modules", module["name"])
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	rlog.Infof("Running module %s ...", module["name"])
	rlog.Debugf("Module %s command: `%s %s", module["name"], entrypoint, args)

	err := cmd.Run()
	if err == nil {
		if isFirstRun {
			setModuleStatus(module["name"], map[string]string{"installed": "true"})
		}

		rlog.Infof("Module %s OK", module["name"])
	} else {
		retryModulesQueue = append(retryModulesQueue, module)

		rlog.Errorf("Module %s FAILED: %s", module["name"], err)
	}

	return err
}

func RunModules(scriptsDir string, modules []map[string]string) {
	if scriptsDir == "" {
		return
	}
	for _, module := range modules {
		RunModule(scriptsDir, module)
	}
}

func Run() {
	rlog.Info("Run")

	go RunConfigManager()
	go RunScriptsManager()

	retryModuleTicker := time.NewTicker(time.Duration(30) * time.Second)

	for {
		select {
		case modules := <-ModulesUpdated:
			lastModules = modules

			// Сброс очереди на рестарт
			retryModulesQueue = make([]map[string]string, 0)

			RunModules(lastScriptsDir, lastModules)

		case upd := <-ScriptsUpdated:
			if lastScriptsDir != "" {
				os.RemoveAll(lastScriptsDir)
			}
			lastScriptsDir = upd.Path

			// Сброс очереди на рестарт
			retryModulesQueue = make([]map[string]string, 0)

			RunModules(lastScriptsDir, lastModules)

		case <-retryModuleTicker.C:
			if len(retryModulesQueue) > 0 && lastScriptsDir != "" {
				retryModule := retryModulesQueue[0]
				retryModulesQueue = retryModulesQueue[1:]

				rlog.Infof("Retrying module %s", retryModule["name"])

				RunModule(lastScriptsDir, retryModule)
			}
		}
	}
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
