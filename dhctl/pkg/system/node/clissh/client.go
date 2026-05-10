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

package clissh

import (
	"fmt"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh/frontend"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

var (
	agentInstanceSingleton sync.Once
	agentInstance          *frontend.Agent
)

// initializeNewInstance disables singleton logic
func initAgentInstance(
	privateKeys []session.AgentPrivateKey,
	initializeNewInstance bool,
	isDebug bool,
	passphrases map[string]string,
) (*frontend.Agent, error) {
	var err error

	if initializeNewInstance {
		inst := frontend.NewAgent(&session.AgentSettings{
			PrivateKeys: privateKeys,
		}).WithIsDebug(isDebug).WithPassphrases(passphrases)

		err = inst.Start()
		return inst, err
	}

	agentInstanceSingleton.Do(func() {
		if agentInstance == nil {
			inst := frontend.NewAgent(&session.AgentSettings{
				PrivateKeys: privateKeys,
			}).WithIsDebug(isDebug).WithPassphrases(passphrases)

			err = inst.Start()
			if err != nil {
				return
			}
			tomb.RegisterOnShutdown("Stop ssh-agent", func() {
				if agentInstance != nil {
					agentInstance.Stop()
				}
			})

			agentInstance = inst
		}
	})

	if err != nil {
		// NOTICE: agentInstance will remain nil forever in the case of err, so give it another try in the next possible init-retry
		agentInstanceSingleton = sync.Once{}
	}

	return agentInstance, err
}

// Config bundles the per-Client values that used to live in clissh package
// globals — the sudo password, scratch directory, debug flag, and the SSH
// key-path → passphrase fallback map.
type Config struct {
	BecomePass  string
	TmpDir      string
	IsDebug     bool
	Passphrases map[string]string
}

// NewClient builds a clissh.Client with the supplied Session/private keys.
// Per-Client knobs (BecomePass, TmpDir, IsDebug, Passphrases) default to
// zero values; pass NewClientFromConfig or chain With* to set them.
func NewClient(session *session.Session, privKeys []session.AgentPrivateKey, initNewAgent bool) *Client {
	return &Client{
		Settings:    session,
		privateKeys: privKeys,

		// We use arbitrary privKeys param, so always reinitialize agent with privKeys
		InitializeNewAgent: initNewAgent,
	}
}

// NewClientFromConfig builds a Client and copies the per-call settings from
// cfg onto it. Use it instead of NewClient + With* chains when the full
// configuration is known up-front.
func NewClientFromConfig(session *session.Session, privKeys []session.AgentPrivateKey, initNewAgent bool, cfg Config) *Client {
	return NewClient(session, privKeys, initNewAgent).
		WithBecomePass(cfg.BecomePass).
		WithTmpDir(cfg.TmpDir).
		WithIsDebug(cfg.IsDebug).
		WithPassphrases(cfg.Passphrases)
}

type Client struct {
	Settings *session.Session
	Agent    *frontend.Agent

	privateKeys        []session.AgentPrivateKey
	InitializeNewAgent bool

	// BecomePass is the sudo password forwarded to spawned Commands.
	BecomePass string
	// TmpDir is the local scratch directory used by File/UploadScript.
	TmpDir string
	// IsDebug toggles ssh/scp -vvv on every spawned subprocess.
	IsDebug bool
	// Passphrases maps SSH private-key path to passphrase, consulted as a
	// fallback by frontend.Agent when the operator-provided passphrase is empty.
	Passphrases map[string]string

	kubeProxies []*frontend.KubeProxy
}

// WithBecomePass sets the sudo password. Returns the receiver for chaining.
func (s *Client) WithBecomePass(p string) *Client {
	s.BecomePass = p
	return s
}

// WithTmpDir sets the local scratch directory. Returns the receiver for chaining.
func (s *Client) WithTmpDir(d string) *Client {
	s.TmpDir = d
	return s
}

// WithIsDebug toggles verbose subprocess logging. Returns the receiver for chaining.
func (s *Client) WithIsDebug(d bool) *Client {
	s.IsDebug = d
	return s
}

// WithPassphrases supplies the SSH key-path → passphrase fallback map. Returns
// the receiver for chaining.
func (s *Client) WithPassphrases(p map[string]string) *Client {
	s.Passphrases = p
	return s
}

func (s *Client) OnlyPreparePrivateKeys() error {
	// Double start is safe here because for initializing private keys we are using sync.Once
	return s.Start()
}

func (s *Client) Start() error {
	if s.Settings == nil {
		return fmt.Errorf("possible bug in ssh client: session should be created before start")
	}

	a, err := initAgentInstance(s.privateKeys, s.InitializeNewAgent, s.IsDebug, s.Passphrases)
	if err != nil {
		return err
	}
	s.Agent = a
	s.Settings.AgentSettings = s.Agent.AgentSettings

	return nil
}

// Easy access to frontends

// Tunnel is used to open local (L) and remote (R) tunnels
func (s *Client) Tunnel(address string) node.Tunnel {
	return frontend.NewTunnel(s.Settings, "L", address).WithIsDebug(s.IsDebug)
}

// ReverseTunnel is used to open remote (R) tunnel
func (s *Client) ReverseTunnel(address string) node.ReverseTunnel {
	return frontend.NewReverseTunnel(s.Settings, address).WithIsDebug(s.IsDebug)
}

// Command is used to run commands on remote server
func (s *Client) Command(name string, arg ...string) node.Command {
	return frontend.NewCommand(s.Settings, name, arg...).WithIsDebug(s.IsDebug).WithBecomePass(s.BecomePass)
}

// KubeProxy is used to start kubectl proxy and create a tunnel from local port to proxy port
func (s *Client) KubeProxy() node.KubeProxy {
	p := frontend.NewKubeProxy(s.Settings)
	s.kubeProxies = append(s.kubeProxies, p)
	return p
}

// File is used to upload and download files and directories
func (s *Client) File() node.File {
	return frontend.NewFile(s.Settings).WithIsDebug(s.IsDebug).WithTmpDir(s.TmpDir)
}

// UploadScript is used to upload script and execute it on remote server
func (s *Client) UploadScript(scriptPath string, args ...string) node.Script {
	return frontend.NewUploadScript(s.Settings, scriptPath, args...).
		WithIsDebug(s.IsDebug).
		WithBecomePass(s.BecomePass).
		WithTmpDir(s.TmpDir)
}

// UploadScript is used to upload script and execute it on remote server
func (s *Client) Check() node.Check {
	isDebug, becomePass := s.IsDebug, s.BecomePass
	return ssh.NewCheck(func(sess *session.Session, cmd string) node.Command {
		return frontend.NewCommand(sess, cmd).WithIsDebug(isDebug).WithBecomePass(becomePass)
	}, s.Settings)
}

// Stop the client
func (s *Client) Stop() {
	// stop agent on shutdown because agent is singleton

	if s.InitializeNewAgent {
		s.Agent.Stop()
		s.Agent = nil
		s.Settings.AgentSettings = nil
	}
	for _, p := range s.kubeProxies {
		p.StopAll()
	}
	s.kubeProxies = nil
}

func (s *Client) Session() *session.Session {
	return s.Settings
}

func (s *Client) PrivateKeys() []session.AgentPrivateKey {
	return s.privateKeys
}

func (s *Client) RefreshPrivateKeys() error {
	return s.Agent.AddKeys(s.PrivateKeys())
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
