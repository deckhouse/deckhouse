/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bashible

import (
	"slices"
	"strings"
)

type ModeType = string

const (
	ModeUnmanaged ModeType = "Unmanaged"
	ModeDirect    ModeType = "Direct"
	ModeProxy     ModeType = "Proxy"

	// The same:
	ModeDetached ModeType = "Detached" // TODO: remove
	ModeLocal    ModeType = "Local"
)

func ToModeType(mode string) ModeType {
	val := strings.ToLower(mode)
	switch val {
	case "direct":
		return ModeDirect
	case "proxy":
		return ModeProxy
	case "detached":
		return ModeDetached
	case "local":
		return ModeLocal
	default:
		return ModeUnmanaged
	}
}

func ShouldRunStaticPodRegistry(mode ModeType) bool {
	staticPodsRegistryModes := []string{ModeProxy, ModeDetached, ModeLocal}

	return slices.Contains(staticPodsRegistryModes, mode)
}
