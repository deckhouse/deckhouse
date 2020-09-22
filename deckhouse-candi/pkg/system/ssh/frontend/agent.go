package frontend

import (
	"fmt"
	"os"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/system/ssh/cmd"
	"flant/deckhouse-candi/pkg/system/ssh/session"
)

type Agent struct {
	Session *session.Session

	Agent *cmd.SshAgent
}

func NewAgent(sess *session.Session) *Agent {
	return &Agent{Session: sess}
}

func (a *Agent) Start() error {
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

	app.Debugf("agent: start ssh-agent\n")
	err := a.Agent.Start()
	if err != nil {
		return fmt.Errorf("start ssh-agent: %v", err)
	}

	app.Debugf("agent: run ssh-add for keys\n")
	err = a.AddKeys()
	if err != nil {
		return fmt.Errorf("add keys: %v", err)
	}

	return nil
}

// TODO replace with x/crypto/ssh/agent ?
func (a *Agent) AddKeys() error {
	for _, k := range a.Session.PrivateKeys {
		app.Debugf("add key %s\n", k)
		sshAdd := cmd.NewSshAdd(a.Session).KeyCmd(k)
		output, err := sshAdd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("ssh-add: %s %v", string(output), err)
		}

		str := string(output)
		if str != "" && str != "\n" {
			log.InfoF("ssh-add: %s\n", output)
		}
	}

	if app.IsDebug {
		app.Debugf("list added keys\n")
		listCmd := cmd.NewSshAdd(a.Session).ListCmd()

		output, err := listCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("ssh-add -l: %v", err)
		}

		str := string(output)
		if str != "" && str != "\n" {
			log.InfoF("ssh-add -l: %s\n", output)
		}
	}

	return nil
}

func (a *Agent) Stop() {
	a.Agent.Stop()
}
