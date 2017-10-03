package main

import (
	"errors"
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
)

func Init() {
	rlog.Info("Init")

	lastScriptsDir = ""
	lastModules = make([]map[string]string, 0)
	retryModulesQueue = make([]map[string]string, 0)

	InitKube()
	InitConfigManager()
	InitScriptsManager()
}

func prepareScripts(commit string) (string, error) {
	var err error
	var cmd *exec.Cmd

	if len(lastKnownRepo) == 0 {
		return "", errors.New("Repo is not initialized yet")
	}

	dir, err := ioutil.TempDir("", "antiopa-scripts-tree-")
	if err != nil {
		return "", err
	}

	cmd = exec.Command("git", "clone", lastKnownRepo["url"], dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return "", err
	}

	cmd = exec.Command("git", "--git-dir", path.Join(dir, ".git"), "checkout", commit)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return "", err
	}

	return dir, nil
}

func RunModule(scriptsDir string, module map[string]string) error {
	entrypoint := module["entrypoint"]
	if entrypoint == "" {
		entrypoint = "./ctl.sh"
	} else {
		entrypoint = "./" + entrypoint
	}

	cmd := exec.Command(entrypoint, strings.Fields(module["args"])...)
	cmd.Dir = filepath.Join(scriptsDir, "modules", module["name"])
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	rlog.Infof("Running module %s ...", module["name"])
	err := cmd.Run()
	if err == nil {
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

		case commit := <-ScriptsCommitted:
			// TODO: получать repo-dir+commit из канала

			scriptsDir, err := prepareScripts(commit)
			if err != nil {
				rlog.Errorf("Unable to prepare scripts from repo %v at commit %s", lastKnownRepo, commit)
			}

			if lastScriptsDir != "" {
				os.RemoveAll(lastScriptsDir)
			}
			lastScriptsDir = scriptsDir

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

func main() {
	Init()
	Run()
}
