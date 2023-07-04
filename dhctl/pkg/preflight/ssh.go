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
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
)

const (
	DefaultLocalPort  = 1234
	DefaultRemotePort = 1234
)

func CheckSSHTunel(sshClient *ssh.Client, localPort, remotePort int) error {
	if app.PreflightSkipSSHForword {
		log.InfoLn("Skip SSH forward preflight check")
		return nil
	}

	if localPort == 0 {
		localPort = DefaultLocalPort
	}
	if remotePort == 0 {
		remotePort = DefaultRemotePort
	}
	log.DebugF("Checking ssh tunnel with remote port %d and local port %d\n", remotePort, localPort)

	builder := strings.Builder{}
	builder.WriteString(strconv.Itoa(localPort))
	builder.WriteString(":localhost:")
	builder.WriteString(strconv.Itoa(remotePort))

	tun := sshClient.Tunnel("L", builder.String())
	err := tun.Up()
	if err != nil {
		return err
	}

	log.DebugLn("Checking ssh tunnel success")
	tun.Stop()
	return nil
}
