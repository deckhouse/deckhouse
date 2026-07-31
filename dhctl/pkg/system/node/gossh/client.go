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
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"slices"
	"sync"
	"time"

	ssh "github.com/deckhouse/lib-gossh"
	"github.com/deckhouse/lib-gossh/agent"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	genssh "github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var (
	errSSHClientNeverStarted = errors.New("ssh client has not been started")
	errSSHClientStopped      = errors.New("ssh client has been stopped")
)

func NewClient(ctx context.Context, session *session.Session, privKeys []session.AgentPrivateKey) *Client {
	return &Client{
		Settings:    session,
		privateKeys: privKeys,
		live:        false,
		sessionList: make([]*ssh.Session, 5),
		ctx:         ctx,
		silent:      false,
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
	stopGen  uint64
	started  bool
	stopped  bool

	kubeProxies []*KubeProxy
	sessionList []*ssh.Session

	signers []ssh.Signer

	ctx          context.Context
	sessionMutex sync.Mutex
	clientMutex  sync.RWMutex

	silent bool
}

func (s *Client) initSigners() error {
	if len(s.signers) > 0 {
		log.DebugF("Signers already initialized\n")
		return nil
	}

	signers := make([]ssh.Signer, 0, len(s.privateKeys))
	for _, keypath := range s.privateKeys {
		key, err := genssh.GetSSHPrivateKey(keypath.Key, keypath.Passphrase)
		if err != nil {
			return err
		}
		signer, err := ssh.NewSignerFromKey(key)
		if err != nil {
			return fmt.Errorf("unable to parse private key: %v", err)
		}
		signers = append(signers, signer)
	}

	s.signers = signers
	return nil
}

func (s *Client) OnlyPreparePrivateKeys() error {
	return s.initSigners()
}

func (s *Client) Start() error {
	return s.start(false, 0)
}

func (s *Client) start(automatic bool, expectedStopGen uint64) error {
	stopGen, err := s.prepareStart(automatic, expectedStopGen)
	if err != nil {
		return err
	}

	if s.ctx != nil {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
		}
	}
	if s.Settings == nil {
		return fmt.Errorf("possible bug in ssh client: session should be created before start")
	}

	if !automatic {
		s.resetForStart()
	}

	log.DebugLn("Starting go ssh client....")

	if err := s.initSigners(); err != nil {
		return err
	}

	var agentClient agent.ExtendedAgent
	socket := os.Getenv("SSH_AUTH_SOCK")
	var socketConn net.Conn
	if socket != "" {
		log.DebugLn("Dialing SSH agent unix socket...")
		var err error
		socketConn, err = net.Dial("unix", socket)
		if err != nil {
			return fmt.Errorf("Failed to open SSH_AUTH_SOCK: %v", err)
		}
		defer socketConn.Close()
		agentClient = agent.NewClient(socketConn)
	}

	var bastionClient *ssh.Client
	bastionClientOwned := false
	var client *ssh.Client
	if s.Settings.BastionHost != "" {
		bastionConfig := &ssh.ClientConfig{}
		log.DebugLn("Initialize bastion connection...")

		var bastionPass string

		if s.Settings.BastionPassword != "" {
			bastionPass = s.Settings.BastionPassword
		} else {
			bastionPass = app.SSHBastionPass
		}

		if len(s.privateKeys) == 0 && len(bastionPass) == 0 {
			return fmt.Errorf("No credentials present to connect to bastion host")
		}

		AuthMethods := []ssh.AuthMethod{ssh.PublicKeys(s.signers...)}

		if len(bastionPass) > 0 {
			log.DebugF("Initial password auth to bastion host\n")
			AuthMethods = append(AuthMethods, ssh.Password(bastionPass))
		}

		if socket != "" {
			AuthMethods = append(AuthMethods, ssh.PublicKeysCallback(agentClient.Signers))
		}

		bastionConfig = &ssh.ClientConfig{
			User:            s.Settings.BastionUser,
			Auth:            AuthMethods,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         3 * time.Second,
		}
		bastionAddr := fmt.Sprintf("%s:%s", s.Settings.BastionHost, s.Settings.BastionPort)
		var err error
		fullHost := fmt.Sprintf("bastion host '%s' with user '%s'", bastionAddr, s.Settings.BastionUser)
		connectToBastion := func() error {
			log.DebugF("Connect to %s\n", fullHost)
			bastionClient, err = DialTimeout(s.ctx, "tcp", bastionAddr, bastionConfig)
			return err
		}
		if s.silent {
			err = retry.NewSilentLoop("Get bastion SSH client", 30, 5*time.Second).RunContext(s.ctx, connectToBastion)
		} else {
			err = retry.NewLoop("Get bastion SSH client", 30, 5*time.Second).RunContext(s.ctx, connectToBastion)
		}

		if err != nil {
			return fmt.Errorf("Could not connect to %s", fullHost)
		}
		bastionClientOwned = true
		defer func() {
			if bastionClientOwned && bastionClient != nil {
				_ = bastionClient.Close()
			}
		}()
		log.DebugF("Connected successfully to bastion host %s\n", bastionAddr)
	}

	var becomePass string

	if s.Settings.BecomePass != "" {
		becomePass = s.Settings.BecomePass
	} else {
		becomePass = app.BecomePass
	}

	if len(s.privateKeys) == 0 && len(becomePass) == 0 && socket == "" {
		return fmt.Errorf("one of SSH keys, SSH_AUTH_SOCK environment variable or become password should be not empty")
	}

	log.DebugF("Initial ssh privater keys auth to master host\n")

	AuthMethods := []ssh.AuthMethod{ssh.PublicKeys(s.signers...)}

	if socket != "" {
		log.DebugF("Adding agent socket to auth methods\n")
		AuthMethods = []ssh.AuthMethod{ssh.PublicKeysCallback(agentClient.Signers)}
	}

	if len(becomePass) > 0 {
		log.DebugF("Initial password auth to master host\n")
		AuthMethods = append(AuthMethods, ssh.Password(becomePass))
	}

	config := &ssh.ClientConfig{
		User:            s.Settings.User,
		Auth:            AuthMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	var targetConn net.Conn
	var clientConn ssh.Conn

	config.BannerCallback = func(message string) error {
		return nil
	}

	if bastionClient == nil {
		log.DebugLn("Try to direct connect host master host")

		connectToHost := func() error {
			if len(s.kubeProxies) == 0 {
				s.Settings.ChoiceNewHost()
			}

			addr := fmt.Sprintf("%s:%s", s.Settings.Host(), s.Settings.Port)
			log.DebugF("Connect to master host '%s' with user '%s'\n", addr, s.Settings.User)
			client, err = DialTimeout(s.ctx, "tcp", addr, config)
			return err
		}
		if s.silent {
			err = retry.NewSilentLoop("Get SSH client", 30, 5*time.Second).RunContext(s.ctx, connectToHost)
		} else {
			err = retry.NewLoop("Get SSH client", 30, 5*time.Second).RunContext(s.ctx, connectToHost)
		}

		if err != nil {
			lastHost := fmt.Sprintf("'%s:%s' with user '%s'", s.Settings.Host(), s.Settings.Port, s.Settings.User)
			return fmt.Errorf("Failed to connect to master host (last %s): %w", lastHost, err)
		}

		if err = s.setConnectionState(stopGen, client, nil, nil, nil, true); err != nil {
			return err
		}

		return nil
	}

	log.DebugF("Try to connect to through bastion host master host \n")

	var (
		addr             string
		targetClientConn ssh.Conn
		targetNewChan    <-chan ssh.NewChannel
		targetReqChan    <-chan *ssh.Request
	)
	connectToTarget := func() error {
		if len(s.kubeProxies) == 0 {
			s.Settings.ChoiceNewHost()
		}
		addr = fmt.Sprintf("%s:%s", s.Settings.Host(), s.Settings.Port)
		log.DebugF("Connect to target host '%s' with user '%s' through bastion host\n", addr, s.Settings.User)
		targetConn, err = bastionClient.DialContext(s.ctx, "tcp", addr)
		if err != nil {
			return err
		}
		if app.IsDebug {
			targetClientConn, targetNewChan, targetReqChan, err = ssh.NewClientConnWithDebug(targetConn, addr, config, logger.NewLogger(&slog.LevelVar{}))
		} else {
			targetClientConn, targetNewChan, targetReqChan, err = ssh.NewClientConn(targetConn, addr, config)
		}
		if err != nil {
			_ = targetConn.Close()
		}

		return err
	}
	if s.silent {
		err = retry.NewSilentLoop("Get SSH client and connect to target host", 50, 2*time.Second).RunContext(s.ctx, connectToTarget)
	} else {
		err = retry.NewLoop("Get SSH client and connect to target host", 50, 2*time.Second).RunContext(s.ctx, connectToTarget)
	}

	if err != nil {
		lastHost := fmt.Sprintf("'%s:%s' with user '%s'", s.Settings.Host(), s.Settings.Port, s.Settings.User)
		return fmt.Errorf("Failed to connect to target host through bastion host (last %s): %w", lastHost, err)
	}

	clientConn = targetClientConn
	client = ssh.NewClient(targetClientConn, targetNewChan, targetReqChan)

	if err = s.setConnectionState(stopGen, client, &clientConn, &targetConn, bastionClient, true); err != nil {
		return err
	}
	bastionClientOwned = false

	return nil
}

func (s *Client) keepAlive(stopCh chan struct{}, stopGen uint64) {
	defer log.DebugLn("keep-alive goroutine stopped")
	errorsCount := 0
	for {
		select {
		case <-stopCh:
			log.DebugLn("Stopping keep-alive goroutine.")
			s.clientMutex.Lock()
			if s.stopChan == stopCh {
				s.stopChan = nil
			}
			s.clientMutex.Unlock()
			return
		default:
			session, err := s.NewSession()
			if err != nil {
				log.DebugF("Keep-alive to %s failed: %v\n", s.Settings.Host(), err)
				if errorsCount > 3 {
					s.restart(stopCh, stopGen)
					return
				}
				errorsCount++
				time.Sleep(5 * time.Second)
				continue
			}
			if _, err := session.SendRequest("keepalive@openssh.com", false, nil); err != nil {
				log.DebugF("Keep-alive failed: %v\n", err)
				if errorsCount > 3 {
					_ = session.Close()
					s.restart(stopCh, stopGen)
					return
				}
				errorsCount++
			}
			session.Close()
			for _, sess := range s.sessionsSnapshot() {
				if sess != nil {
					if _, err := sess.SendRequest("keepalive@openssh.com", false, nil); err != nil {
						log.DebugF("Keep-alive for session failed: %v\n", err)
					}
				} else {
					s.UnregisterSession(sess)
				}
			}
			time.Sleep(5 * time.Second)
		}
	}
}

func (s *Client) restart(stopCh chan struct{}, expectedStopGen uint64) {
	stopGen, ok := s.claimAutoRestart(stopCh, expectedStopGen)
	if !ok {
		return
	}

	s.setLive(false)
	s.closeSessions()
	s.closeConnections()
	s.silent = true
	if err := s.start(true, stopGen); err != nil {
		log.DebugF("SSH client restart failed: %v\n", err)
	}
}

func (s *Client) closeSessions() {
	log.DebugLn("closing sessions")
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	for _, sess := range s.sessionList {
		if sess != nil {
			_ = sess.Signal(ssh.SIGKILL)
			_ = sess.Close()
		}
	}
	s.sessionList = nil
}

func (s *Client) closeConnections() {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	if s.sshClient != nil {
		s.sshClient.Close()
		s.sshClient = nil
	}
	if s.SSHConn != nil {
		sshconn := *s.SSHConn
		sshconn.Close()
		s.SSHConn = nil
	}
	if s.NetConn != nil {
		netconn := *s.NetConn
		netconn.Close()
		s.NetConn = nil
	}
	if s.BastionClient != nil {
		s.BastionClient.Close()
		s.BastionClient = nil
	}
	s.live = false
}

func (s *Client) resetForStart() {
	if !s.hasPublishedConnection() {
		return
	}

	log.DebugLn("closing existing SSH client before start")
	s.stopKeepAlive()
	s.closeSessions()
	s.closeConnections()
}

func (s *Client) hasPublishedConnection() bool {
	s.clientMutex.RLock()
	defer s.clientMutex.RUnlock()

	return s.live ||
		s.stopChan != nil ||
		s.sshClient != nil ||
		s.SSHConn != nil ||
		s.NetConn != nil ||
		s.BastionClient != nil
}

func (s *Client) setConnectionState(
	stopGen uint64,
	client *ssh.Client,
	sshConn *ssh.Conn,
	netConn *net.Conn,
	bastionClient *ssh.Client,
	live bool,
) error {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	if stopGen != s.stopGen {
		if client != nil {
			client.Close()
		}
		if sshConn != nil {
			(*sshConn).Close()
		}
		if netConn != nil {
			(*netConn).Close()
		}
		if bastionClient != nil {
			bastionClient.Close()
		}
		return fmt.Errorf("ssh client start interrupted by stop")
	}

	s.sshClient = client
	s.SSHConn = sshConn
	s.NetConn = netConn
	s.BastionClient = bastionClient
	s.live = live
	s.started = true

	if live && s.stopChan == nil {
		stopCh := make(chan struct{})
		s.stopChan = stopCh
		go s.keepAlive(stopCh, stopGen)
	}

	return nil
}

func (s *Client) prepareStart(automatic bool, expectedStopGen uint64) (uint64, error) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	if automatic {
		if s.stopped || s.stopGen != expectedStopGen {
			return 0, fmt.Errorf("ssh client automatic restart was stopped")
		}
	} else {
		s.stopped = false
		s.stopGen++
	}

	return s.stopGen, nil
}

