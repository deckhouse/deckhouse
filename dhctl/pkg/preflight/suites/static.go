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
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

type StaticDeps struct {
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
	MetaConfig             *config.MetaConfig
	// LegacyMode reflects whether the SSH client uses the legacy clissh
	// backend. Threaded into RegistryProxy for the SSH tunnel direction.
	LegacyMode bool
}

func NewStaticSuite(deps StaticDeps, ctx context.Context) (preflight.Suite, error) {
	nodeInterface, err := helper.GetNodeInterface(ctx, deps.SSHProviderInitializer, deps.SSHProviderInitializer.GetSettings())
	dc := &directoryconfig.DirectoryConfig{
		DownloadDir:      deps.MetaConfig.DownloadRootDir,
		DownloadCacheDir: deps.MetaConfig.DownloadCacheDir,
	}

	return preflight.NewSuite(
		checks.CidrIntersectionStatic(deps.MetaConfig),
		checks.StaticInstancesIPDuplication(deps.MetaConfig),
		checks.SingleSSHHost(deps.SSHProviderInitializer),
		checks.SSHCredential(deps.SSHProviderInitializer),
		checks.SSHTunnel(deps.SSHProviderInitializer, dc),
		checks.StaticInstancesSSHCredentials(deps.MetaConfig, deps.SSHProviderInitializer),
		checks.DeckhouseUser(nodeInterface, dc),
		checks.StaticSystemRequirements(deps.SSHProviderInitializer),
		checks.Python(nodeInterface),
		checks.RegistryProxy(deps.MetaConfig, deps.SSHProviderInitializer, deps.LegacyMode),
		checks.Ports(deps.SSHProviderInitializer, dc),
		checks.LocalhostDomain(nodeInterface, dc),
		checks.SudoAllowed(nodeInterface),
		checks.TimeDrift(nodeInterface),
	), err
}
