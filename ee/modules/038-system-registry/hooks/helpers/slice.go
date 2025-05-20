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
	var result []T
	for _, v := range in {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

func DeduplicateAndSortSlice[T constraints.Ordered](in []T) []T {
	result := DeduplicateSlice(in)
	slices.Sort(result)
	return result
}
