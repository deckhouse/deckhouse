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

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

const DefaultSSHAgentPrivateKeys = "~/.ssh/id_rsa"

type PrivateKeyFileToPassphrase map[string][]byte

var (
	SSHPrivateKeys = make([]string, 0)

	ConnectionConfigPath = ""
	SSHAgentPrivateKeys  = make([]string, 0)
	SSHBastionHost       = ""
	SSHBastionPort       = ""
	SSHBastionUser       = os.Getenv("USER")
	SSHBastionPass       = ""
	SSHUser              = os.Getenv("USER")
	SSHHosts             = make([]session.Host, 0)
	sshHostsRaw          = make([]string, 0)
	SSHPort              = ""
	SSHExtraArgs         = ""

	AskBecomePass = false
	BecomePass    = ""

	AskBastionPass = false

	SSHLegacyMode = false
	SSHModernMode = false

	// todo ugly solution need refact
	PrivateKeysToPassPhrasesFromConfig = make(PrivateKeyFileToPassphrase)
)

type connectionConfigParser interface {
	ParseConnectionConfigFromFile() error
}

func DefineSSHFlags(cmd *kingpin.CmdClause, parser connectionConfigParser) {
	var sshFlagSetByUser, sshUserFlagSetByUser, sshBastionUserFlagSetByUser bool

	cmd.Flag("ssh-agent-private-keys", "Paths to private keys. Those keys will be used to connect to servers and to the bastion. Can be specified multiple times (default: '~/.ssh/id_rsa')").
		IsSetByUser(&sshFlagSetByUser).
		Envar(configEnvName("SSH_AGENT_PRIVATE_KEYS")).
		StringsVar(&SSHAgentPrivateKeys)
	cmd.Flag("ssh-bastion-host", "Jumper (bastion) host to connect to servers (will be used both by infrastructure creation utility and ansible). Only IPs or hostnames are supported, name from ssh-config will not work.").
		IsSetByUser(&sshFlagSetByUser).
		Envar(configEnvName("SSH_BASTION_HOST")).
		StringVar(&SSHBastionHost)
	cmd.Flag("ssh-bastion-port", "SSH destination port").
		Default("22").
		IsSetByUser(&sshFlagSetByUser).
		Envar(configEnvName("SSH_BASTION_PORT")).
		StringVar(&SSHBastionPort)
	cmd.Flag("ssh-bastion-user", "User to authenticate under when connecting to bastion (default: $USER)").
		IsSetByUser(&sshBastionUserFlagSetByUser).
		Default(SSHBastionUser).
		Envar(configEnvName("SSH_BASTION_USER")).
		StringVar(&SSHBastionUser)
	cmd.Flag("ssh-user", "User to authenticate under (default: $USER)").
		IsSetByUser(&sshUserFlagSetByUser).
		Envar(configEnvName("SSH_USER")).
		Default(SSHUser).
		StringVar(&SSHUser)
	cmd.Flag("ssh-host", "SSH destination hosts, can be specified multiple times").
		IsSetByUser(&sshFlagSetByUser).
		Envar(configEnvName("SSH_HOSTS")).
		StringsVar(&sshHostsRaw)
	cmd.Flag("ssh-port", "SSH destination port").
		Default("22").
		IsSetByUser(&sshFlagSetByUser).
		Envar(configEnvName("SSH_PORT")).
		StringVar(&SSHPort)
	cmd.Flag("ssh-extra-args", "extra args for ssh commands (-vvv)").
		IsSetByUser(&sshFlagSetByUser).
		Envar(configEnvName("SSH_EXTRA_ARGS")).
		StringVar(&SSHExtraArgs)
	cmd.Flag("connection-config", "SSH connection config file path").
		Envar(configEnvName("CONNECTION_CONFIG")).
		StringVar(&ConnectionConfigPath)
	cmd.Flag("ssh-legacy-mode", "Force legacy SSH mode").
		Envar(configEnvName("SSH_LEGACY_MODE")).
		BoolVar(&SSHLegacyMode)
	cmd.Flag("ssh-modern-mode", "Force modern SSH mode").
		Envar(configEnvName("SSH_MODERN_MODE")).
		BoolVar(&SSHModernMode)
	cmd.Flag("ask-bastion-pass", "Ask for bastion password before the installation process.").
		Envar(configEnvName("ASK_BASTION_PASS")).
		BoolVar(&AskBastionPass)

	cmd.PreAction(func(c *kingpin.ParseContext) error {
		if !sshBastionUserFlagSetByUser && sshUserFlagSetByUser {
			SSHBastionUser = SSHUser
			sshFlagSetByUser = true
		}
		return nil
	})
	cmd.PreAction(func(c *kingpin.ParseContext) error {
		if len(sshHostsRaw) > 0 {
			for i, host := range sshHostsRaw {
				SSHHosts = append(SSHHosts, session.Host{Host: host, Name: strconv.Itoa(i)})
			}
		}
		return nil
	})

	cmd.PreAction(func(c *kingpin.ParseContext) (err error) {
		if len(ConnectionConfigPath) == 0 {
			return nil
		}
		return processConnectionConfigFile(sshFlagSetByUser, parser)
	})

	cmd.PreAction(func(c *kingpin.ParseContext) (err error) {
		if len(SSHPrivateKeys) != 0 {
			return nil
		}
		return processConnectionConfigFlags()
	})
	cmd.PreAction(func(c *kingpin.ParseContext) (err error) {
		if SSHLegacyMode && (AskBecomePass && len(SSHPrivateKeys) == 0) {
			return fmt.Errorf("SSH legacy mode does not support password-based SSH authentication. If you are using `--ask-become-pass`, please either specify `--ssh-modern-mode`, or leave the SSH mode unset to allow automatic detection of the appropriate method.")
		}
		return nil
	})
}

