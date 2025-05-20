/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package helpers

import (
	"slices"

	"golang.org/x/exp/constraints"
)

func DeduplicateSlice[T comparable](in []T) []T {
	seen := make(map[T]struct{}, len(in))
	var ret []T
	for _, v := range in {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			ret = append(ret, v)
		}
	}
	return ret
}

func DeduplicateAndSortSlice[T constraints.Ordered](in []T) []T {
	ret := DeduplicateSlice(in)
	slices.Sort(ret)
	return ret
}
