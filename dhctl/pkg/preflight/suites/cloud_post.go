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
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

type PostCloudDeps struct {
	MetaConfig    *config.MetaConfig
	InstallConfig *config.DeckhouseInstaller
	GlobalOpts    *options.GlobalOptions
	// SSHProviderInitializer rather than a built SSHProvider: this suite is constructed before
	// the master exists, and a provider built then carries no hosts. The checks resolve one
	// when they run, which is after the infrastructure they need to reach has been created.
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
	// KubeDataDevicePath reports the disk the provider attached for /mnt/kubernetes-data. It is
	// an output of the infrastructure, so it is read when the check runs rather than now; nil
	// where the layout keeps Kubernetes data on the root disk.
	KubeDataDevicePath func() string
	// MasterAPIEndpoint says how to reach the API port of an immutable first master, resolved
	// when the check runs because the address is an output of the infrastructure. nil on every
	// bootstrap whose master is not an immutable one.
	MasterAPIEndpoint func(context.Context) (*checks.MasterAPIEndpoint, error)
}

// NewPostCloudSuite is what is asked of a cloud cluster once its master exists.
//
// Until now that was one check. Everything else dhctl knows how to ask of a machine — sudo,
// python, the clock, the hostname, the disk, what is already installed on it — was asked only of
// a static cluster, and a cloud master went into bashible unexamined.
func NewPostCloudSuite(deps PostCloudDeps) preflight.Suite {
	nodeInterface := nodeInterfaceResolver(deps.SSHProviderInitializer)

	built := make([]preflight.Check, 0, 6+len(nodeChecks(nodeCheckDeps{})))
	built = append(built,
		checks.BastionAvailabilityAfterInfra(deps.SSHProviderInitializer),
		// First, and it stops the phase: everything below is asked over this connection. Without
		// it a wrong --ssh-user reached whichever check happened to run first, and each of them
		// reported the failure as its own subject — cloud-api-accessibility blamed sshd's
		// AllowTcpForwarding for a login that never happened. The cloud suite had no credential
		// check at all; only the static one did.
		//
		// AfterInfra, not the plain one: this phase runs between creating the machine and waiting
		// for it, so the first minutes of refusals are the machine booting, not a bad credential.
		checks.SSHCredentialAfterInfra(nodeInterface, endpointOf(deps.SSHProviderInitializer)),
		// Declared, not merely relied on: the credential check is what proves this connection
		// and carries the wait for the machine to boot, so this one no longer probes it itself.
		checks.CloudAPIAccess(deps.MetaConfig, deps.SSHProviderInitializer, endpointOf(deps.SSHProviderInitializer)).
			After(checks.SSHCredentialCheckName),
		checks.RegistryFromMaster(deps.MetaConfig, deps.SSHProviderInitializer),
		checks.NodeSystemRequirements(nodeInterface, deps.InstallConfig),
		checks.CloudKubeDataDevice(deps.KubeDataDevicePath, nodeInterface),
		checks.ImmutableAPIReachable(deps.MasterAPIEndpoint),
	)

	built = append(built, nodeChecks(nodeCheckDeps{
		MetaConfig:    deps.MetaConfig,
		InstallConfig: deps.InstallConfig,
		GlobalOpts:    deps.GlobalOpts,
		NodeInterface: nodeInterface,
	})...)

	return preflight.NewSuite(built...)
}
