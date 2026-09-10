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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks"
)

type ImmutableDeps struct {
	MetaConfig    *config.MetaConfig
	BootstrapOpts *options.BootstrapOptions
	GlobalOpts    *options.GlobalOptions
	CommanderMode bool
	// MachinesAvailability reaches the machines named with --master-host. Supplied by
	// the bootstrapper, which owns the tunnel to them; nil where there are none.
	MachinesAvailability func(context.Context) error
}

// NewImmutableSuite gathers the checks that only apply when the master
// NodeGroup asks for systemType: Immutable. It is added to the runner only in
// that case, so none of its checks has to test for it again.
func NewImmutableSuite(deps ImmutableDeps) preflight.Suite {
	return preflight.NewSuite(
		checks.ImmutableSupportedProvider(deps.MetaConfig),
		checks.ImmutableInstallerImages(deps.MetaConfig),
		checks.ImmutableRegistryMode(deps.MetaConfig),
		checks.ImmutableSignatureMode(deps.MetaConfig, deps.GlobalOpts),
		checks.ImmutablePostBootstrapScript(deps.BootstrapOpts),
		checks.ImmutableKubeconfigOut(deps.BootstrapOpts, deps.CommanderMode),
		checks.ImmutableKubeconfigKept(deps.BootstrapOpts, deps.GlobalOpts),
		checks.ImmutableMachinesAvailability(deps.MachinesAvailability),
	)
}

// NewImmutableStaticSuite is what a static cluster of immutable machines can be checked for. It
// takes the two checks of NewStaticSuite that read the configuration and nothing else: every
// other one is built over an SSH provider this path has none of, or over the node interface
// helper.GetNodeInterface hands back without one — the installer container itself.
func NewImmutableStaticSuite(metaConfig *config.MetaConfig) preflight.Suite {
	return preflight.NewSuite(
		checks.CidrIntersectionStatic(metaConfig),
		checks.StaticInstancesIPDuplication(metaConfig),
	)
}
