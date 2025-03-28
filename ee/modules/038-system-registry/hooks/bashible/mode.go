/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bashible

import (
	"slices"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

type modeType string

const (
	inputValuesMode = "systemRegistry.mode"
)

const (
	modeUnmanaged modeType = "Unmanaged"
	modeDirect    modeType = "Direct"
	modeProxy     modeType = "Proxy"
	modeLocal     modeType = "Local"
)

func getMode(input *go_hook.HookInput) modeType {
	val := strings.ToLower(input.Values.Get(inputValuesMode).Str)

	switch val {
	case "direct":
		return modeDirect
	case "proxy":
		return modeProxy
	case "local":
		return modeLocal
	default:
		return modeUnmanaged
	}
}

func shouldRunStaticPodRegistry(mode modeType) bool {
	return slices.Contains([]string{string(modeProxy), string(modeLocal)}, string(mode))
}
