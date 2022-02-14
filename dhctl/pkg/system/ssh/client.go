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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/frontend"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
)

var (
	agentInstanceSingleton sync.Once
	agentInstance          *frontend.Agent
)

func initAgentInstance() (*frontend.Agent, error) {
	var err error

	agentInstanceSingleton.Do(func() {
		if agentInstance == nil {
			inst := frontend.NewAgent(&session.AgentSettings{
				PrivateKeys: app.SSHPrivateKeys,
			})

			err = inst.Start()
			if err != nil {
				return
			}
			tomb.RegisterOnShutdown("Stop ssh-agent", func() {
				agentInstance.Stop()
			})

			agentInstance = inst
		}
	})

	return agentInstance, err
}

type Client struct {
	Settings *session.Session
	Agent    *frontend.Agent
}

func (s *Client) Start() (*Client, error) {
	if s.Settings == nil {
		return nil, fmt.Errorf("possible bug in ssh client: session should be created before start")
	}

	a, err := initAgentInstance()
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

// Command is used to run commands on remote server
func (s *Client) Command(name string, arg ...string) *frontend.Command {
	return frontend.NewCommand(s.Settings, name, arg...)
}

// KubeProxy is used to start kubectl proxy and create a tunnel from local port to proxy port
func (s *Client) KubeProxy() *frontend.KubeProxy {
	return frontend.NewKubeProxy(s.Settings)
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

// Stop stop client
func (s *Client) Stop() {
	// do nothing
	// stop agent on shutdown because agent is singleton
}
