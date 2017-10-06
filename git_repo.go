package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
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
	hasher := md5.New()
	hasher.Write([]byte(url))
	hasher.Write([]byte(branch))
	path := path.Join(path.Join(RunDir, "scripts-repo"), hex.EncodeToString(hasher.Sum(nil)))

	gitRepo := &GitRepo{url, branch, path}
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
	tmpDir, err := ioutil.TempDir("", "antiopa-scripts-run-tree-")
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
	cmd := exec.Command("git", "-C", r.Path, "fetch", "--progress", "origin", fmt.Sprintf("%s:%s", r.Branch, r.Branch))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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
		return "", fmt.Errorf("bad branch %s", r.Branch)
	}

	ref := strings.TrimSpace(out.String())
	return ref, nil
}

func Clone(url string, path string) error {
	cmd := exec.Command("git", "clone", url, path)
	cmd.Env = append(cmd.Env, []string{"GIT_ASKPASS=", "GIT_TERMINAL_PROMPT=0"}...)

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func CloneBare(url string, path string) error {
	cmd := exec.Command("git", "clone", "--bare", url, path)
	cmd.Env = append(cmd.Env, []string{"GIT_ASKPASS=", "GIT_TERMINAL_PROMPT=0"}...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

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
