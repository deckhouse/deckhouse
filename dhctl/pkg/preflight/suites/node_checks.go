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
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks"
)

type nodeCheckDeps struct {
	MetaConfig    *config.MetaConfig
	InstallConfig *config.DeckhouseInstaller
	GlobalOpts    *options.GlobalOptions
	NodeInterface checks.NodeInterfaceFunc
	// KubeDataDevicePath is the provider's separate disk for Kubernetes data, where there is one.
	// nil on a static cluster.
	KubeDataDevicePath func() string
}

// nodeChecks are the questions asked of a machine that exists: can a command be run on it, does
// it have what the bootstrap will install, is its clock right, is its network what the cluster was
// told it is.
//
// They are the same questions whoever created the machine, and they were asked only of a static
// cluster. A cloud master got none of them, so a missing sudo package became 150 seconds of
// retries and "Timeout while \"Execute 01-bootstrap-prerequisites.sh\"", a missing python became
// a hundred re-runs of the bundle, and a wrong clock became "kubernetes API is not ready" via an
// x509 certificate that was not yet valid. A t3.micro passes cloud-master-system-requirements —
// which reads the instance class, not the machine — and then fails to run a control plane.
//
// They are named node-*, not static-*: the prefix used to say which suite a check happened to
// live in, and now that the same checks run on both paths it would say the wrong thing. The old
// names are still accepted by --preflight-skip-check (legacyPreflightSkipAliases), so a pipeline
// that carries one keeps working.
func nodeChecks(deps nodeCheckDeps) []preflight.Check {
	return []preflight.Check{
		checks.SudoInstalled(deps.NodeInterface),
		checks.SudoAllowed(deps.NodeInterface).After(checks.SudoInstalledCheckName),
		checks.DeckhouseUser(deps.NodeInterface, deps.GlobalOpts),
		checks.Python(deps.NodeInterface),
		checks.LocalhostDomain(deps.NodeInterface, deps.GlobalOpts),
		checks.TimeDrift(deps.NodeInterface),
		checks.HostNetworkCIDRIntersection(deps.MetaConfig, deps.NodeInterface),
		checks.NodeHostname(deps.NodeInterface),
		checks.NodeLeftovers(deps.NodeInterface),
		checks.NodeCRIRequirements(deps.MetaConfig, deps.NodeInterface),
		checks.NodeKernelModules(deps.MetaConfig, deps.NodeInterface),
		checks.NodeSELinuxTools(deps.NodeInterface),
		checks.NodeDiskSpace(deps.NodeInterface, deps.KubeDataDevicePath),
		checks.NodeInternalNetwork(deps.MetaConfig, deps.NodeInterface),
		checks.NodeOSSupported(deps.NodeInterface),
		checks.NodeXFSFtype(deps.NodeInterface),
		checks.NodeResolveHostname(deps.NodeInterface),
	}
}
