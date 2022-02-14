// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package frontend

import (
	"fmt"
	"os"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/cmd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
)

type Agent struct {
	AgentSettings *session.AgentSettings

	Agent *cmd.SSHAgent
}

func NewAgent(sess *session.AgentSettings) *Agent {
	return &Agent{AgentSettings: sess}
}

func (a *Agent) Start() error {
	if len(a.AgentSettings.PrivateKeys) == 0 {
		a.Agent = &cmd.SSHAgent{
			AgentSettings: a.AgentSettings,
			AuthSock:      os.Getenv("SSH_AUTH_SOCK"),
		}
		return nil
	}

	a.Agent = &cmd.SSHAgent{
		AgentSettings: a.AgentSettings,
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
	for _, k := range a.AgentSettings.PrivateKeys {
		log.DebugF("add key %s\n", k)
		sshAdd := cmd.NewSSHAdd(a.AgentSettings).KeyCmd(k)
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
		listCmd := cmd.NewSSHAdd(a.AgentSettings).ListCmd()

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
