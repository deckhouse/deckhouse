package main

import (
	"time"

	"github.com/romana/rlog"
)

/*
RepoUpdated
ModulesUpdated
ScriptsUpdated

RepoUpdated -> FetchScripts
ModulesUpdated -> RunScripts
ScriptsUpdated -> RunScripts
*/

func Init() {
	rlog.Info("Init")

	InitConfigManager()
}

func Run() {
	rlog.Info("Run")

	go RunConfigManager()

	for {
	    // Получаем RepoUpdated => запускаем FetchScripts(cfg)
	    // Получаем ScriptsUpdated => запускаем скрипты
		time.Sleep(time.Duration(1) * time.Second)
	}
}

func main() {
	Init()
	Run()
}
