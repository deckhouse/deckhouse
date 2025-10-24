// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssh

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func tryToExtractPassPhraseFromConfig(path string) string {
	if path == "" {
		return ""
	}

	l := len(app.PrivateKeysToPassPhrasesFromConfig)
	log.DebugF("Passphrases map has %d passphrases\n", l)

	if l == 0 {
		return ""
	}

	p, ok := app.PrivateKeysToPassPhrasesFromConfig[path]
	if !ok || len(p) == 0 {
		return ""
	}

	log.DebugF("Passphrase for key %s found in map!\n", path)

	return p
}

func tryToExtractPassPhraseFromConfigOrTerminal(path string) (string, error) {
	p := tryToExtractPassPhraseFromConfig(path)
	if len(p) > 0 {
		return p, nil
	}

	log.DebugF("Passphrase for key %s not found in map. Try to get from terminal\n", path)

	enteredPassword, err := terminal.AskPassword(
		fmt.Sprintf("Enter passphrase for ssh key %q: ", path),
	)
	if err != nil {
		return "", fmt.Errorf("Getting passphrase for ssh key %q get error: %w", path, err)
	}

	if len(enteredPassword) == 0 {
		return "", fmt.Errorf("Passphrase for ssh key %q is empty", path)
	}

	return string(enteredPassword), nil
}

func CollectDHCTLPrivateKeysFromFlags() []session.AgentPrivateKey {
	keys := make([]session.AgentPrivateKey, 0, len(app.SSHPrivateKeys))
	for _, key := range app.SSHPrivateKeys {
		keys = append(keys, session.AgentPrivateKey{Key: key})
	}

	return keys
}

func GetSSHPrivateKey(keyPath string, passphrase string) (any, error) {
	log.DebugF("Parsing private ssh key %s\n", keyPath)

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("Reading key file %q got error: %w", keyPath, err)
	}

	keyData = append(bytes.TrimSpace(keyData), '\n')

	sshKey, err := ssh.ParseRawPrivateKey(keyData)
	if err != nil {
		var passphraseMissingError *ssh.PassphraseMissingError
		switch {
		case errors.As(err, &passphraseMissingError):
			var err error
			if passphrase, err = tryToExtractPassPhraseFromConfigOrTerminal(keyPath); err != nil {
				return nil, err
			}
			sshKey, err = ssh.ParseRawPrivateKeyWithPassphrase(keyData, []byte(passphrase))
			if err != nil {
				return nil, fmt.Errorf("Wrong passphrase for ssh key")
			}
		default:
			return nil, fmt.Errorf("Parsing private key %q got error: %w", keyPath, err)
		}
	}

	return sshKey, nil
}
