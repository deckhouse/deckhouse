package main

import (
	"bytes"
	"github.com/gosimple/slug"
	"io/ioutil"
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
		if err := CloneBare(gitRepo.Url, gitRepo.Path); err != nil {
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

func (r *GitRepo) CreateClone(commit string) (string, error) {
	tmpDir, err := ioutil.TempDir("/tmp/antiopa", slug.Make(r.Url))
	if err != nil {
		return "", err
	}

	if err = Clone(r.Path, tmpDir); err != nil {
		return "", err
	}

	if err = Checkout(tmpDir, commit); err != nil {
		return "", err
	}

	return tmpDir, nil
}

func (r *GitRepo) Fetch() error {
	cmd := exec.Command("git", "-C", r.Path, "fetch")
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

func Clone(url string, path string) error {
	cmd := exec.Command("git", "clone", url, path)
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func CloneBare(url string, path string) error {
	cmd := exec.Command("git", "clone", "--bare", url, path)
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func Checkout(gitDir string, commit string) error {
	cmd := exec.Command("git", "-C", gitDir, "checkout", commit)
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
