/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, handleSetRegistryMode)

func handleSetRegistryMode(input *go_hook.HookInput) error {

	mode, exists := input.Values.GetOk("systemRegistry.registryMode")

	// TODO, some preparations before setting registry mode
	// this is like a stub for now

	// Do we really need to set default value?
	if !exists {
		input.Values.Set("systemRegistry.internal.registryMode", "Direct")
	} else {
		input.Values.Set("systemRegistry.internal.registryMode", mode.String())
	}

	return nil
}
