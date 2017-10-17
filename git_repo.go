package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/romana/rlog"
	git "gopkg.in/libgit2/git2go.v24"
)

var HttpUserPasswdRegex = regexp.MustCompile(`https?:\/\/((([^:@]*):?([^@]*))@)?[^@].*`)
var RunDir = "FAKE"

type GitRepo struct {
	Ref  string
	Path string
	Hash string
	*git.Repository
}

func (r *GitRepo) Fetch() (err error) {
	remote, err := r.Remotes.Lookup("origin")
	if err != nil {
		rlog.Debugf("GITREPO fetch err remote lookup `%v`", err)
		return
	}

	err = remote.Fetch(nil, &git.FetchOptions{
		RemoteCallbacks: git.RemoteCallbacks{
			CredentialsCallback: credentialsCallback,
		},
	}, "")
	if err != nil {
		rlog.Debugf("GITREPO fetch err fetch `%v`", err)
		return
	}

	rlog.Debug("GITREPO fetch ended")
	return nil
}

func (r *GitRepo) GetHeadRef() (string, error) {
	branch, err := r.LookupBranch(fmt.Sprintf("origin/%s", r.Ref), git.BranchRemote)
	if err != nil {
		rlog.Debugf("GITREPO copy err LookupBranch `%v`", err)
		return "", err
	}

	commit, err := branch.Peel(git.ObjectCommit)
	if err != nil {
		rlog.Debugf("GITREPO copy err peel commit `%v`", err)
		return "", err
	}

	return commit.Id().String(), nil
}

func (r *GitRepo) Clone() (clonedRepoPath string, err error) {
	if err = r.CheckoutRef(); err != nil {
		return "", err
	}

	clonedRepoPath, err = ioutil.TempDir("", "antiopa-scripts-run-tree-")
	if err != nil {
		return "", err
	}
	_, err = CloneRepo(r.Path, clonedRepoPath)

	return clonedRepoPath, err
}

// Построение workdir из ветки - то, что делает
// git checkout -b test origin/test
// В процессе достаётся коммит - его можно возвратить для хранения и сравнения с предыдущим
func (r *GitRepo) CheckoutRef() error {
	branch, err := r.LookupBranch(fmt.Sprintf("origin/%s", r.Ref), git.BranchRemote)
	if err != nil {
		rlog.Debugf("GITREPO copy err LookupBranch `%v`", err)
		return err
	}

	treeObj, err := branch.Peel(git.ObjectTree)
	if err != nil {
		rlog.Debugf("GITREPO copy err Peel `%v`", err)
		return err
	}

	tree, err := treeObj.AsTree()
	if err != nil {
		rlog.Debugf("GITREPO copy err AsTree `%v`", err)
		return err
	}

	commit, err := branch.Peel(git.ObjectCommit)
	if err != nil {
		rlog.Debugf("GITREPO copy err peel commit `%v`", err)
		return err
	}
	rlog.Debugf("GITREPO branch %s has commit %s and tree %s", r.Ref, commit.Id(), tree.Id())

	err = r.CheckoutTree(tree, &git.CheckoutOpts{
		Strategy: git.CheckoutForce,
	})
	if err != nil {
		rlog.Debugf("GITREPO copy err checkout tree `%v`", err)
		return err
	}

	refname := fmt.Sprintf("refs/heads/%s", r.Ref)
	newRef, err := r.References.Create(refname, commit.Id(), true, "")
	if err != nil {
		rlog.Debugf("GITREPO copy err reference create `%v`", err)
		return err
	}

	err = r.SetHead(newRef.Name())
	if err != nil {
		rlog.Debugf("GITREPO copy err SetHead `%v`", err)
		return err
	}

	return nil
}

func OpenOrCloneMainRepo(url string, branch string) (*GitRepo, error) {
	hasher := md5.New()
	hasher.Write([]byte(url))
	hasher.Write([]byte(branch))
	bareRepoPath := path.Join(path.Join(RunDir, "scripts-repo"), hex.EncodeToString(hasher.Sum(nil)))

	repo, err := OpenOrCloneRepo(url, bareRepoPath)
	return &GitRepo{branch, bareRepoPath, hex.EncodeToString(hasher.Sum(nil)), repo}, err
}

func OpenOrCloneRepo(url string, dstDir string) (repo *git.Repository, err error) {
	if IsExist(dstDir) {
		rlog.Debugf("GITREPO directory `%s` already exist", dstDir)

		repo, err = git.OpenRepository(dstDir)
		if err != nil {
			rlog.Debugf("GITREPO copy err open `%v`", err)
			return nil, err
		}
	} else {
		repo, err = CloneRepo(url, dstDir)
	}

	return
}

func CloneRepo(url string, dstDir string) (repo *git.Repository, err error) {
	repo, err = git.Clone(url, dstDir, &git.CloneOptions{
		FetchOptions: &git.FetchOptions{
			RemoteCallbacks: git.RemoteCallbacks{
				CredentialsCallback: credentialsCallback,
			},
		},
	})
	if err != nil {
		rlog.Debugf("GITREPO copy err clone `%v`", err)
		return nil, err
	}

	rlog.Debug("GITREPO Successfully cloned")

	return
}

// Кандидат в пакет utils
// IsExist returns true if file exists
func IsExist(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// в url Приходит то, что передаётся в PlainClone в ключе URL
// Здесь парсится url на предмет username и password.
func credentialsCallback(url string, username string, allowedTypes git.CredType) (git.ErrorCode, *git.Cred) {
	rlog.Debugf("GITREPO allowed types: `%+v`", allowedTypes)
	rlog.Debugf("GITREPO url `%s` username `%s`", url, username)
	if strings.HasPrefix(url, "http") {
		matches := HttpUserPasswdRegex.FindStringSubmatch(url)
		if matches != nil {
			ret, cred := git.NewCredUserpassPlaintext(matches[3], matches[4])
			return git.ErrorCode(ret), &cred
		}
	} else {
		rlog.Debugf("GITREPO cannot determine credentials for url `%s`", url)
	}
	return git.ErrUser, nil
}