func ParseSSHPrivateKeyPaths(pathSets []string) ([]string, error) {
	res := make([]string, 0)
	if len(pathSets) == 0 || (len(pathSets) == 1 && pathSets[0] == "") {
		return res, nil
	}

	for _, pathSet := range pathSets {
		keys := strings.Split(pathSet, ",")
		for _, k := range keys {
			if strings.HasPrefix(k, "~") {
				home := os.Getenv("HOME")
				if home == "" {
					return nil, fmt.Errorf("HOME is not defined for key '%s'", k)
				}
				k = strings.Replace(k, "~", home, 1)
			}

			keyPath, err := filepath.Abs(k)
			if err != nil {
				return nil, fmt.Errorf("get absolute path for '%s': %v", k, err)
			}

			_, err = os.Stat(keyPath)
			if err != nil {
				if pathSet == DefaultSSHAgentPrivateKeys {
					continue
				}
				return nil, fmt.Errorf("cannot stat file %s", keyPath)
			}
			res = append(res, keyPath)
		}
	}
	return res, nil
}

func DefineBecomeFlags(cmd *kingpin.CmdClause) {
	// Ansible compatible
	cmd.Flag("ask-become-pass", "Ask for sudo password before the installation process.").
		Envar(configEnvName("ASK_BECOME_PASS")).
		Short('K').
		BoolVar(&AskBecomePass)
}

func processConnectionConfigFile(sshFlagSetByUser bool, parser connectionConfigParser) error {
	if sshFlagSetByUser {
		return fmt.Errorf("'connection-config' cannot be specified with other ssh flags at the same time")
	}

	return parser.ParseConnectionConfigFromFile()
}

func processConnectionConfigFlags() error {
	if len(SSHAgentPrivateKeys) == 0 {
		SSHAgentPrivateKeys = append(SSHAgentPrivateKeys, DefaultSSHAgentPrivateKeys)
	}

	var err error
	SSHPrivateKeys, err = ParseSSHPrivateKeyPaths(SSHAgentPrivateKeys)
	if err != nil {
		return fmt.Errorf("ssh private keys: %w", err)
	}
	return nil
}
