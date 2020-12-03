package frontend

import (
	"fmt"
	"os"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/system/ssh/cmd"
	"flant/candictl/pkg/system/ssh/session"
)

type Agent struct {
	Session *session.Session

	Agent *cmd.SSHAgent
}

func NewAgent(sess *session.Session) *Agent {
	return &Agent{Session: sess}
}

func (a *Agent) Start() error {
	if len(a.Session.PrivateKeys) == 0 {
		a.Agent = &cmd.SSHAgent{
			Session:  a.Session,
			AuthSock: os.Getenv("SSH_AUTH_SOCK"),
		}
		return nil
	}

	a.Agent = &cmd.SSHAgent{
		Session: a.Session,
	}

	log.DebugLn("agent: start ssh-agent")
	err := a.Agent.Start()
	if err != nil {
		return fmt.Errorf("start ssh-agent: %v", err)
	}

	log.DebugLn("agent: run ssh-add for keys")
	err = a.AddKeys()
	if err != nil {
		return fmt.Errorf("add keys: %v", err)
	}

	return nil
}

// TODO replace with x/crypto/ssh/agent ?
func (a *Agent) AddKeys() error {
	for _, k := range a.Session.PrivateKeys {
		log.DebugF("add key %s\n", k)
		sshAdd := cmd.NewSSHAdd(a.Session).KeyCmd(k)
		output, err := sshAdd.CombinedOutput()
		if err != nil {
			werr := "signal: interrupt"
			if err.Error() == werr {
				return fmt.Errorf("process stopped")
			}
			return fmt.Errorf("ssh-add: %s %v", string(output), err)
		}

		str := string(output)
		if str != "" && str != "\n" {
			log.InfoF("ssh-add: %s\n", output)
		}
	}

	if app.IsDebug {
		log.DebugLn("list added keys")
		listCmd := cmd.NewSSHAdd(a.Session).ListCmd()

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
