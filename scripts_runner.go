package main

import (
	"github.com/romana/rlog"
)

func RunScripts(Modules []map[string]string, Commit string) {
	// Делает checkout во временную директорию по указанному Commit
	// Запускает скрипты без mutex'ов во временной директории. Впоследствии можно делать diff по OldCommit и NewCommit и запускать только изменившиеся модули
	// Удаляет временную директорию

	rlog.Infof("RunScripts for commit '%s' from modules %v", Commit, Modules)
}
