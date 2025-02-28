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
	"net"
	"os"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"golang.org/x/crypto/ssh"
)

type SSHClientConfig struct {
	SSHCllient *ssh.Client

	SSHConn       *ssh.Conn
	NetConn       *net.Conn
	BastionClient *ssh.Client

	SudoPassword string
}

func NewSSHClientFromFlags() (*SSHClientConfig, error) {
	var bastionClient *ssh.Client
	var client *ssh.Client
	if len(app.SSHBastionHost) > 0 {
		bastionConfig := &ssh.ClientConfig{}
		if len(app.SSHAgentPrivateKeys) > 0 {
			signers := make([]ssh.Signer, 0, len(app.SSHPrivateKeys))
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

			bastionConfig = &ssh.ClientConfig{
				User:            app.SSHBastionUser,
				Auth:            AuthMethods,
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}
			bastionAddr := fmt.Sprintf("%s:%s", app.SSHBastionHost, app.SSHBastionPort)
			var err error
			bastionClient, err = ssh.Dial("tcp", bastionAddr, bastionConfig)
			if err != nil {
				return nil, fmt.Errorf("could not connect to bastion host")
			}
		} else {
			return nil, fmt.Errorf("no SSH key present to connect to bastion host")
		}
	}

	config := &ssh.ClientConfig{}
	if len(app.SSHAgentPrivateKeys) > 0 {
		signers := make([]ssh.Signer, 0, len(app.SSHPrivateKeys))
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

	var targetConn net.Conn
	var ClientConn ssh.Conn
	addr := fmt.Sprintf("%s:%s", app.SSHHosts, app.SSHPort)
	if bastionClient == nil {
		var err error
		client, err = ssh.Dial("tcp", addr, config)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to host: %w", err)
		}
	} else {
		var err error
		targetConn, err = bastionClient.Dial("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to target host through bastion host: %w", err)
		}
		targetClientConn, targetNewChan, targetReqChan, err := ssh.NewClientConn(targetConn, addr, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create client connection to target host: %w", err)
		}
		ClientConn = ClientConn
		client = ssh.NewClient(targetClientConn, targetNewChan, targetReqChan)
	}

	return &SSHClientConfig{
		SSHCllient:    client,
		BastionClient: bastionClient,
		NetConn:       &targetConn,
		SSHConn:       &ClientConn,
	}, nil
}
