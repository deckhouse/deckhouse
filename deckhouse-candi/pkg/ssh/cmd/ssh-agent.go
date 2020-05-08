package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"time"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/process"
	"flant/deckhouse-candi/pkg/ssh/session"
)

var SshAgentPath = "ssh-agent"

type SshAgent struct {
	*process.Executor

	Session *session.Session

	agentCmd *exec.Cmd

	Pid      string
	AuthSock string
}

var SshAgentAuthSockRe = regexp.MustCompile(`SSH_AUTH_SOCK=(.*?);`)

func NewSshAgent(sess *session.Session) *SshAgent {
	return &SshAgent{Session: sess}
}

// Start runs ssh-agent as a subprocess, gets SSH_AUTH_SOCK path and
func (a *SshAgent) Start() error {
	a.agentCmd = exec.Command(SshAgentPath, "-D")
	a.agentCmd.Env = os.Environ()
	a.agentCmd.Dir = "/"

	a.Executor = process.NewDefaultExecutor(a.agentCmd)
	a.EnableLive()
	a.WithStdoutHandler(func(l string) {
		app.Debugf("ssh agent: got '%s'\n", l)
		m := SshAgentAuthSockRe.FindStringSubmatch(l)
		if len(m) == 2 && m[1] != "" {
			a.AuthSock = m[1]
		}
	})

	a.WithWaitHandler(func(err error) {
		if err != nil {
			fmt.Printf("Ssh-agent process exited, now stop. Wait error: %v", err)
		} else {
			fmt.Printf("Ssh-agent process exited, now stop.")
		}
		go func() {
			process.DefaultSession.Stop()
			os.Exit(12)
		}()
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
			app.Debugf("ssh-agent: SSH_AUTH_SOCK=%s\n", a.AuthSock)
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
	return nil
}
