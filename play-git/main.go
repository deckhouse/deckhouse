package main

// Сделать git clone из локального репо в другой локальный реп

import (
	"fmt"
	"os"
	"os/exec"
)

var token = "Sf5zFGUrzXm5vraq7xgp"
var branch = "test-go-gits"
var bare_dir = "antiopa-git"
var copy_dir = "antiops-git-copy"

func main() {
	fmt.Println("start clone bare")
	CloneToBareRepo()
	fmt.Println("start clone to workdir")
	CloneBareToLocalWorkdir()
}

func CloneToBareRepo() {
	cmd := "/usr/bin/git"
	args := []string{"clone", "--bare", fmt.Sprintf("https://oauth2:%s@github.com/deckhouse/deckhouse-scripts", token), bare_dir}
	if err := exec.Command(cmd, args...).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Successfully cloned")
}

func CloneBareToLocalWorkdir() {
	cmd := "/usr/bin/git"
	args := []string{"clone", fmt.Sprintf("--branch=%s", branch), bare_dir, copy_dir}
	if err := exec.Command(cmd, args...).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Successfully copied")
}
