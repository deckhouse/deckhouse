package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"flant/candictl/pkg/log"
	"flant/candictl/pkg/system/process"
	"flant/candictl/pkg/system/ssh/session"
	"flant/candictl/pkg/util/tomb"
)

const SSHAgentPath = "ssh-agent"

type SSHAgent struct {
	*process.Executor

	Session *session.Session

	agentCmd *exec.Cmd

	Pid      string
	AuthSock string
}

var SSHAgentAuthSockRe = regexp.MustCompile(`SSH_AUTH_SOCK=(.*?);`)

// Start runs ssh-agent as a subprocess, gets SSH_AUTH_SOCK path and
func (a *SSHAgent) Start() error {
	a.agentCmd = exec.Command(SSHAgentPath, "-D")
	a.agentCmd.Env = os.Environ()
	a.agentCmd.Dir = "/"
	a.agentCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	a.Executor = process.NewDefaultExecutor(a.agentCmd)
	// a.EnableLive()
	a.WithStdoutHandler(func(l string) {
		log.DebugF("ssh agent: got '%s'\n", l)

		m := SSHAgentAuthSockRe.FindStringSubmatch(l)
		if len(m) == 2 && m[1] != "" {
			a.AuthSock = m[1]
		}
	})

	a.WithWaitHandler(func(err error) {
		if err != nil {
			log.ErrorF("SSH-agent process exited, now stop. Wait error: %v\n", err)
			return
		}
		log.InfoF("SSH-agent process exited, now stop.\n")
	})

	err := a.Executor.Start()
	if err != nil {
		a.agentCmd = nil
		return fmt.Errorf("start ssh-agent subprocess: %v", err)
	}

	// wait for ssh agent pid
	success := false
	maxWait := 1000
	retries := 0
	t := time.NewTicker(5 * time.Millisecond)
	for {
		<-t.C
		if a.AuthSock != "" {
			log.DebugF("ssh-agent: SSH_AUTH_SOCK=%s\n", a.AuthSock)
			success = true
			break
		}
		retries++
		if retries > maxWait {
			break
		}
	}
	t.Stop()

	if !success {
		a.Stop()
		return fmt.Errorf("cannot get pid and auth sock path for ssh-agent")
	}

	// save auth sock in session to access it from other cmds and frontends
	a.Session.AuthSock = a.AuthSock
	tomb.RegisterOnShutdown(func() {
		_ = os.RemoveAll(filepath.Dir(a.AuthSock))
	})
	return nil
}
