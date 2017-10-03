package main

import (
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"io/ioutil"
	"strings"
)

type GitRepo struct {
	*git.Repository
	ClonedBranch string
}

func GitRepoClone(url string, remoteBranch string) (*GitRepo, error) {
	tmpDir, err := ioutil.TempDir("/tmp", "antiopa")
	if err != nil {
		return nil, err
	}

	r, err := git.PlainClone(tmpDir, false, &git.CloneOptions{
		URL:           url,
		ReferenceName: branchReference(remoteBranch),
	})

	return &GitRepo{r, remoteBranch}, err
}

func (r *GitRepo) FetchCurrentBranch() error {
	return r.Fetch(&git.FetchOptions{RemoteName: r.ClonedBranch})
}

func (r *GitRepo) GetHeadRef() (string, error) {
	ref, err := r.Head()
	if err != nil {
		return "", err
	}

	commit, err := r.CommitObject(ref.Hash())
	if err != nil {
		return "", err
	}

	return commit.Hash.String(), nil
}

func branchReference(ref string) plumbing.ReferenceName {
	if !strings.HasPrefix(ref, "refs/heads/") {
		ref = strings.Join([]string{"refs/heads/", ref}, "")
	}
	return plumbing.ReferenceName(ref)
}
