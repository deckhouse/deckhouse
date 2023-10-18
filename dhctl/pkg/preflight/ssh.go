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
	"fmt"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const (
	DefaultTunnelLocalPort  = 22322
	DefaultTunnelRemotePort = 22322
)

func (pc *PreflightCheck) CheckSSHTunel() error {
	if app.PreflightSkipSSHForword {
		log.InfoLn("SSH forward preflight check was skipped")
		return nil
	}

	log.DebugF("Checking ssh tunnel with remote port %d and local port %d\n", DefaultTunnelRemotePort, DefaultTunnelLocalPort)

	builder := strings.Builder{}
	builder.WriteString(strconv.Itoa(DefaultTunnelLocalPort))
	builder.WriteString(":localhost:")
	builder.WriteString(strconv.Itoa(DefaultTunnelRemotePort))

	tun := pc.sshClient.Tunnel("L", builder.String())
	err := tun.Up()
	if err != nil {
		return fmt.Errorf(`Cannot setup tunnel to control-plane host: %w.
Please check connectivity to control-plane host or
check that sshd config 'AllowTcpForwarding' set to 'yes' on control-plane node.`, err)
	}

	log.InfoLn("Checking ssh tunnel success")
	tun.Stop()
	return nil
}
