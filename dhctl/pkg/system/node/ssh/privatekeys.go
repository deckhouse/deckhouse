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

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func ParsePrivateSSHKey(keyPath string, passphrase []byte) (any, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("error reading key file %q: %w", keyPath, err)
	}

	keyData = append(bytes.TrimSpace(keyData), '\n')

	var privateKey interface{}

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
		return nil, fmt.Errorf("error reading key file %q: %w", keyPath, err)
	}

	keyData = append(bytes.TrimSpace(keyData), '\n')

	var passphrase []byte
	_, err = ssh.ParseRawPrivateKey(keyData)
	if err != nil {
		var passphraseMissingError *ssh.PassphraseMissingError
		switch {
		case errors.As(err, &passphraseMissingError):
			var err error
			passphrase, err = terminal.AskPassword(
				fmt.Sprintf("Enter passphrase for ssh key %q: ", keyPath),
			)
			if err != nil {
				return nil, fmt.Errorf("getting passphrase for ssh key %q: %w", keyPath, err)
			}
			_, err = ssh.ParseRawPrivateKeyWithPassphrase(keyData, passphrase)
			if err != nil {
				return nil, fmt.Errorf("wrong passphrase for ssh key")
			}
		default:
			return nil, fmt.Errorf("parsing private key %q: %w", keyPath, err)
		}
	}

	return &session.AgentPrivateKey{Key: keyPath, Passphrase: string(passphrase)}, nil
}
