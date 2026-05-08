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

package sshclient

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

// TODO(nabokikhms): fix package level setters in the following PRs.
//
// Set once at startup via SetGlobals from the resolved *options.Options.
var (
	sshLegacyMode  bool
	sshModernMode  bool
	sshPrivateKeys []string
	sshHosts       []session.Host
	tmpDir         string
)

// SetGlobals wires in options at startup.
// TODO(nabokikhms): fix package level setters in the following PRs.
func SetGlobals(opts *options.Options) {
	if opts == nil {
		return
	}
	sshLegacyMode = opts.SSH.LegacyMode
	sshModernMode = opts.SSH.ModernMode
	sshPrivateKeys = opts.SSH.PrivateKeys
	sshHosts = opts.SSH.Hosts
	tmpDir = opts.Global.TmpDir
}
