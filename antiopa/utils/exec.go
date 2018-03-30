package utils

import (
	"os"
	"os/exec"
)

func MakeCommand(dir string, entrypoint string, args []string, envs []string) *exec.Cmd {
	cmd := exec.Command(entrypoint, args...)
	cmd.Env = append(cmd.Env, envs...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
