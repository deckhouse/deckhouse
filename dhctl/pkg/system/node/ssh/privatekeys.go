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
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func tryToExtractPassPhraseFromConfig(path string) []byte {
	if path == "" {
		return nil
	}

	if len(app.PrivateKeysToPassPhrasesFromConfig) == 0 {
		return nil
	}

	p, ok := app.PrivateKeysToPassPhrasesFromConfig[path]
	if !ok {
		return nil
	}

	if len(p) == 0 {
		return nil
	}

	return p
}

func tryToExtractPassPhraseFromConfigOrTerminal(path string) ([]byte, error) {
	p := tryToExtractPassPhraseFromConfig(path)
	if len(p) > 0 {
		return p, nil
	}

	p, err := terminal.AskPassword(
		fmt.Sprintf("Enter passphrase for ssh key %q: ", path),
	)
	if err != nil {
		return nil, fmt.Errorf("Getting passphrase for ssh key %q get error: %w", path, err)
	}

	if len(p) == 0 {
		return nil, fmt.Errorf("Passphrase for ssh key %q is empty", path)
	}

	return p, nil
}

func ParsePrivateSSHKey(keyPath string, passphrase []byte) (any, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("error reading key file %q: %w", keyPath, err)
	}

	keyData = append(bytes.TrimSpace(keyData), '\n')

	var privateKey interface{}

	if len(passphrase) == 0 {
		passphrase = tryToExtractPassPhraseFromConfig(keyPath)
	}

	if len(passphrase) == 0 {
		privateKey, err = ssh.ParseRawPrivateKey(keyData)
	} else {
		privateKey, err = ssh.ParseRawPrivateKeyWithPassphrase(keyData, passphrase)
	}

	return privateKey, nil
}

func GetPrivateKeys(keyPath string) (*session.AgentPrivateKey, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("Reading key file %q got error: %w", keyPath, err)
	}

	keyData = append(bytes.TrimSpace(keyData), '\n')

	var passphrase []byte
	_, err = ssh.ParseRawPrivateKey(keyData)
	if err != nil {
		var passphraseMissingError *ssh.PassphraseMissingError
		switch {
		case errors.As(err, &passphraseMissingError):
			var err error
			if passphrase, err = tryToExtractPassPhraseFromConfigOrTerminal(keyPath); err != nil {
				return nil, err
			}
			_, err = ssh.ParseRawPrivateKeyWithPassphrase(keyData, passphrase)
			if err != nil {
				return nil, fmt.Errorf("Wrong passphrase for ssh key")
			}
		default:
			return nil, fmt.Errorf("Parsing private key %q got error: %w", keyPath, err)
		}
	}

	return &session.AgentPrivateKey{Key: keyPath, Passphrase: string(passphrase)}, nil
}
