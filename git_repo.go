package main

import (
	"bytes"
	"github.com/gosimple/slug"
	"os"
	"os/exec"
	"path"
	"strings"
)

type GitRepo struct {
	Url    string
	Branch string
	Path   string
}

func GetOrCreateGitBareRepo(url string, branch string) (*GitRepo, error) {
	gitRepo := &GitRepo{url, branch, path.Join("/tmp/antiopa", slug.Make(url), ".git")}
	if !gitRepo.IsExist() {
		if err := gitRepo.CloneBare(); err != nil {
			return nil, err
		}
	}
	return gitRepo, nil
}

func (r *GitRepo) IsExist() bool {
	if _, err := os.Stat(r.Path); os.IsNotExist(err) {
		return false
	}
	return true
}

func (r *GitRepo) CloneBare() error {
	cmd := exec.Command("git", "clone", "--bare", r.Url, r.Path)
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (r *GitRepo) Fetch() error {
	cmd := exec.Command("git", "-C", r.Path, "fetch")
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (r *GitRepo) GetHead() (string, error) {
	cmd := exec.Command("git", "-C", r.Path, "show-ref", "-s", path.Join("refs/heads", r.Branch))
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()

	if err != nil {
		return "", err
	}

	ref := strings.TrimSpace(out.String())
	return ref, nil
}
