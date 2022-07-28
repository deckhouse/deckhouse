/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/hooks/ensure_crds"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup:    &go_hook.OrderedConfig{Order: 10}, // Order matters — we need globalVersion from discovery_revisions.go
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10}, // Order matters — we need globalVersion from discovery_revisions.go
}, dependency.WithExternalDependencies(ensureCRDs))

func ensureCRDs(input *go_hook.HookInput, dc dependency.Container) error {
	var theNewestVersion string

	var globalVersion string
	if !input.Values.Exists("istio.internal.globalVersion") {
		return fmt.Errorf("istio.internal.globalVersion value isn't discovered by revisions_discovery.go yet")
	}
	globalVersion = input.Values.Get("istio.internal.globalVersion").String()
	var additionalVersions = make([]string, 0)
	for _, versionResult := range input.ConfigValues.Get("istio.additionalVersions").Array() {
		additionalVersions = append(additionalVersions, versionResult.String())
	}

	for _, versionResult := range input.Values.Get("istio.internal.supportedVersions").Array() {
		version := versionResult.String()
		if version == globalVersion || internal.Contains(additionalVersions, version) {
			theNewestVersion = version
		}
	}

	return ensure_crds.EnsureCRDsHandler("/deckhouse/modules/110-istio/crds/istio/"+theNewestVersion+"/*.yaml")(input, dc)
}
