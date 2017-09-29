package main

import (
	"time"

	"github.com/romana/rlog"
)

var (
    lastModules []map[string]string,
    lastScriptsCommit string,
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

	InitConfigManager()
	InitScriptsManager()
}

func RunScripts(Modules []map[string]string, Commit string) {
  // Делает checkout во временную директорию по указанному Commit
  // Запускает скрипты без mutex'ов во временной директории. Впоследствии можно делать diff по OldCommit и NewCommit и запускать только изменившиеся модули
  // Удаляет временную директорию
}

func Run() {
	rlog.Info("Run")

    // Общее правило: запускаем всех "менеджеров" в отдельные goroutine
	go RunConfigManager()
	go RunScriptsManager()

    // В главной goroutine оркестрируем получение новых данных и запускаем сами скрипты
	for {
	    // Получаем RepoUpdated => запускаем FetchScripts(cfg)
	    // Получаем ScriptsUpdated => запускаем скрипты если
		time.Sleep(time.Duration(1) * time.Second)
	}
}

func main() {
	Init()
	Run()
}
