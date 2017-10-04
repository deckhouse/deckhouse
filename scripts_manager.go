package main

import (
	"github.com/romana/rlog"
	"time"
)

var (
	NotClonedRepo  map[string]string
	ScriptsGitRepo *GitRepo
	ScriptsUpdated chan ScriptsUpdate

	// TODO: хранить в ConfigMap в кластере
	currentCommit string
)

type ScriptsUpdate struct {
	Path   string
	Commit string
}

func InitScriptsManager() {
	ScriptsUpdated = make(chan ScriptsUpdate)
	NotClonedRepo = map[string]string{}
	currentCommit = ""
}

func RunScriptsManager() {
	ticker := time.NewTicker(time.Duration(10) * time.Second)

	for {
		select {
		case repo := <-RepoUpdated:
			NotClonedRepo = repo
		case <-ticker.C:
			if len(NotClonedRepo) != 0 {
				cloneRepo()
			}

			if ScriptsGitRepo != nil {
				fetchScripts()
			}
		}
	}
}

func cloneRepo() {
	branch := NotClonedRepo["branch"]
	if branch == "" {
		branch = "master"
	}

	clonedGitRepo, err := GetOrCreateGitBareRepo(NotClonedRepo["url"], branch)
	if err != nil {
		rlog.Errorf("REPOCLONE `%s` (`%s`): %s", NotClonedRepo["url"], branch, err.Error())
	} else {
		ScriptsGitRepo = clonedGitRepo
		NotClonedRepo = map[string]string{}
	}
}

func fetchScripts() {
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
