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

func RunScripts(Modules []map[string]string, Commit string) {
  // todo: делаем checkout во временную директорию по указанному Commit
  // Запускает скрипты без mutex'ов во временной директории. Впоследствии можно делать diff по OldCommit и NewCommit и запускать только изменившиеся модули
  // Удаляет временную директорию
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
