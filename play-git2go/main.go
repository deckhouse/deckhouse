package main

// libgit2 версия 24 - в ubuntu 16.04, поэтому берём её.

// Сделать git clone из локального репо в другой локальный реп

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	git "gopkg.in/libgit2/git2go.v24"
)

var HttpUserPasswdRegex = regexp.MustCompile(`https?:\/\/((([^:@]*):?([^@]*))@)?[^@].*`)

// Прототип интерфейс для работы с гитом.
// Возможные действия при работе антиопы с гитом:
// Склонировать или открыть главную копию репозитория скриптов
// Склонировать главную копию во временную для запуска скриптов
// Обновить главную копию
type AntiopaScriptsActions interface {
	CloneMainToTmpRepo(mainDir string, tmpdir string)
	OpenOrCloneMainRepo(url string, ref string, dir string) (*git.Repository, error)
	FetchMainRepo(url string, ref string, dir string)
	GetHeadCommit(repoDir string)
}

var repo_url = "https://oauth2:Sf5zFGUrzXm5vraq7xgp@github.com/deckhouse/deckhouse-scripts"
//var token = "Sf5zFGUrzXm5vraq7xgp"
var branch = "test-go-gits"
var bare_dir = "antiopa"
var copy_dir = "antiopa-copy"

func main() {
	defer TimeTrack(time.Now(), "main")
	fmt.Println("start clone Main")

	//repo, err := OpenOrCloneMainRepo(repo_url, branch, bare_dir)
	// клонирование из директории с основным репом в директорию для запуска скриптов
	repo, err := OpenOrCloneMainRepo("antiopa-local", "test-go-gits", "antiopa-local-2")
	if err != nil {
		fmt.Println("Error open or close main repo: %v", err)
		os.Exit(1)
	}

	// Обновление основного репо
	FetchMainRepo(repo, "")
	// Процесс построения workdir в libgit2 - отдельный от обновления
	CheckoutRef(repo, "test-go-gits")
}

// открыть или склонировать основной репозиторий
func OpenOrCloneMainRepo(repoUrl string, ref string, dstDir string) (*git.Repository, error) {
	defer TimeTrack(time.Now(), "OpenOrCloneMainRepo")
	if IsExist(dstDir) {
		fmt.Printf("Directory %s already exist\n", dstDir)

		repo, err := git.OpenRepository(dstDir)
		if err != nil {
			//return nil, err
			fmt.Printf("copy err open %v\n", err)
			os.Exit(1)
		}

		// TODO check if current ref == ref
		// if not - checkout ref

		return repo, nil
	}

	repo, err := git.Clone(repoUrl, dstDir, &git.CloneOptions{
		FetchOptions: &git.FetchOptions{
			RemoteCallbacks: git.RemoteCallbacks{
				CredentialsCallback: credentialsCallback,
			},
		},
	})

	if err != nil {
		//return nil, err
		fmt.Printf("Clone %s err %v\n", repoUrl, err)
		os.Exit(1)
	}
	fmt.Println("Successfully cloned")
	return repo, nil
}

// в url Приходит то, что передаётся в PlainClone в ключе URL
// Здесь парсится url на предмет username и password.
func credentialsCallback(url string, username string, allowedTypes git.CredType) (git.ErrorCode, *git.Cred) {
	fmt.Printf("allowed types: %+v\n", allowedTypes)
	fmt.Printf("url %s username %s\n", url, username)
	if strings.HasPrefix(url, "http") {
		matches := HttpUserPasswdRegex.FindStringSubmatch(url)
		if matches != nil {
			ret, cred := git.NewCredUserpassPlaintext(matches[3], matches[4])
			return git.ErrorCode(ret), &cred
		}
	} else {
		fmt.Println("cannot determine credentials for url %s")
	}
	return git.ErrUser, nil
}

// Пример для ключа ssh
// func credentialsCallback(url string, username string, allowedTypes git.CredType) (git.ErrorCode, *git.Cred) {
// 	ret, cred := git.NewCredSshKey("git", "/home/vagrant/.ssh/id_rsa.pub", "/home/vagrant/.ssh/id_rsa", "")
// 	return git.ErrorCode(ret), &cred
// }

// Made this one just return 0 during troubleshooting...
func certificateCheckCallback(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
	return 0
}

