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

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/frontend"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
)

const (
	DefaultLocalPort  = 20000
	DefaultRemotePort = 20000
)

func CheckSSHTunel(sess *session.Session, localPort, remotePort int) error {
	log.DebugF("Checking ssh tunnel with remote port %s and local port %d\n", remotePort, localPort)
	if localPort == 0 {
		localPort = DefaultLocalPort
	}
	if remotePort == 0 {
		remotePort = DefaultRemotePort
	}

	builder := strings.Builder{}
	builder.WriteString(strconv.Itoa(localPort))
	builder.WriteString(":localhost:")
	builder.WriteString(strconv.Itoa(remotePort))

	tun := frontend.NewTunnel(sess, "L", builder.String())
	err := tun.Up()
	if err != nil {
		return err
	}

	log.DebugLn("Checking ssh tunnel success")
	tun.Stop()
	return nil
}
