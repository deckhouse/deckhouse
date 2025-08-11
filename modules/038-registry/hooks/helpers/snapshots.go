/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helpers

import (
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

var (
	ErrNoSnapshot        = errors.New("no snapshot found or too many snapshots")
	ErrSnapshotTypeError = errors.New("snapshot cannot be converted to requested type")
)

func SnapshotToMap[TKey comparable, TValue any](input *go_hook.HookInput, name string) (map[TKey]TValue, error) {
	ret := make(map[TKey]TValue)

	snapshot := input.Snapshots[name]
	for _, val := range snapshot {
		if val == nil {
			continue
		}

		if kv, ok := val.(KeyValue[TKey, TValue]); ok {
			ret[kv.Key] = kv.Value
		} else {
			return ret, fmt.Errorf("value of type %T not convertible to %T: %w", val, kv, ErrSnapshotTypeError)
		}
	}

	return ret, nil
}

func SnapshotToSingle[TValue any](input *go_hook.HookInput, name string) (TValue, error) {
	var value TValue

	snapshot := input.Snapshots[name]
	snapLen := len(snapshot)

	if snapLen != 1 {
		return value, fmt.Errorf("snapshot values count %d not equal one: %w", snapLen, ErrNoSnapshot)
	}

	snapValue := snapshot[0]
	value, ok := snapValue.(TValue)

	if !ok {
		return value, fmt.Errorf("value of type %T not convertible to %T: %w", snapValue, value, ErrSnapshotTypeError)
	}

	return value, nil
}

func SnapshotToList[TValue any](input *go_hook.HookInput, name string) ([]TValue, error) {
	snapshot := input.Snapshots[name]
	ret := make([]TValue, 0, len(snapshot))
	for _, snap := range snapshot {
		if snap == nil {
			continue
		}

		value, ok := snap.(TValue)

		if !ok {
			return ret, fmt.Errorf("value of type %T not convertible to %T: %w", snap, value, ErrSnapshotTypeError)
		}

		ret = append(ret, value)
	}

	return ret, nil
}