// Кандидат в пакет utils
// IsExist returns true if file exists
func IsExist(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// Построение workdir из ветки - то, что делает
// git checkout -b test origin/test
// В процессе достаётся коммит - его можно возвратить для хранения и сравнения с предыдущим
func CheckoutRef(repo *git.Repository, ref string) {
	branch, err := repo.LookupBranch(ref, git.BranchRemote)
	if err != nil {
		fmt.Printf("copy err LookupBranch %v\n", err)
		os.Exit(1)
	}

	treeObj, err := branch.Peel(git.ObjectTree)
	if err != nil {
		fmt.Printf("copy err Peel %v\n", err)
		os.Exit(1)
	}

	tree, err := treeObj.AsTree()
	if err != nil {
		fmt.Printf("copy err AsTree %v\n", err)
		os.Exit(1)
	}

	commit, err := branch.Peel(git.ObjectCommit)
	if err != nil {
		fmt.Printf("copy err peel commit %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("branch %s has commit %s and tree %s\n", ref, commit.Id(), tree.Id())

	err = repo.CheckoutTree(tree, &git.CheckoutOpts{
		Strategy: git.CheckoutForce,
	})
	if err != nil {
		fmt.Printf("copy err checkouttree %v\n", err)
		os.Exit(1)
	}

	refname := fmt.Sprintf("refs/heads/%s", ref)
	newRef, err := repo.References.Create(refname, commit.Id(), true, "")
	if err != nil {
		fmt.Printf("copy err reference create %v\n", err)
		os.Exit(1)
	}

	err = repo.SetHead(newRef.Name())
	if err != nil {
		fmt.Printf("copy err SetHead %v\n", err)
		os.Exit(1)
	}

}

// git fetch
func FetchMainRepo(repo *git.Repository, ref string) {
	remote, err := repo.Remotes.Lookup("origin")
	if (err != nil) {
		fmt.Printf("fetch err remote lookup %v", err)
		return
	}

	err = remote.Fetch(nil, nil, "")
	if (err != nil) {
		fmt.Printf("fetch err fetch %v", err)
		return
	}

	fmt.Println("FetchMainRepo ended")
}


// Предыдущий неудачный эксперимент - клон основного bare-repo, клон из bare-repo во временный.
// Клон из bare во временный репо прошёл, но сделать checkout не получилось.

// CloneToBare clones repo to bare_dir
func CloneToBare() *git.Repository {
	defer TimeTrack(time.Now(), "CloneToBareRepo")
	if IsExist(bare_dir) {
		fmt.Printf("Directory %s already exist\n", bare_dir)

		repo, err := git.OpenRepository(bare_dir)
		if err != nil {
			fmt.Printf("copy err open %v\n", err)
			os.Exit(1)
		}
		return repo
	}

	//repo_url := fmt.Sprintf("https://oauth2:%s@github.com/deckhouse/deckhouse-scripts", token)

	repo, err := git.Clone(repo_url, bare_dir, &git.CloneOptions{
		FetchOptions: &git.FetchOptions{
			RemoteCallbacks: git.RemoteCallbacks{
				CredentialsCallback: credentialsCallback,
				//CertificateCheckCallback: certificateCheckCallback,
			},
		},
		CheckoutBranch: branch,
		//Bare: true,
	})

	if err != nil {
		fmt.Printf("Clone %s err %v\n", repo_url, err)
		os.Exit(1)
	}
	fmt.Println("Successfully cloned")
	return repo
}



// CloneBareToLocalWorkdir clones from bare_dir to copy_dir using branch
func CloneBareToLocalWorkdir() {
	defer TimeTrack(time.Now(), "CloneBareToLocalWorkdir")
	_, err := git.Clone(bare_dir, copy_dir, &git.CloneOptions{
		CheckoutOpts: &git.CheckoutOpts{
			Strategy: git.CheckoutForce,
		},
		//		CheckoutBranch: "origin/test-go-gits",
		Bare: false,
	})
	if err != nil {
		fmt.Printf("copy err %v\n", err)
		os.Exit(1)
	}

	// rev, err := repo.RevparseSingle(branch)
	// if err != nil {
	// 	fmt.Printf("copy err revparsesingle %v\n", err)
	// 	os.Exit(1)
	// }

	// tree, err := rev.AsTree()
	// if err != nil {
	// 	fmt.Printf("copy err as tree %v\n", err)
	// 	os.Exit(1)
	// }

	// err = repo.CheckoutTree(tree, nil)
	// if err != nil {
	// 	fmt.Printf("copy err checkouttree %v\n", err)
	// 	os.Exit(1)
	// }

	//err = repo.SetHead(tree.AsCommit().ref.Name())
	//if err != nil {
	//	fmt.Printf("copy err sethead %v\n", err)
	//	os.Exit(1)
	//}

	//err = repo.CheckoutHead(nil)
	//if err != nil {
	//	fmt.Printf("copy err checkout %v\n", err)
	//	os.Exit(1)
	//}

	fmt.Println("Successfully copied")
}

func CheckoutToAnotherWorkdir() {
	repo, err := git.OpenRepository(bare_dir)
	if err != nil {
		fmt.Printf("copy err open %v\n", err)
		os.Exit(1)
	}

	branch, err := repo.LookupBranch("origin/test-go-gits", git.BranchRemote)
	if err != nil {
		fmt.Printf("copy err LookupBranch %v\n", err)
		os.Exit(1)
	}

	treeObj, err := branch.Peel(git.ObjectTree)
	if err != nil {
		fmt.Printf("copy err Peel %v\n", err)
		os.Exit(1)
	}

	tree, err := treeObj.AsTree()
	if err != nil {
		fmt.Printf("copy err AsTree %v\n", err)
		os.Exit(1)
	}

	commit, err := branch.Peel(git.ObjectCommit)
	if err != nil {
		fmt.Printf("copy err pell commit %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("commit %+v %s\n", commit, commit.Id())

	err = repo.CheckoutTree(tree, &git.CheckoutOpts{
		TargetDirectory: copy_dir,
		Strategy:        git.CheckoutForce,
	})
	if err != nil {
		fmt.Printf("copy err checkouttree %v\n", err)
		os.Exit(1)
	}

}

func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	fmt.Printf("%s started at %02d:%02d:%02d took %.2fs\n", name, start.Hour(), start.Minute(), start.Second(), elapsed.Seconds())
}
