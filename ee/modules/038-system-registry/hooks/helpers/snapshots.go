/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package helpers

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

func SnapshotToMap[TKey comparable, TValue any](input *go_hook.HookInput, name string) (map[TKey]TValue, error) {
	ret := make(map[TKey]TValue)

	snapshot, ok := input.Snapshots[name]
	if !ok {
		return ret, fmt.Errorf("no snapshot with name \"%v\" found", name)
	}

	for _, val := range snapshot {
		if val == nil {
			continue
		}

		if kv, ok := val.(KeyValue[TKey, TValue]); ok {
			ret[kv.Key] = kv.Value
		} else {
			return ret, fmt.Errorf("snapshot value of type %T not convertible to %T", val, kv)
		}
	}

	return ret, nil
}

func SnapshotToSingle[TValue any](input *go_hook.HookInput, name string) (TValue, error) {
	var value TValue

	snapshot, ok := input.Snapshots[name]
	if !ok {
		return value, fmt.Errorf("no snapshot with name \"%v\" found", name)
	}

	snapLen := len(snapshot)

	if snapLen != 1 {
		return value, fmt.Errorf("snapshot contains values count %d != 1", snapLen)
	}

	snapValue := snapshot[0]
	value, ok = snapValue.(TValue)

	if !ok {
		return value, fmt.Errorf("snapshot value of type %T not convertible to %T", snapValue, value)
	}

	return value, nil
}

func SnapshotToList[TValue any](input *go_hook.HookInput, name string) ([]TValue, error) {
	snapshot, ok := input.Snapshots[name]
	if !ok {
		return []TValue{}, fmt.Errorf("no snapshot with name \"%v\" found", name)
	}

	ret := make([]TValue, 0, len(snapshot))
	for _, snap := range snapshot {
		if snap == nil {
			continue
		}

		value, ok := snap.(TValue)

		if !ok {
			return ret, fmt.Errorf("snapshot value of type %T not convertible to %T", snap, value)
		}

		ret = append(ret, value)
	}

	return ret, nil
}
