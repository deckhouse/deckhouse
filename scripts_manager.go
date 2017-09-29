package main

import (
	"time"
)

var (
	ScriptsCommitted chan string
)

func FetchScripts(Repo map[string]string) {
	// todo: git clone или fetch + смотрим изменение коммита, шлем сигнал в ScriptsUpdated
}

func InitScriptsManager() {
}

// Запускается в отдельной goroutine
func RunScriptsManager() {
	for {
		select {
		case repo := <-RepoUpdated:
			FetchScripts(repo)
		}
		// Ловим RepoUpdated -> запускаем FetchScripts
		// Периодически запускаем FetchScripts
		time.Sleep(time.Duration(1) * time.Second)
	}
}
