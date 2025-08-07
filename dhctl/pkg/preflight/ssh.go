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
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
)

const (
	DefaultTunnelLocalPort  = 22322
	DefaultTunnelRemotePort = 22322
)

var ErrAuthSSHFailed = errors.New("authentication failed")

func (pc *Checker) CheckSSHTunnel(_ context.Context) error {
	if app.PreflightSkipSSHForward {
		log.InfoLn("SSH forward preflight check was skipped (via skip flag)")
		return nil
	}

	wrapper, ok := pc.nodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		log.InfoLn("SSH forward preflight check was skipped (local run)")
		return nil
	}

	log.DebugF(
		"Checking ssh tunnel with remote port %d and local port %d\n",
		DefaultTunnelRemotePort,
		DefaultTunnelLocalPort,
	)

	localhost := "127.0.0.1"

	local := net.JoinHostPort(localhost, strconv.Itoa(DefaultTunnelLocalPort))
	remote := net.JoinHostPort(localhost, strconv.Itoa(DefaultTunnelRemotePort))
	addr := strings.Join([]string{local, remote}, ":")

	tun := wrapper.Client().ReverseTunnel(addr)
	err := tun.Up()
	if err != nil {
		return fmt.Errorf(`Cannot setup tunnel to control-plane host: %w.
Please check connectivity to control-plane host and that the sshd config parameters 'AllowTcpForwarding' is set to 'yes' and 'DisableForwarding' is set to 'no'  on the control-plane node.`, err)
	}

	tun.Stop()
	return nil
}

func (pc *Checker) CheckSSHCredential(ctx context.Context) error {
	if app.PreflightSkipSSHCredentialsCheck {
		log.InfoLn("SSH credentials preflight check was skipped (via skip flag)")
		return nil
	}

	wrapper, ok := pc.nodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		log.InfoLn("SSH credentials preflight check was skipped (local run)")
		return nil
	}

	sshCheck := wrapper.Client().Check()
	err := sshCheck.CheckAvailability(ctx)
	if err != nil {
		return fmt.Errorf(
			"ssh %w. Please check ssh credential and try again. Error: %w",
			ErrAuthSSHFailed, err,
		)
	}
	return nil
}

func (pc *Checker) CheckSingleSSHHostForStatic(_ context.Context) error {
	if app.PreflightSkipOneSSHHost {
		log.InfoLn("Only one --ssh-host parameter used preflight check was skipped (via skip flag)")
		return nil
	}

	wrapper, ok := pc.nodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		log.InfoLn("Only one --ssh-host parameter used preflight check was skipped (local run)")
		return nil
	}

	if len(wrapper.Client().Session().AvailableHosts()) > 1 {
		return fmt.Errorf(
			"during the bootstrap of the first static master node, only one --ssh-host parameter is allowed",
		)
	}
	return nil
}
