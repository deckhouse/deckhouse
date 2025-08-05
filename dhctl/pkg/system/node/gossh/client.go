// Copyright 2025 Flant JSC
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
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

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
		live:        false,
		sessionList: make([]*ssh.Session, 5),
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
	live     bool

	kubeProxies []*KubeProxy
	sessionList []*ssh.Session
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

	var agentClient agent.ExtendedAgent
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket != "" {
		socketConn, err := net.Dial("unix", socket)
		if err != nil {
			return fmt.Errorf("Failed to open SSH_AUTH_SOCK: %v", err)
		}
		agentClient = agent.NewClient(socketConn)
	}

	var bastionClient *ssh.Client
	var client *ssh.Client
	if s.Settings.BastionHost != "" {
		bastionConfig := &ssh.ClientConfig{}
		log.DebugLn("Initialize bastion connection...")

		if len(s.privateKeys) == 0 && len(app.SSHBastionPass) == 0 {
			return fmt.Errorf("no credentials present to connect to bastion host")
		}

		AuthMethods := []ssh.AuthMethod{ssh.PublicKeys(signers...)}

		if len(app.SSHBastionPass) > 0 {
			log.DebugF("Initial password auth to bastion host\n")
			AuthMethods = append(AuthMethods, ssh.Password(app.SSHBastionPass))
		}

		if socket != "" {
			AuthMethods = append(AuthMethods, ssh.PublicKeysCallback(agentClient.Signers))
		}

		bastionConfig = &ssh.ClientConfig{
			User:            s.Settings.BastionUser,
			Auth:            AuthMethods,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		bastionAddr := fmt.Sprintf("%s:%s", s.Settings.BastionHost, s.Settings.BastionPort)
		var err error
		log.DebugF("Connect to bastion host %s\n", bastionAddr)
		err = retry.NewSilentLoop("Get bastion SSH client", 30, 5*time.Second).Run(func() error {
			bastionClient, err = ssh.Dial("tcp", bastionAddr, bastionConfig)
			return err
		})
		if err != nil {
			return fmt.Errorf("could not connect to bastion host")
		}
		log.DebugF("Connected successfully to bastion host %s", bastionAddr)
	}

	var becomePass string

	if s.Settings.BecomePass != "" {
		becomePass = s.Settings.BecomePass
	} else {
		becomePass = app.BecomePass
	}

	if len(s.privateKeys) == 0 && len(becomePass) == 0 && socket != "" {
		return fmt.Errorf("one of SSH keys, SSH_AUTH_SOCK environment variable or become password should be not empty")
	}

	log.DebugF("Initial ssh privater keys auth to master host\n")

	AuthMethods := []ssh.AuthMethod{ssh.PublicKeys(signers...)}

	if len(becomePass) > 0 {
		log.DebugF("Initial password auth to master host\n")
		AuthMethods = append(AuthMethods, ssh.Password(becomePass))
	}

	if socket != "" {
		AuthMethods = append(AuthMethods, ssh.PublicKeysCallback(agentClient.Signers))
	}

	config := &ssh.ClientConfig{
		User:            s.Settings.User,
		Auth:            AuthMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	var targetConn net.Conn
	var clientConn ssh.Conn

	s.Settings.ChoiceNewHost()
	addr := fmt.Sprintf("%s:%s", s.Settings.Host(), s.Settings.Port)

	config.Timeout = 30 * time.Second
	config.BannerCallback = func(message string) error {
		return nil
	}

	if bastionClient == nil {
		log.DebugF("Try to direct connect host master host %s\n", addr)

		var err error
		err = retry.NewSilentLoop("Get SSH client", 30, 5*time.Second).Run(func() error {
			client, err = ssh.Dial("tcp", addr, config)
			return err
		})
		if err != nil {
			return fmt.Errorf("failed to connect to host %s: %w", addr, err)
		}

		s.sshClient = client
		s.live = true

		if s.stopChan == nil {
			stopCh := make(chan struct{})
			s.stopChan = stopCh
			go s.keepAlive()
		}

		return nil
	}

	log.DebugF("Try to connect to through bastion host master host %s\n", addr)

	var err error
	err = retry.NewSilentLoop("Get SSH client", 30, 5*time.Second).Run(func() error {
		targetConn, err = bastionClient.Dial("tcp", addr)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to connect to target host through bastion host: %w", err)
	}
	var targetClientConn ssh.Conn
	var targetNewChan <-chan ssh.NewChannel
	var targetReqChan <-chan *ssh.Request
	err = retry.NewSilentLoop("Connect to target SSH host", 30, 5*time.Second).Run(func() error {
		targetClientConn, targetNewChan, targetReqChan, err = ssh.NewClientConn(targetConn, addr, config)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to create client connection to target host: %w", err)
	}

	clientConn = targetClientConn
	client = ssh.NewClient(targetClientConn, targetNewChan, targetReqChan)

	s.sshClient = client
	s.BastionClient = bastionClient
	s.NetConn = &targetConn
	s.SSHConn = &clientConn
	s.live = true

	if s.stopChan == nil {
		stopCh := make(chan struct{})
		s.stopChan = stopCh
		go s.keepAlive()
	}

	return nil
}

func (s *Client) keepAlive() {
	defer log.DebugLn("keep-alive goroutine stopped")
	for {
		select {
		case <-s.stopChan:
			log.DebugLn("Stopping keep-alive goroutine.")
			close(s.stopChan)
			s.stopChan = nil
			return
		default:
			session, err := s.sshClient.NewSession()
			if err != nil {
				log.DebugF("Keep-alive to %s failed: %v", s.Settings.Host(), err)
				s.live = false
				s.stopChan = nil
				s.Start()
				for _, sess := range s.sessionList {
					if sess != nil {
						sess.Signal(ssh.SIGKILL)
						sess.Close()
					}
				}
				s.sessionList = nil
				return
			}
			if _, err := session.SendRequest("keepalive", false, nil); err != nil {
				log.DebugF("Keep-alive failed: %v", err)
				s.live = false
				s.stopChan = nil
				s.Start()
				for _, sess := range s.sessionList {
					if sess != nil {
						sess.Signal(ssh.SIGKILL)
						sess.Close()
					}
				}
				s.sessionList = nil
				return
			}
			time.Sleep(30 * time.Second)
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
	return NewSSHCommand(s, name, arg...)
}

// KubeProxy is used to start kubectl proxy and create a tunnel from local port to proxy port
func (s *Client) KubeProxy() node.KubeProxy {
	p := NewKubeProxy(s, s.Settings)
	s.kubeProxies = append(s.kubeProxies, p)
	return p
}

// File is used to upload and download files and directories
func (s *Client) File() node.File {
	return NewSSHFile(s.sshClient)
}

// UploadScript is used to upload script and execute it on remote server
func (s *Client) UploadScript(scriptPath string, args ...string) node.Script {
	return NewSSHUploadScript(s, scriptPath, args...)
}

// UploadScript is used to upload script and execute it on remote server
func (s *Client) Check() node.Check {
	return genssh.NewCheck(func(sess *session.Session, cmd string) node.Command {
		return NewSSHCommand(s, cmd)
	}, s.Settings)
}

// Stop the client
func (s *Client) Stop() {
	log.DebugLn("SSH Client is stopping now")
	log.DebugLn("stopping kube proxies")
	for _, p := range s.kubeProxies {
		// log.InfoF("found non-stoped kube-proxy: %-v\n", p)
		p.StopAll()
	}
	s.kubeProxies = nil

	log.DebugLn("closing sessions")
	for _, sess := range s.sessionList {
		if sess != nil {
			sess.Signal(ssh.SIGKILL)
			sess.Close()
		}
	}
	s.sessionList = nil

	// by starting kubeproxy on remote, there is one more process starts
	// it cannot be killed by sending any signal to his parrent process
	// so we need to use killall command to kill all this processes
	log.DebugLn("stopping kube proxies on remote")
	s.stopKubeproxy()
	log.DebugLn("kube proxies on remote were stopped")

	log.DebugLn("stopping keep-alive goroutine")
	if s.stopChan != nil {
		log.DebugLn("sendind message to stop keep-alive")
		s.stopChan <- struct{}{}
	}

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
	log.DebugLn("SSH Client is stopped")
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

func (s *Client) Live() bool {
	return s.live
}

func (s *Client) RegisterSession(sess *ssh.Session) {
	s.sessionList = append(s.sessionList, sess)
}

func (s *Client) stopKubeproxy() {
	cmd := NewSSHCommand(s, "killall kubectl")
	cmd.Sudo(context.Background())
	cmd.Run(context.Background())
}
