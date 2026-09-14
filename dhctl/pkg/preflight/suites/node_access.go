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

package suites

import (
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// NodeAccessDeps is the one dependency these checks have.
type NodeAccessDeps struct {
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
}

// NewNodeAccessSuite is the smallest suite there is: can dhctl log in to the machines, and can it
// run a command on them. It is what every command that touches existing nodes needs before it
// starts — abort, converge, destroy, check — and none of them had it. An operator with a key the
// node does not accept waited about four minutes of "Try to connect to host" before the timeout;
// one without sudo waited for "Timeout while \"Get Kubernetes API client\"".
func NewNodeAccessSuite(deps NodeAccessDeps) preflight.Suite {
	nodeInterface := nodeInterfaceResolver(deps.SSHProviderInitializer)
	return preflight.NewSuite(
		checks.SSHCredential(nodeInterfaceResolver(deps.SSHProviderInitializer)),
		checks.SudoInstalled(nodeInterface),
		checks.SudoAllowed(nodeInterface).After(checks.SudoInstalledCheckName),
	)
}
