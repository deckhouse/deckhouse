/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package registry

import (
	"encoding/base64"
	"fmt"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	init_config "github.com/deckhouse/deckhouse/go_lib/registry/models/initconfig"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"
)

// FoldLegacyDirectIntoInit reads a `Direct` section of the deckhouse ModuleConfig as a statement of
// WHERE the cluster pulls from, rather than as a request for the previous implementation, and moves
// it into InitConfiguration.
//
// It exists for one producer. Commander writes `settings.registry` with `mode: Direct` and the
// registry the cluster already uses whenever nobody stated a registry at all, and the e2e suites of
// this repository install clusters exactly that way. Nothing in this release can serve that mode —
// none of the previous implementation's objects render any more — so an installation that honoured
// it would point the nodes at the in-cluster address and then leave nobody behind it.
//
// Read as an address, the same configuration installs: `useInitConfig` turns InitConfiguration into
// `Unmanaged` with LegacyMode set, which leaves the registry module managing nothing and keeps dhctl
// from writing the registry section back into the ModuleConfig of the installed cluster. The nodes
// then pull straight from the registry that section named, which is what `Direct` meant to whoever
// wrote it.
//
// An InitConfiguration that already names a registry wins and is left alone: the two describe the
// same registry on a cluster Commander installs, and that one is the operator's own statement.
//
// Only `Direct` is folded. `Local` is how an installation from a bundle is expressed inside dhctl,
// `Proxy` is refused earlier with a message of its own, and `Unmanaged` already means what this
// produces.
func FoldLegacyDirectIntoInit(
	init *init_config.Config,
	settings *module_config.DeckhouseSettings,
) (*init_config.Config, *module_config.DeckhouseSettings, error) {
	if settings == nil || settings.Mode != constant.ModeDirect {
		return init, settings, nil
	}

	if init != nil && !init.IsEmpty() {
		return init, nil, nil
	}

	// Through New().Merge() so that a section naming only some of the fields is folded with the
	// same defaults the mode would have been given.
	direct := module_config.New(constant.ModeDirect).Merge(settings).Direct
	if direct == nil {
		return init, settings, nil
	}

	folded := init_config.Config{
		ImagesRepo:     direct.ImagesRepo,
		RegistryScheme: string(direct.Scheme),
		RegistryCA:     direct.CA,
	}

	// A license is the same credentials under another name — expanded here exactly as
	// Data.fromRegistrySettings expands it for the modes that keep it.
	username, password := direct.Username, direct.Password
	if direct.License != "" {
		username, password = constant.LicenseUsername, direct.License
	}

	if username != "" || password != "" {
		address, _ := helpers.SplitAddressAndPath(direct.ImagesRepo)

		dockerCfg, err := helpers.DockerCfgFromCreds(username, password, address)
		if err != nil {
			return nil, nil, fmt.Errorf("build the docker configuration for %q: %w", address, err)
		}

		folded.RegistryDockerCfg = base64.StdEncoding.EncodeToString(dockerCfg)
	}

	return &folded, nil, nil
}
