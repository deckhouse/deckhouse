package main

import (
	"github.com/romana/rlog"
	"io/ioutil"
	"os"
	"path"
	"time"
)

var (
	NotClonedRepo  map[string]string
	ScriptsGitRepo *GitRepo
	ScriptsUpdated chan ScriptsUpdate

	// TODO: хранить в ConfigMap в кластере
)

type ScriptsUpdate struct {
	Path   string
	Commit string
}

func InitScriptsManager() {
	ScriptsUpdated = make(chan ScriptsUpdate)
	NotClonedRepo = map[string]string{}
}

func RunScriptsManager() {
	ticker := time.NewTicker(time.Duration(60) * time.Second)

	for {
		select {
		case repo := <-RepoUpdated:
			NotClonedRepo = repo
			cloneAndFetchRepo()
		case <-ticker.C:
			cloneAndFetchRepo()
		}
	}
}

func cloneAndFetchRepo() {
	if len(NotClonedRepo) != 0 {
		cloneRepo()
	}

	if ScriptsGitRepo != nil {
		fetchScripts()
	}
}

func cloneRepo() {
	branch := NotClonedRepo["branch"]
	if branch == "" {
		branch = "master"
	}

	clonedGitRepo, err := OpenOrCloneMainRepo(NotClonedRepo["url"], branch)
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
		rlog.Errorf("Unable to fetch scripts: %s", err.Error())
		return
	}

	newCommit, err := ScriptsGitRepo.GetHeadRef()
	if err != nil {
		rlog.Errorf("Unable to get head: %s", err.Error())
		return
	}

	currentCommit, err := getCurrentCommit()
	if err != nil {
		rlog.Errorf("Getting current commit failed: %s", err.Error())
		return
	}

	if newCommit != currentCommit {
		rlog.Debugf("REPOCHANGE currentCommit='%s' newCommit='%s'", currentCommit, newCommit)

		if err = setCurrentCommit(newCommit); err != nil {
			rlog.Errorf("Setting current commit failed: %s", err.Error())
			return
		}

		var clonedRepoPath string
		if clonedRepoPath, err = ScriptsGitRepo.Clone(); err != nil {
			rlog.Errorf("Unable to prepare scripts run tree: %s", err.Error())
			return
		}

		ScriptsUpdated <- ScriptsUpdate{clonedRepoPath, currentCommit}
	}
}

func getCurrentCommit() (commit string, err error) {
	ccfp := currentCommitFilePath()
	if IsExist(ccfp) {
		var bytes []byte
		bytes, err = ioutil.ReadFile(ccfp)
		if err != nil {
			return
		}

		return string(bytes), nil
	}

	return
}

func setCurrentCommit(commit string) error {
	ccfp := currentCommitFilePath()

	if !IsExist(path.Dir(ccfp)) {
		os.Mkdir(path.Dir(ccfp), 0755)
	}

	return ioutil.WriteFile(ccfp, []byte(commit), 0644)
}

func currentCommitFilePath() string {
	return path.Join(path.Join(RunDir, "scripts-commit"), ScriptsGitRepo.Hash)
}
