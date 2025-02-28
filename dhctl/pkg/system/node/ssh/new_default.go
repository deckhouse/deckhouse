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

package ssh

import (
	"fmt"
	"os"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"golang.org/x/crypto/ssh"
)

func NewSSHClientFromFlags() (*ssh.Client, error) {
	// TODO bastion logic
	config := &ssh.ClientConfig{}
	if len(app.SSHAgentPrivateKeys) > 0 {
		var signers []ssh.Signer
		for _, keypath := range app.SSHAgentPrivateKeys {
			key, err := os.ReadFile(keypath)
			if err != nil {
				return nil, fmt.Errorf("unable to read private key: %v", err)
			}

			// Create the Signer for this private key.
			signer, err := ssh.ParsePrivateKey(key)
			if err != nil {
				return nil, fmt.Errorf("unable to parse private key: %v", err)
			}
			signers = append(signers, signer)
		}

		AuthMethods := []ssh.AuthMethod{ssh.PublicKeys(signers...)}

		config = &ssh.ClientConfig{
			User:            app.SSHUser,
			Auth:            AuthMethods,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	} else if len(app.BecomePass) > 0 {
		config = &ssh.ClientConfig{
			User: app.SSHUser,
			Auth: []ssh.AuthMethod{
				ssh.Password(app.BecomePass),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	} else {
		return nil, fmt.Errorf("no authentication config for SSH found")
	}

	addr := fmt.Sprintf("%s:%s", app.SSHHosts, app.SSHPort)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to host: %w", err)
	}

	return client, nil
}
