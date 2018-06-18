package executor

import (
	"os/exec"
	"strings"
	"sync"

	"github.com/romana/rlog"
)

var ExecutorLock = &sync.Mutex{}

func Run(cmd *exec.Cmd) error {
	ExecutorLock.Lock()
	defer ExecutorLock.Unlock()

	rlog.Debugf("Executing command in '%s': '%s'", cmd.Dir, strings.Join(cmd.Args, " "))
	return cmd.Run()
}

func Output(cmd *exec.Cmd) (output []byte, err error) {
	ExecutorLock.Lock()
	defer ExecutorLock.Unlock()

	output, err = cmd.Output()
	return
}
