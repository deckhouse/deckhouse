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
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/frontend"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

var (
	agentInstanceSingleton sync.Once
	agentInstance          *frontend.Agent
)

// initializeNewInstance disables singleton logic
func initAgentInstance(privateKeys []session.AgentPrivateKey, initializeNewInstance bool) (*frontend.Agent, error) {
	var err error

	if initializeNewInstance {
		inst := frontend.NewAgent(&session.AgentSettings{
			PrivateKeys: privateKeys,
		})

		err = inst.Start()
		return inst, err
	}

	agentInstanceSingleton.Do(func() {
		if agentInstance == nil {
			inst := frontend.NewAgent(&session.AgentSettings{
				PrivateKeys: privateKeys,
			})

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

func NewClient(session *session.Session, privKeys []session.AgentPrivateKey) *Client {
	return &Client{
		Settings:    session,
		PrivateKeys: privKeys,

		// We use arbitrary privKeys param, so always reinitialize agent with privKeys
		InitializeNewAgent: true,
	}
}

type Client struct {
	Settings *session.Session
	Agent    *frontend.Agent

	PrivateKeys        []session.AgentPrivateKey
	InitializeNewAgent bool

	kubeProxies []*frontend.KubeProxy
}

func (s *Client) Start() (*Client, error) {
	if s.Settings == nil {
		return nil, fmt.Errorf("possible bug in ssh client: session should be created before start")
	}

	a, err := initAgentInstance(s.PrivateKeys, s.InitializeNewAgent)
	if err != nil {
		return nil, err
	}
	s.Agent = a
	s.Settings.AgentSettings = s.Agent.AgentSettings

	return s, nil
}

// Easy access to frontends

// Tunnel is used to open local (L) and remote (R) tunnels
func (s *Client) Tunnel(ttype, address string) *frontend.Tunnel {
	return frontend.NewTunnel(s.Settings, ttype, address)
}

// ReverseTunnel is used to open remote (R) tunnel
func (s *Client) ReverseTunnel(address string) *frontend.ReverseTunnel {
	return frontend.NewReverseTunnel(s.Settings, address)
}

// Command is used to run commands on remote server
func (s *Client) Command(name string, arg ...string) *frontend.Command {
	return frontend.NewCommand(s.Settings, name, arg...)
}

// KubeProxy is used to start kubectl proxy and create a tunnel from local port to proxy port
func (s *Client) KubeProxy() *frontend.KubeProxy {
	p := frontend.NewKubeProxy(s.Settings)
	s.kubeProxies = append(s.kubeProxies, p)
	return p
}

// File is used to upload and download files and directories
func (s *Client) File() *frontend.File {
	return frontend.NewFile(s.Settings)
}

// UploadScript is used to upload script and execute it on remote server
func (s *Client) UploadScript(scriptPath string, args ...string) *frontend.UploadScript {
	return frontend.NewUploadScript(s.Settings, scriptPath, args...)
}

// UploadScript is used to upload script and execute it on remote server
func (s *Client) Check() *frontend.Check {
	return frontend.NewCheck(s.Settings)
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
