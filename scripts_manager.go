package main

import (
	"os/exec"
	"strings"
	"time"

	"github.com/romana/rlog"
)

var (
	ScriptsCommitted chan string

	lastRepoInitialized bool
	lastRepo            map[string]string

	// TODO: хранить в ConfigMap в кластере
	currentCommit string
)

func FetchScripts(repo map[string]string) {
	// todo: git clone или fetch + смотрим изменение коммита, шлем сигнал в ScriptsCommitted
	// посылаем новый коммит только если он поменялся с последнего раза

	out, err := exec.Command("uuidgen").Output()
	if err != nil {
		return
	}

	newCommit := strings.TrimSpace(string(out))

	rlog.Debugf("REPOFETCH %v currentCommit='%s' newCommit='%s'", repo, currentCommit, newCommit)

	currentCommit = newCommit

	ScriptsCommitted <- newCommit
}

func InitScriptsManager() {
	ScriptsCommitted = make(chan string)
	lastRepoInitialized = false
	currentCommit = ""
}

func RunScriptsManager() {
	ticker := time.NewTicker(time.Duration(10) * time.Second)

	for {
		select {
		case repo := <-RepoUpdated:
			FetchScripts(repo)

			lastRepo = repo
			lastRepoInitialized = true
		case <-ticker.C:
			if lastRepoInitialized {
				FetchScripts(lastRepo)
			}
		}
	}
}