func (s *Client) claimAutoRestart(stopCh chan struct{}, expectedStopGen uint64) (uint64, bool) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	if s.stopped || s.stopChan != stopCh || s.stopGen != expectedStopGen {
		return 0, false
	}

	s.stopChan = nil
	return expectedStopGen, true
}

func (s *Client) setLive(live bool) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()
	s.live = live
}

func (s *Client) getSSHClient() (*ssh.Client, bool) {
	s.clientMutex.RLock()
	defer s.clientMutex.RUnlock()

	if s.sshClient == nil || !s.live {
		return nil, false
	}

	return s.sshClient, true
}

func (s *Client) snapshotSSHClient() (*ssh.Client, error) {
	return s.snapshotSSHClientWithOptions(false)
}

func (s *Client) snapshotSSHClientWithOptions(allowStopped bool) (*ssh.Client, error) {
	s.clientMutex.RLock()
	defer s.clientMutex.RUnlock()

	if s.stopped && !allowStopped {
		return nil, errSSHClientStopped
	}

	if s.sshClient == nil || !s.live {
		if !s.started {
			return nil, errSSHClientNeverStarted
		}
		if s.stopped {
			return nil, errSSHClientStopped
		}
		return nil, fmt.Errorf("ssh client is not connected")
	}

	return s.sshClient, nil
}

