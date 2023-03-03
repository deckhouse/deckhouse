/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"os"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/hooks/ensure_crds"
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
		if !internal.Contains(istioVersions, versionResult.String()) {
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
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		prefix += "ee/"
	}
	return ensure_crds.EnsureCRDsHandler(prefix+"modules/110-istio/crds/istio/"+CRDversionToInstall+"/*.yaml")(input, dc)
}
