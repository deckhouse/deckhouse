/*
Copyright 2023 Flant JSC

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

package hooks

import (
	"fmt"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/hooks/ensure_crds"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup:    &go_hook.OrderedConfig{Order: 10}, // Order matters — we need globalVersion from discovery_versions_to_install.go
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10}, // Order matters — we need globalVersion from discovery_versions_to_install.go
}, dependency.WithExternalDependencies(ensureCRDs))

func ensureCRDs(input *go_hook.HookInput, dc dependency.Container) error {
	// collect all istio versions (global + additional | uniq)
	istioVersions := make([]string, 0)

	if !input.Values.Get("istio.internal.globalVersion").Exists() {
		return fmt.Errorf("istio.internal.globalVersion value isn't discovered by discovery_versions.go yet")
	}
	globalVersion := input.Values.Get("istio.internal.globalVersion").String()
	istioVersions = append(istioVersions, globalVersion)

	for _, versionResult := range input.ConfigValues.Get("istio.additionalVersions").Array() {
		if !lib.Contains(istioVersions, versionResult.String()) {
			istioVersions = append(istioVersions, versionResult.String())
		}
	}

	// semvers is a slice for sorting by semver
	semvers := make([]*semver.Version, len(istioVersions))
	for i, version := range istioVersions {
		v, err := semver.NewVersion(version)
		if err != nil {
			return err
		}
		semvers[i] = v
	}

	sort.Sort(semver.Collection(semvers))

	CRDversionToInstall := fmt.Sprintf("%d.%d", semvers[len(semvers)-1].Major(), semvers[len(semvers)-1].Minor())

	prefix := "/deckhouse/"
	return ensure_crds.EnsureCRDsHandler(prefix+"modules/110-istio/crds/istio/"+CRDversionToInstall+"/*.yaml")(input, dc)
}