func (s *Client) NewSession() (*ssh.Session, error) {
	return s.newSession(false)
}

func (s *Client) newSession(allowStopped bool) (*ssh.Session, error) {
	sshClient, err := s.snapshotSSHClientWithOptions(allowStopped)
	if err != nil {
		return nil, err
	}

	return sshClient.NewSession()
}

func (s *Client) withSSHClient(fn func(*ssh.Client) error) error {
	sshClient, err := s.snapshotSSHClient()
	if err != nil {
		return err
	}

	return fn(sshClient)
}

func DialTimeout(ctx context.Context, network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	d := net.Dialer{Timeout: config.Timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		conn.Close()
		return nil, err
	}

	err = tcpConn.SetKeepAlive(true)
	if err != nil {
		tcpConn.Close()
		return nil, err
	}

	timeFactor := time.Duration(3)
	err = tcpConn.SetDeadline(time.Now().Add(config.Timeout * timeFactor))
	if err != nil {
		tcpConn.Close()
		return nil, err
	}

	var (
		c     ssh.Conn
		chans <-chan ssh.NewChannel
		reqs  <-chan *ssh.Request
	)

	if app.IsDebug {
		c, chans, reqs, err = ssh.NewClientConnWithDebug(tcpConn, addr, config, logger.NewLogger(&slog.LevelVar{}))
	} else {
		c, chans, reqs, err = ssh.NewClientConn(tcpConn, addr, config)
	}
	if err != nil {
		tcpConn.Close()
		return nil, err
	}

	err = tcpConn.SetDeadline(time.Time{})
	if err != nil {
		tcpConn.Close()
		return nil, err
	}

	return ssh.NewClient(c, chans, reqs), nil
}

