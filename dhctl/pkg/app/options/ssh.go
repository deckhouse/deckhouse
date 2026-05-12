// Copyright 2026 Flant JSC
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

package options

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	otattribute "go.opentelemetry.io/otel/attribute"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

// DefaultSSHAgentPrivateKeys is the default value used when --ssh-agent-private-keys
// is not provided.
const DefaultSSHAgentPrivateKeys = "~/.ssh/id_rsa"

// PrivateKeyFileToPassphrase maps an SSH private key file path to its passphrase.
type PrivateKeyFileToPassphrase = map[string]string

// SSHOptions groups all SSH-related runtime configuration.
type SSHOptions struct {
	PrivateKeys []string

	ConnectionConfigPath string
	AgentPrivateKeys     []string
	BastionHost          string
	BastionPort          string
	BastionUser          string
	BastionPass          string
	User                 string
	Hosts                []session.Host
	HostsRaw             []string
	Port                 string
	ExtraArgs            string

	AskBastionPass bool

	LegacyMode bool
	ModernMode bool

	// PrivateKeysToPassPhrasesFromConfig is populated by the connection-config parser.
	PrivateKeysToPassPhrasesFromConfig PrivateKeyFileToPassphrase
}

// NewSSHOptions returns SSHOptions with defaults derived from $USER.
func NewSSHOptions() SSHOptions {
	user := os.Getenv("USER")
	return SSHOptions{
		PrivateKeys:                        make([]string, 0),
		AgentPrivateKeys:                   make([]string, 0),
		Hosts:                              make([]session.Host, 0),
		HostsRaw:                           make([]string, 0),
		User:                               user,
		BastionUser:                        user,
		PrivateKeysToPassPhrasesFromConfig: make(PrivateKeyFileToPassphrase),
	}
}

// ProcessConnectionConfigFlags fills PrivateKeys from AgentPrivateKeys, applying
// the default agent key when no key is configured. It used to live in pkg/app
// as the unexported processConnectionConfigFlags.
func (o *SSHOptions) ProcessConnectionConfigFlags() error {
	if len(o.AgentPrivateKeys) == 0 {
		o.AgentPrivateKeys = append(o.AgentPrivateKeys, DefaultSSHAgentPrivateKeys)
	}
	keys, err := ParseSSHPrivateKeyPaths(o.AgentPrivateKeys)
	if err != nil {
		return fmt.Errorf("ssh private keys: %w", err)
	}
	o.PrivateKeys = keys
	return nil
}

func (o *SSHOptions) ToSpanAttributes() []otattribute.KeyValue {
	return []otattribute.KeyValue{
		otattribute.Int("ssh.privateKeysCount", len(o.PrivateKeys)),
		otattribute.Int("ssh.agentPrivateKeysCount", len(o.AgentPrivateKeys)),
		otattribute.StringSlice("ssh.hosts", o.HostsRaw),
		otattribute.String("ssh.port", o.Port),
		otattribute.String("ssh.user", o.User),
		otattribute.String("ssh.bastionHost", o.BastionHost),
		otattribute.String("ssh.bastionPort", o.BastionPort),
		otattribute.String("ssh.bastionUser", o.BastionUser),
		otattribute.String("ssh.extraArgs", o.ExtraArgs),
		otattribute.Bool("ssh.askBastionPass", o.AskBastionPass),
		otattribute.Bool("ssh.legacyMode", o.LegacyMode),
		otattribute.Bool("ssh.modernMode", o.ModernMode),
	}
}

// ParseSSHPrivateKeyPaths expands tilde-prefixed paths, makes them absolute and
// verifies their existence.
//
// The default agent key path is silently skipped if the file does not exist
// (preserving the previous pkg/app behavior).
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

			if _, err = os.Stat(keyPath); err != nil {
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

// BecomeOptions holds the sudo/become password prompt configuration.
// BecomePass is filled in at runtime by the interactive password prompt.
type BecomeOptions struct {
	AskBecomePass bool
	BecomePass    string
}

func (o *BecomeOptions) ToSpanAttributes() []otattribute.KeyValue {
	return []otattribute.KeyValue{
		otattribute.Bool("become.askBecomePass", o.AskBecomePass),
		otattribute.Bool("become.becomePassIsSet", o.BecomePass != ""),
	}
}
