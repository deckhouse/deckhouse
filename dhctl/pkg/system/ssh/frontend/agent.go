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
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/cmd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
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
	err := addKeys(a.AgentSettings.AuthSock, a.AgentSettings.PrivateKeys)
	if err != nil {
		return fmt.Errorf("add keys: %w", err)
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

func addKeys(authSock string, keys []session.AgentPrivateKey) error {
	conn, err := net.Dial("unix", authSock)
	if err != nil {
		return fmt.Errorf("error dialing with ssh agent %s: %w", authSock, err)
	}
	defer conn.Close()

	agentClient := agent.NewClient(conn)

	for _, key := range keys {
		privateKey, err := parsePrivateSSHKey(key.Key, []byte(key.Passphrase))
		if err != nil {
			return err
		}

		err = agentClient.Add(agent.AddedKey{PrivateKey: privateKey})
		if err != nil {
			return fmt.Errorf("adding ssh key with ssh agent %s: %w", authSock, err)
		}
	}

	return nil
}

func parsePrivateSSHKey(keyPath string, passphrase []byte) (any, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("error reading key file %q: %w", keyPath, err)
	}

	var privateKey interface{}

	privateKey, err = ssh.ParseRawPrivateKey(keyData)
	if err != nil {
		var passphraseMissingError *ssh.PassphraseMissingError
		switch {
		case errors.As(err, &passphraseMissingError):
			if len(passphrase) == 0 {
				passphraseFromStdin, err := terminal.AskPassword(
					fmt.Sprintf("Enter passphrase for ssh key %q: ", keyPath),
				)
				if err != nil {
					return nil, fmt.Errorf("getting passphrase for ssh key %q: %w", keyPath, err)
				}
				passphrase = passphraseFromStdin
			}
			privateKey, err = ssh.ParseRawPrivateKeyWithPassphrase(keyData, passphrase)
			if err != nil {
				return nil, fmt.Errorf("parsing private key %q: %w", keyPath, err)
			}
		default:
			return nil, fmt.Errorf("parsing private key %q: %w", keyPath, err)
		}
	}

	return privateKey, nil
}
