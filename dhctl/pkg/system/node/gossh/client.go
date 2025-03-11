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

package gossh

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	genssh "github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
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

	stopChan chan struct{}
}

func (s *Client) Start() error {
	if s.Settings == nil {
		return fmt.Errorf("possible bug in ssh client: session should be created before start")
	}

	log.DebugLn("Starting go ssh client....")

	signers := make([]ssh.Signer, 0, len(s.privateKeys))
	for _, keypath := range s.privateKeys {
		key, err := genssh.ParsePrivateSSHKey(keypath.Key, []byte(keypath.Passphrase))
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
		err = retry.NewSilentLoop("Get bastion SSH client", 10, 15*time.Second).Run(func() error {
			bastionClient, err = ssh.Dial("tcp", bastionAddr, bastionConfig)
			return err
		})
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
		err = retry.NewSilentLoop("Get SSH client", 10, 15*time.Second).Run(func() error {
			client, err = ssh.Dial("tcp", addr, config)
			return err
		})
		if err != nil {
			return fmt.Errorf("failed to connect to host: %w", err)
		}

		s.sshClient = client

		if s.stopChan == nil {
			go s.keepAlive()
		}

		return nil
	}

	log.DebugF("Try to connect to through bastion host master host %s\n", addr)

	var err error
	err = retry.NewSilentLoop("Get SSH client", 10, 15*time.Second).Run(func() error {
		targetConn, err = bastionClient.Dial("tcp", addr)
		return err
	})
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

	if s.stopChan == nil {
		go s.keepAlive()
	}

	return nil
}

func (s *Client) keepAlive() {
	for {
		select {
		case <-s.stopChan:
			log.DebugLn("Stopping keep-alive goroutine.")
			return
		default:
			time.Sleep(30 * time.Second)
			session, err := s.sshClient.NewSession()
			if err != nil {
				log.DebugF("Keep-alive failed: %v", err)
				s.Start()
				return
			}
			if _, err := session.SendRequest("keepalive", false, nil); err != nil {
				log.DebugF("Keep-alive failed: %v", err)
				s.Start()
				return
			}
		}
	}
}

// Tunnel is used to open local (L) and remote (R) tunnels
func (s *Client) Tunnel(address string) node.Tunnel {
	return NewTunnel(s.sshClient, address)
}

// ReverseTunnel is used to open remote (R) tunnel
func (s *Client) ReverseTunnel(address string) node.ReverseTunnel {
	return NewReverseTunnel(s.sshClient, address)
}

// Command is used to run commands on remote server
func (s *Client) Command(name string, arg ...string) node.Command {
	return NewSSHCommand(s.sshClient, name, arg...)
}

// KubeProxy is used to start kubectl proxy and create a tunnel from local port to proxy port
func (s *Client) KubeProxy() node.KubeProxy {
	return NewKubeProxy(s.sshClient, s.Settings)
}

// File is used to upload and download files and directories
func (s *Client) File() node.File {
	return NewSSHFile(s.sshClient)
}

// UploadScript is used to upload script and execute it on remote server
func (s *Client) UploadScript(scriptPath string, args ...string) node.Script {
	return NewSSHUploadScript(s.sshClient, scriptPath, args...)
}

// UploadScript is used to upload script and execute it on remote server
func (s *Client) Check() node.Check {
	return genssh.NewCheck(func(sess *session.Session, cmd string) node.Command {
		return NewSSHCommand(s.sshClient, cmd)
	}, s.Settings)
}

// Stop the client
func (s *Client) Stop() {
	close(s.stopChan)
	s.sshClient.Close()
	if s.SSHConn != nil {
		sshconn := *s.SSHConn
		sshconn.Close()
	}
	if s.NetConn != nil {
		netconn := *s.NetConn
		netconn.Close()
	}
	if s.BastionClient != nil {
		s.BastionClient.Close()
	}
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

func (s *Client) GetClient() *ssh.Client {
	return s.sshClient
}