// Tunnel is used to open local (L) and remote (R) tunnels
func (s *Client) Tunnel(address string) node.Tunnel {
	return NewTunnel(s, address)
}

// ReverseTunnel is used to open remote (R) tunnel
func (s *Client) ReverseTunnel(address string) node.ReverseTunnel {
	return NewReverseTunnel(s, address)
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
	return NewSSHFile(s)
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
	s.clientMutex.Lock()
	s.stopGen++
	s.stopped = true
	s.clientMutex.Unlock()

	log.DebugLn("stopping kube proxies")
	for _, p := range s.kubeProxies {
		p.StopAll()
	}
	s.kubeProxies = nil

	s.closeSessions()

	// by starting kubeproxy on remote, there is one more process starts
	// it cannot be killed by sending any signal to his parrent process
	// so we need to use killall command to kill all this processes
	if _, ok := s.getSSHClient(); ok {
		log.DebugLn("stopping kube proxies on remote")
		s.stopKubeproxy()
		log.DebugLn("kube proxies on remote were stopped")
	} else {
		log.DebugLn("no SSH client found to stop remote kube proxies. Skip.")
	}

	log.DebugLn("stopping keep-alive goroutine")
	s.stopKeepAlive()

	s.closeConnections()
	log.DebugLn("SSH Client is stopped")
}

