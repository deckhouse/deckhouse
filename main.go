package main

import (
	"github.com/romana/rlog"
)

var (
	lastModulesInitialized bool
	lastModules            []map[string]string

	lastScriptsCommitInitialized bool
	lastScriptsCommit            string
)

func Init() {
	rlog.Info("Init")

	lastModulesInitialized = false
	lastScriptsCommitInitialized = false

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
			if lastScriptsCommitInitialized {
				// TODO: Заметить разницу между modules и запустить только новые скрипты
				RunScripts(modules, lastScriptsCommit)
			}

			lastModules = modules
			lastModulesInitialized = true
		case commit := <-ScriptsCommitted:
			if lastModulesInitialized {
				// TODO: Заметить разницу между modules и запустить только новые скрипты
				RunScripts(lastModules, commit)
			}

			lastScriptsCommit = commit
			lastScriptsCommitInitialized = true
		}
	}
}

func main() {
	Init()
	Run()
}
