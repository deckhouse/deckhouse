package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"time"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/ssh/session"
)

var SshAgentPath = "ssh-agent"

type SshAgent struct {
	Session *session.Session

	agentCmd *exec.Cmd

	Pid      string
	AuthSock string
}

func NewSshAgent(sess *session.Session) *SshAgent {
	return &SshAgent{Session: sess}
}

func (a *SshAgent) Start() error {
	stdoutReadPipe, stdoutWritePipe, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("unable to create os pipe for stdout: %s", err)
	}

	a.agentCmd = exec.Command(SshAgentPath, "-D")
	a.agentCmd.Env = os.Environ()
	a.agentCmd.Dir = "/"
	a.agentCmd.Stdout = stdoutWritePipe

	err = a.agentCmd.Start()
	if err != nil {
		a.agentCmd = nil
		return fmt.Errorf("start ssh-agent subprocess: %v", err)
	}

	go func() {
		//defer wg.Done()
		a.ConsumeSshAgentLines(stdoutReadPipe)
	}()

	// TODO create a channel to distinguish between agent error and planned stop
	go func() {
		//defer wg.Done()
		err = a.agentCmd.Wait()
		if err != nil {
			fmt.Sprintf("Ssh-agent process exited, now stop. Wait error: %v", err)
		} else {
			fmt.Sprintf("Ssh-agent process exited, now stop.")
		}
		os.Exit(1)
	}()

	// wait for ssh agent pid
	success := false
	maxWait := 1000
	retries := 0
	t := time.NewTicker(5 * time.Millisecond)
	for {
		<-t.C
		//app.Debugf("retry %d\n", retries)
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
		return fmt.Errorf("cannot get pid and auth sock path for ssh-agent")
	}

	// save auth sock in session to access it from other cmds and frontends
	a.Session.AuthSock = a.AuthSock
	return nil

}

func (a *SshAgent) Stop() {
	if a == nil {
		return
	}
	if a.agentCmd != nil {
		a.agentCmd.Process.Kill()
	}
}

func (a *SshAgent) ConsumeSshAgentLines(r io.Reader) {
	authSockRe := regexp.MustCompile(`SSH_AUTH_SOCK=(.*?);`)
	//pidRe := regexp.MustCompile(`Agent\ pid\ (\d+);`)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		// try to find auth sock
		m := authSockRe.FindStringSubmatch(text)
		if len(m) == 2 && m[1] != "" {
			a.AuthSock = m[1]
		}

		if app.IsDebug == 1 && text != "" {
			fmt.Printf("ssh-agent: %s\n", text)
		}
	}
}
