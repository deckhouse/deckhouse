/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bashible

import (
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
)

const (
	inputValuesMode = "systemRegistry.mode"
)

func getMode(input *go_hook.HookInput) registry_const.ModeType {
	val := strings.ToLower(input.Values.Get(inputValuesMode).Str)
	return registry_const.ToModeType(val)
}
