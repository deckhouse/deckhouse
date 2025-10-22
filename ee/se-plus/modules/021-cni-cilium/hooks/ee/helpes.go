/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

func Filter[T any](slice []T, fn func(T) bool) []T {
	result := make([]T, 0, len(slice))

	for _, v := range slice {
		if fn(v) {
			result = append(result, v)
		}
	}

	return result
}
