package main

import (
	"github.com/romana/rlog"
	"time"
)

var (
	ScriptsGitRepo *GitRepo
	ScriptsUpdated chan ScriptsUpdate

	// TODO: хранить в ConfigMap в кластере
	currentCommit string
)

type ScriptsUpdate struct {
	Path   string
	Commit string
}

func FetchScripts() {
	err := ScriptsGitRepo.Fetch()
	if err != nil {
		rlog.Errorf("REPOFETCH: %s", err.Error())
		return
	}

	newCommit, err := ScriptsGitRepo.GetHead()
	if err != nil {
		rlog.Errorf("REPOGETHEAD: %s", err.Error())
		return
	}

	if newCommit != currentCommit {
		rlog.Debugf("REPOFETCH currentCommit='%s' newCommit='%s'", currentCommit, newCommit)

		currentCommit = newCommit

		var repoPath string
		if repoPath, err = ScriptsGitRepo.CreateClone(currentCommit); err != nil {
			rlog.Errorf("REPOCLONE: %s", err.Error())
			return
		}

		ScriptsUpdated <- ScriptsUpdate{repoPath, currentCommit}
	}
}

func InitScriptsManager() {
	ScriptsUpdated = make(chan ScriptsUpdate)
	currentCommit = ""
}

func RunScriptsManager() {
	ticker := time.NewTicker(time.Duration(10) * time.Second)

	for {
		select {
		case repo := <-RepoUpdated:
			subticker := time.NewTicker(time.Duration(10) * time.Second)

			branch := repo["branch"]
			if branch == "" {
				branch = "master"
			}

		loop:
			for {
				select {
				case <-subticker.C:
					clonedRepo, err := GetOrCreateGitBareRepo(repo["url"], branch)
					if err != nil {
						rlog.Errorf("REPOCLONE `%s` (`%s`): %s", repo["url"], branch, err.Error())
					} else {
						ScriptsGitRepo = clonedRepo
						break loop
					}
				}
			}
			subticker.Stop()

			FetchScripts()
		case <-ticker.C:
			if ScriptsGitRepo != nil {
				FetchScripts()
			}
		}
	}
}
