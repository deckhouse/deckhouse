// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
)

type PreflightCheck interface {
	CloudCheck() error
	StaticCheck() error
}

type preflightCheck struct {
	sshClient        *ssh.Client
	tunnelLocalPort  int
	tunnelRemotePort int
}

func NewPreflightCheck(sshClient *ssh.Client) PreflightCheck {
	return &preflightCheck{
		sshClient:        sshClient,
		tunnelLocalPort:  DefaultTunnelLocalPort, // TODO: add cli param
		tunnelRemotePort: DefaultTunnelRemotePort,
	}
}

func (pc *preflightCheck) StaticCheck() error {
	return log.Process("common", "Preflight Checks", func() error {
		if app.PreflightSkipAll {
			log.InfoLn("Skip all preflight checks")
			return nil
		}
		return pc.CheckSSHTunel()
	})
}

func (pc *preflightCheck) CloudCheck() error {
	return nil
}
