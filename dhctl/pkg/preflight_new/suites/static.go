// Copyright 2025 Flant JSC
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
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new/checks"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

type StaticDeps struct {
	Node       node.Interface
	MetaConfig *config.MetaConfig
}

func NewStaticSuite(deps StaticDeps) preflightnew.Suite {
	return preflightnew.NewSuite(
		checks.CidrIntersectionStatic(deps.MetaConfig),
		checks.StaticInstancesIPDuplication(deps.MetaConfig),
		checks.SingleSSHHost(deps.Node),
		checks.SSHCredential(deps.Node),
		checks.SSHTunnel(deps.Node),
		checks.DeckhouseUser(deps.Node),
		checks.StaticSystemRequirements(deps.Node),
		checks.Python(deps.Node),
		checks.RegistryProxy(deps.MetaConfig, deps.Node),
		checks.Ports(deps.Node),
		checks.LocalhostDomain(deps.Node),
		checks.SudoAllowed(deps.Node),
		checks.TimeDrift(deps.Node),
	)
}
