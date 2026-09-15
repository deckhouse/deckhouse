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

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

type StaticDeps struct {
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
	MetaConfig             *config.MetaConfig
	InstallConfig          *config.DeckhouseInstaller

	// LegacyMode reflects whether the SSH client uses the legacy clissh
	// backend. Threaded into RegistryProxy for the SSH tunnel direction.
	LegacyMode bool
	GlobalOpts *options.GlobalOptions
}

// NewStaticSuite assembles the checks; it opens nothing. The connection to the node is resolved
// by each check when it runs — see checks.NodeInterfaceFunc. Building it here used to open SSH
// during the pre-infra phase, before the preflight header had been printed, so a wrong credential
// arrived as a raw lib-connection line after a silent retry loop and static-ssh-credential, the
// check for exactly that, ran afterwards and was cosmetic.
func NewStaticSuite(deps StaticDeps) preflight.Suite {
	nodeInterface := nodeInterfaceResolver(deps.SSHProviderInitializer)

	built := make([]preflight.Check, 0, 11+len(nodeChecks(nodeCheckDeps{})))
	built = append(built,
		checks.StaticInstancesIPDuplication(deps.MetaConfig),
		checks.SingleSSHHost(nodeInterface),
		checks.BastionAvailability(deps.SSHProviderInitializer),
		checks.SSHConnectivity(deps.SSHProviderInitializer),
		checks.SSHCredential(nodeInterface, endpointOf(deps.SSHProviderInitializer)),
		checks.SSHTunnel(deps.SSHProviderInitializer, deps.GlobalOpts),
		checks.StaticInstancesSSHAccess(deps.MetaConfig, deps.SSHProviderInitializer),
		checks.NodeSystemRequirements(nodeInterface, deps.InstallConfig),
		checks.RegistryProxy(deps.MetaConfig, deps.SSHProviderInitializer, deps.LegacyMode),
		checks.RegistryFromMaster(deps.MetaConfig, deps.SSHProviderInitializer),
		checks.Ports(deps.SSHProviderInitializer, deps.GlobalOpts),
		// Static only: a cloud master arrives on a disk dhctl asked the provider for, and it is
		// empty. A static node is a machine that has been doing something else.
		checks.StaticFreeDiskSpace(nodeInterface),
	)

	// The questions asked of the machine itself, shared with the cloud path. They declare no
	// dependency on the SSH checks above: those stop the phase outright when they fail, because
	// nothing here — and nothing in the bootstrap that follows — can be done over a connection
	// that was refused.
	built = append(built, nodeChecks(nodeCheckDeps{
		MetaConfig:    deps.MetaConfig,
		InstallConfig: deps.InstallConfig,
		GlobalOpts:    deps.GlobalOpts,
		NodeInterface: nodeInterface,
	})...)

	return preflight.NewSuite(built...)
}

// nodeInterfaceResolver defers helper.GetNodeInterface to the moment a check runs.
func nodeInterfaceResolver(initializer *providerinitializer.SSHProviderInitializer) checks.NodeInterfaceFunc {
	return func(ctx context.Context) (libcon.Interface, error) {
		return helper.GetNodeInterface(ctx, initializer, initializer.GetSettings())
	}
}

// endpointOf names the machine from the configuration, for the failures that happen before there
// is a connection to read it from.
func endpointOf(initializer *providerinitializer.SSHProviderInitializer) checks.EndpointFunc {
	return checks.EndpointOfConfig(initializer.GetConfig())
}
