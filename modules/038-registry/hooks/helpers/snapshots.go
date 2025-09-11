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

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var (
	ErrNoSnapshot        = errors.New("no snapshot found or too many snapshots")
	ErrSnapshotTypeError = errors.New("snapshot cannot be converted to requested type")
)

func SnapshotToMap[TKey comparable, TValue any](input *go_hook.HookInput, name string) (map[TKey]TValue, error) {
	ret := make(map[TKey]TValue)

	snapshot := input.Snapshots.Get(name)
	for val, err := range sdkobjectpatch.SnapshotIter[KeyValue[TKey, TValue]](snapshot) {
		if err != nil {
			return ret, fmt.Errorf("value of type %T not convertible to KeyValue: %w", val, err)
		}

		ret[val.Key] = val.Value
	}

	return ret, nil
}

func SnapshotToSingle[TValue any](input *go_hook.HookInput, name string) (TValue, error) {
	var value TValue

	snapshot := input.Snapshots.Get(name)
	snapLen := len(snapshot)

	if snapLen != 1 {
		return value, fmt.Errorf("snapshot values count %d not equal one: %w", snapLen, ErrNoSnapshot)
	}

	snapValue := snapshot[0]
	err := snapValue.UnmarshalTo(&value)

	if err != nil {
		return value, fmt.Errorf("value of type %T not convertible to %T: %w", snapValue, value, ErrSnapshotTypeError)
	}

	return value, nil
}

func SnapshotToList[TValue any](input *go_hook.HookInput, name string) ([]TValue, error) {
	snapshot := input.Snapshots.Get(name)
	ret := make([]TValue, 0, len(snapshot))
	for snap, err := range sdkobjectpatch.SnapshotIter[TValue](snapshot) {
		if err != nil {
			return ret, fmt.Errorf("failed to convert snapshot value: %w", err)
		}

		ret = append(ret, snap)
	}

	return ret, nil
}
