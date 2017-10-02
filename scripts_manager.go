package main

import (
	"github.com/romana/rlog"
	"time"
)

var (
	ScriptsGitRepo   *GitRepo
	ScriptsCommitted chan string

	// TODO: хранить в ConfigMap в кластере
	currentCommit string
)

func FetchScripts() {
	err := ScriptsGitRepo.FetchCurrentBranch()
	if err != nil {
		rlog.Errorf("REPOFETCH: %s", err.Error())
		return
	}

	newCommit, err := ScriptsGitRepo.GetHeadRef()
	if err != nil {
		rlog.Errorf("REPOGETHEAD", err.Error())
		return
	}

	if newCommit != currentCommit {
		rlog.Debugf("REPOFETCH %v currentCommit='%s' newCommit='%s'", currentCommit, newCommit)

		currentCommit = newCommit

		ScriptsCommitted <- newCommit
	}
}

func InitScriptsManager() {
	ScriptsCommitted = make(chan string)
	currentCommit = ""
}

func RunScriptsManager() {
	ticker := time.NewTicker(time.Duration(10) * time.Second)

	var err error
	for {
		select {
		case repo := <-RepoUpdated:
			subticker := time.NewTicker(time.Duration(10) * time.Second)

			branch := repo["branch"]
			if branch == "" {
				branch = "master"
			}

			for {
				select {
				case <-subticker.C:
					ScriptsGitRepo, err = GitRepoCloneMemory(repo["url"], repo["branch"])
					if err != nil {
						rlog.Errorf("REPOCLONE `%s` (`%s`): %s", repo["url"], repo["branch"], err.Error())
					} else {
						break
					}
				}
			}
			FetchScripts()
		case <-ticker.C:
			if ScriptsGitRepo != nil {
				FetchScripts()
			}
		}
	}
}
