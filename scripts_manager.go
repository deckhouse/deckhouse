package main

import (
	"time"
)

var (
	ScriptsCommitted chan string
	lastRepo         map[string]string
)

func FetchScripts(Repo map[string]string) {
	// todo: git clone или fetch + смотрим изменение коммита, шлем сигнал в ScriptsCommitted
	// посылаем новый коммит только если он поменялся с последнего раза
	ScriptsCommitted <- "997c972a602521e68f10f018344c9aed6734792b"
}

func InitScriptsManager() {
	lastRepo = make(map[string]string)
}

func RunScriptsManager() {
	timer := time.NewTimer(time.Duration(10) * time.Second)

	for {
		select {
		case repo := <-RepoUpdated:
			FetchScripts(repo)
			lastRepo = repo
		case <-timer.C:
			if len(lastRepo) != 0 {
				FetchScripts(lastRepo)
			}
		}
	}
}
