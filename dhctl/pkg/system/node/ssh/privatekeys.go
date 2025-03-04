package ssh

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func ParsePrivateSSHKey(keyPath string, passphrase []byte) (any, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("error reading key file %q: %w", keyPath, err)
	}

	keyData = append(bytes.TrimSpace(keyData), '\n')

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
