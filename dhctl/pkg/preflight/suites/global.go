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

type GlobalDeps struct {
	MetaConfig    *config.MetaConfig
	InstallConfig *config.DeckhouseInstaller
	BuildInfo     options.BuildInfo
}

func NewGlobalSuite(deps GlobalDeps) preflight.Suite {
	// The image config is fetched once, by deckhouse-image-available, and read by every check
	// that only needs to look at a label on it.
	image := checks.NewDeckhouseImage()

	// The registry is reached, then authenticated to, then asked for the image — each step
	// declaring the one before it, so a registry that is simply unreachable produces one failure
	// instead of four.
	//
	// The CIDR and publicDomainTemplate checks that used to lead this suite are now part of
	// loading the configuration (pkg/config): they read nothing but the documents, and as
	// preflight checks they did not run for `dhctl config` or converge and were turned off by
	// --preflight-skip-all-checks.
	return preflight.NewSuite(
		checks.RegistryReachable(deps.MetaConfig),
		checks.RegistryCredentials(deps.MetaConfig, deps.InstallConfig).
			After(checks.RegistryReachableCheckName),
		checks.DeckhouseImageAvailable(deps.MetaConfig, deps.InstallConfig, image).
			After(checks.RegistryReachableCheckName, checks.RegistryCredentialsCheckName),
		checks.DhctlEdition(deps.BuildInfo, image).
			After(checks.DeckhouseImageAvailableCheckName),
		checks.DhctlVersion(deps.BuildInfo, image).
			After(checks.DeckhouseImageAvailableCheckName),
		// The tag being there says nothing about what is behind it: a registry filled by copying
		// the tag holds the Deckhouse image and none of the images it refers to.
		checks.RegistryRequiredImages(deps.MetaConfig).
			After(checks.RegistryReachableCheckName, checks.RegistryCredentialsCheckName),
		// Reads the documents and nothing else, so by the rule above it belongs in pkg/config
		// rather than here. Left where #22688 put it: relocating another team's check is not a
		// merge's business, and it is the one thing standing between a half-migrated
		// ClusterConfiguration/ModuleConfig pair and a silently picked winner.
		checks.NetworkSingleSource(deps.MetaConfig),
	)
}
