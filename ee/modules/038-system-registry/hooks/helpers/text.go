/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package helpers

import "slices"

func TrimWithEllipsis(value string) string {
	const limit = 15
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(slices.Clone(runes[:limit])) + "…"
}
