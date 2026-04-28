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
	"context"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

type StaticDeps struct {
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
	MetaConfig             *config.MetaConfig
}

func NewStaticSuite(deps StaticDeps, ctx context.Context) (preflight.Suite, error) {
	nodeInterface, err := helper.GetNodeInterface(ctx, deps.SSHProviderInitializer, deps.SSHProviderInitializer.GetSettings())
	return preflight.NewSuite(
		checks.CidrIntersectionStatic(deps.MetaConfig),
		checks.StaticInstancesIPDuplication(deps.MetaConfig),
		checks.SingleSSHHost(deps.SSHProviderInitializer),
		checks.SSHCredential(deps.SSHProviderInitializer),
		checks.SSHTunnel(deps.SSHProviderInitializer),
		checks.StaticInstancesSSHCredentials(deps.MetaConfig, deps.SSHProviderInitializer),
		checks.DeckhouseUser(nodeInterface),
		checks.StaticSystemRequirements(deps.SSHProviderInitializer),
		checks.Python(nodeInterface),
		checks.RegistryProxy(deps.MetaConfig, deps.SSHProviderInitializer),
		checks.Ports(deps.SSHProviderInitializer),
		checks.LocalhostDomain(nodeInterface),
		checks.SudoAllowed(nodeInterface),
		checks.TimeDrift(nodeInterface),
	), err
}
