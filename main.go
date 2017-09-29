package main

import (
	"time"

	"github.com/romana/rlog"
)

var (
	lastModules       []map[string]string
	lastScriptsCommit string
)

/*
RepoUpdated
ModulesUpdated
ScriptsUpdated

RepoUpdated -> FetchScripts
ModulesUpdated -> RunScripts(newModules, lastScriptsCommit), lastModules = newModules
ScriptsUpdated -> RunScripts(lastModules, newCommit), lastScriptsCommit = newCommit
*/

func Init() {
	rlog.Info("Init")

	lastModules = make([]map[string]string, 0)
	lastScriptsCommit = ""

	InitConfigManager()
	InitScriptsManager()
}

func Run() {
	rlog.Info("Run")

	go RunConfigManager()
	go RunScriptsManager()

	for {
		select {
		case modules := <-ModulesUpdated:
			if lastScriptsCommit != "" {
				// TODO: Заметить разницу между modules и запустить только новые скрипты
				RunScripts(modules, lastScriptsCommit)
			}
		case commit := <-ScriptsCommitted:
			if len(lastModules) != 0 {
				// TODO: Заметить разницу между modules и запустить только новые скрипты
				RunScripts(lastModules, commit)
			}
		}
		time.Sleep(time.Duration(1) * time.Second)
	}
}

func main() {
	Init()
	Run()
}
