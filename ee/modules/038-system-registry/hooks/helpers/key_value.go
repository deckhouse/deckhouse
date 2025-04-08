/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package helpers

import "github.com/flant/addon-operator/pkg/module_manager/go_hook"

type KeyValue[TKey comparable, TValue any] struct {
	Key   TKey
	Value TValue
}

func SnapshotToMap[TKey comparable, TValue any](snapshot []go_hook.FilterResult) map[TKey]TValue {
	ret := make(map[TKey]TValue)

	for _, val := range snapshot {
		if kv, ok := val.(KeyValue[TKey, TValue]); ok {
			ret[kv.Key] = kv.Value
		}
	}

	return ret
}
