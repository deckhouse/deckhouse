// Copyright 2026 Flant JSC
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
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

// TODO(nabokikhms): fix package level setters in the following PRs.
//
// These package-level vars replace the dhctl/pkg/app globals this package used
// to read directly. Set once at startup via SetGlobals.
var (
	sshHosts         []session.Host
	sshUser          string
	sshPort          string
	sshBastionHost   string
	sshBastionPort   string
	sshBastionUser   string
	sshExtraArgs     string
	pkgBecomeOptions options.BecomeOptions
)

// SetGlobals wires in SSH options at startup.
// TODO(nabokikhms): fix package level setters in the following PRs.
func SetGlobals(opts *options.Options) {
	if opts == nil {
		return
	}
	sshHosts = opts.SSH.Hosts
	sshUser = opts.SSH.User
	sshPort = opts.SSH.Port
	sshBastionHost = opts.SSH.BastionHost
	sshBastionPort = opts.SSH.BastionPort
	sshBastionUser = opts.SSH.BastionUser
	sshExtraArgs = opts.SSH.ExtraArgs
	pkgBecomeOptions = opts.Become
}
