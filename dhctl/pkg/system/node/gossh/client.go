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
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

func NewClient(session *session.Session, privKeys []session.AgentPrivateKey) *Client {
	return &Client{
		Settings:    session,
		privateKeys: privKeys,
	}
}

type Client struct {
	sshClient *ssh.Client

	Settings *session.Session

	privateKeys []session.AgentPrivateKey

	SSHConn       *ssh.Conn
	NetConn       *net.Conn
	BastionClient *ssh.Client
}

func (s *Client) Start() error {
	if s.Settings == nil {
		return fmt.Errorf("possible bug in ssh client: session should be created before start")
	}

	log.DebugLn("Starting go ssh client....")

	signers := make([]ssh.Signer, 0, len(s.privateKeys))
	for _, keypath := range s.privateKeys {
		key, err := node.ParsePrivateSSHKey(keypath.Key, []byte(keypath.Passphrase))
		if err != nil {
			return err
		}
		signer, err := ssh.NewSignerFromKey(key)
		if err != nil {
			return fmt.Errorf("unable to parse private key: %v", err)
		}
		signers = append(signers, signer)
	}

	var bastionClient *ssh.Client
	var client *ssh.Client
	if s.Settings.BastionHost != "" {
		bastionConfig := &ssh.ClientConfig{}
		log.DebugLn("Initialize bastion connection...")

		if len(s.privateKeys) == 0 {
			return fmt.Errorf("no SSH key present to connect to bastion host")
		}

		AuthMethods := []ssh.AuthMethod{ssh.PublicKeys(signers...)}

		bastionConfig = &ssh.ClientConfig{
			User:            s.Settings.BastionUser,
			Auth:            AuthMethods,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		bastionAddr := fmt.Sprintf("%s:%s", s.Settings.BastionHost, s.Settings.BastionPort)
		var err error
		log.DebugF("Connect to bastion host %s\n", bastionAddr)
		bastionClient, err = ssh.Dial("tcp", bastionAddr, bastionConfig)
		if err != nil {
			return fmt.Errorf("could not connect to bastion host")
		}
		log.DebugF("Connected successfully to bastion host %s", bastionAddr)
	}

	config := &ssh.ClientConfig{}
	if len(s.privateKeys) > 0 {
		log.DebugF("Initial ssh privater keys auth to master host\n")

		AuthMethods := []ssh.AuthMethod{ssh.PublicKeys(signers...)}

		config = &ssh.ClientConfig{
			User:            s.Settings.User,
			Auth:            AuthMethods,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	} else if len(app.BecomePass) > 0 {
		log.DebugF("Initial password auth to master host\n")
		config = &ssh.ClientConfig{
			User: s.Settings.User,
			Auth: []ssh.AuthMethod{
				ssh.Password(app.BecomePass),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	} else {
		return fmt.Errorf("no authentication config for SSH found")
	}

	var targetConn net.Conn
	var clientConn ssh.Conn

	s.Settings.ChoiceNewHost()
	addr := fmt.Sprintf("%s:%s", s.Settings.Host(), s.Settings.Port)

	config.Timeout = 10 * time.Second
	config.BannerCallback = func(message string) error {
		return nil
	}

	if bastionClient == nil {
		log.DebugF("Try to direct connect host master host %s\n", addr)

		var err error
		client, err = ssh.Dial("tcp", addr, config)
		if err != nil {
			return fmt.Errorf("failed to connect to host: %w", err)
		}

		s.sshClient = client

		return nil
	}

	log.DebugF("Try to connect to through bastion host master host %s\n", addr)

	targetConn, err := bastionClient.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to target host through bastion host: %w", err)
	}
	targetClientConn, targetNewChan, targetReqChan, err := ssh.NewClientConn(targetConn, addr, config)
	if err != nil {
		return fmt.Errorf("failed to create client connection to target host: %w", err)
	}
	clientConn = targetClientConn
	client = ssh.NewClient(targetClientConn, targetNewChan, targetReqChan)

	s.sshClient = client
	s.BastionClient = bastionClient
	s.NetConn = &targetConn
	s.SSHConn = &clientConn

	return nil
}

// Tunnel is used to open local (L) and remote (R) tunnels
func (s *Client) Tunnel(ttype, address string) node.Tunnel {
	return nil
}

// ReverseTunnel is used to open remote (R) tunnel
func (s *Client) ReverseTunnel(address string) node.ReverseTunnel {
	return nil
}

// Command is used to run commands on remote server
func (s *Client) Command(name string, arg ...string) node.Command {
	return nil
}

// KubeProxy is used to start kubectl proxy and create a tunnel from local port to proxy port
func (s *Client) KubeProxy() node.KubeProxy {
	return nil
}

// File is used to upload and download files and directories
func (s *Client) File() node.File {
	return nil
}

// UploadScript is used to upload script and execute it on remote server
func (s *Client) UploadScript(scriptPath string, args ...string) node.Script {
	return nil
}

// UploadScript is used to upload script and execute it on remote server
func (s *Client) Check() node.Check {
	return nil
}

// Stop the client
func (s *Client) Stop() {
}

func (s *Client) Session() *session.Session {
	return s.Settings
}

func (s *Client) PrivateKeys() []session.AgentPrivateKey {
	return s.privateKeys
}

// Loop Looping all available hosts
func (s *Client) Loop(fn node.SSHLoopHandler) error {
	var err error

	resetSession := func() {
		s.Settings = s.Settings.Copy()
		s.Settings.ChoiceNewHost()
	}
	defer resetSession()
	resetSession()

	for range s.Settings.AvailableHosts() {
		err = fn(s)
		if err != nil {
			return err
		}
		s.Settings.ChoiceNewHost()
	}

	return nil
}
