/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package helpers

type KeyValue[TKey comparable, TValue any] struct {
	Key   TKey
	Value TValue
}

func NewKeyValue[TKey comparable, TValue any](key TKey, value TValue) KeyValue[TKey, TValue] {
	return KeyValue[TKey, TValue]{
		Key:   key,
		Value: value,
	}
}
