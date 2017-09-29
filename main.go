package main

import (
	"github.com/romana/rlog"
)

var (
	lastModules       []map[string]string
	lastScriptsCommit string
)

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
			rlog.Debugf("ModulesUpdated %v", modules)
			if lastScriptsCommit != "" {
				// TODO: Заметить разницу между modules и запустить только новые скрипты
				RunScripts(modules, lastScriptsCommit)
			}
		case commit := <-ScriptsCommitted:
			rlog.Debugf("ScriptsCommitted %s", commit)
			if len(lastModules) != 0 {
				// TODO: Заметить разницу между modules и запустить только новые скрипты
				RunScripts(lastModules, commit)
			}
		}
	}
}

func main() {
	Init()
	Run()
}
