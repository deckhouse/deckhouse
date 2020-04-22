package frontend

import (
	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/ssh/cmd"
	"flant/deckhouse-candi/pkg/ssh/session"
	"fmt"
	"os"
)

type Agent struct {
	Session *session.Session

	Agent *cmd.SshAgent
}

func NewAgent(sess *session.Session) *Agent {
	return &Agent{Session: sess}
}

func (a *Agent) Start() error {
	success := false
	defer func() {
		if !success {
			a.Stop()
		}
	}()
	if len(a.Session.PrivateKeys) == 0 {
		a.Agent = &cmd.SshAgent{
			Session:  a.Session,
			AuthSock: os.Getenv("SSH_AUTH_SOCK"),
		}
		return nil
	}

	a.Agent = &cmd.SshAgent{
		Session: a.Session,
	}

	err := a.Agent.Start()
	if err != nil {
		return fmt.Errorf("start ssh-agent: %v", err)
	}

	err = a.AddKeys()
	if err != nil {
		return fmt.Errorf("add keys: %v", err)
	}

	a.Session.RegisterStoppable(a.Agent)
	success = true
	return nil

}

// TODO replace with x/crypto/ssh/agent ?
func (a *Agent) AddKeys() error {
	for _, k := range a.Session.PrivateKeys {
		app.Debugf("add key %s\n", k)
		sshAdd := cmd.NewSshAdd(a.Session).KeyCmd(k)
		output, err := sshAdd.Output()

		if err != nil {
			return fmt.Errorf("ssh-add: %v", err)
		}
		str := string(output)
		if str != "" && str != "\n" {
			fmt.Printf("ssh-add: %s\n", output)
		}
	}

	if app.IsDebug == 1 {
		app.Debugf("list added keys\n")
		listCmd := cmd.NewSshAdd(a.Session).ListCmd()
		output, err := listCmd.CombinedOutput()

		if err != nil {
			return fmt.Errorf("ssh-add -l: %v", err)
		}
		str := string(output)
		if str != "" && str != "\n" {
			fmt.Printf("ssh-add -l: %s\n", output)
		}
	}

	return nil
}

func (a *Agent) Stop() {
	if a.Agent != nil {
		a.Agent.Stop()
		a.Agent = nil
	}
}