func (s *Client) Session() *session.Session {
	return s.Settings
}

func (s *Client) PrivateKeys() []session.AgentPrivateKey {
	return s.privateKeys
}

func (s *Client) RefreshPrivateKeys() error {
	// new go ssh client already have all keys
	return nil
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
	s.clientMutex.RLock()
	defer s.clientMutex.RUnlock()
	if !s.live {
		return nil
	}
	return s.sshClient
}

func (s *Client) stopKeepAlive() {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	if s.stopChan == nil {
		return
	}

	log.DebugLn("sendind message to stop keep-alive")
	close(s.stopChan)
	s.stopChan = nil
}

func (s *Client) Live() bool {
	s.clientMutex.RLock()
	defer s.clientMutex.RUnlock()
	return s.live
}

func (s *Client) RegisterSession(sess *ssh.Session) {
	if sess == nil {
		return
	}
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	s.sessionList = append(s.sessionList, sess)
}

func (s *Client) sessionsSnapshot() []*ssh.Session {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	sessions := make([]*ssh.Session, len(s.sessionList))
	copy(sessions, s.sessionList)
	return sessions
}

func (s *Client) UnregisterSession(sess *ssh.Session) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	num := len(s.sessionList)
	for i, s := range s.sessionList {
		if s == sess {
			num = i
			break
		}
	}
	if num < len(s.sessionList) {
		s.sessionList = slices.Delete(s.sessionList, num, num+1)
	}
}

func (s *Client) stopKubeproxy() {
	cmd := newSSHCommand(s, "killall kubectl", true)
	cmd.Sudo(context.Background())
	_ = cmd.Run(context.Background())
}
