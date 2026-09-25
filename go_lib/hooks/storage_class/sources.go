/*
Copyright 2026 Flant JSC

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

package storage_class

import (
	"context"
	"encoding/json"
	"fmt"

	storagev1 "k8s.io/api/storage/v1"
)

// Static yields a fixed set of entries — the model of the providers whose classes are disk types known
// up front rather than discovered from the cloud.
func Static[T Namer](classes ...T) Source[T] {
	return func(_ context.Context, _ *State[T]) ([]T, error) {
		return classes, nil
	}
}

// FromExisting rebuilds the entries from the StorageClasses the module has already created. It stands
// in for the discovery data while that is not in values yet, e.g. right after a Deckhouse restart.
func FromExisting[T Namer](convert func(storagev1.StorageClass) T) Source[T] {
	return func(_ context.Context, state *State[T]) ([]T, error) {
		classes := make([]T, 0, len(state.Actual))
		for _, storageClass := range state.Actual {
			classes = append(classes, convert(storageClass))
		}

		return classes, nil
	}
}

// FromValues decodes the array at the values path into entries, field by field through JSON, so the
// entry type has to carry the same JSON names as the values schema. A missing path yields nothing.
func FromValues[T Namer](path string) Source[T] {
	return func(_ context.Context, state *State[T]) ([]T, error) {
		raw, ok := state.Input.Values.GetOk(path)
		if !ok {
			return nil, nil
		}

		var classes []T
		if err := json.Unmarshal([]byte(raw.Raw), &classes); err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}

		return classes, nil
	}
}

// FirstNonEmpty yields the entries of the first source that produces any, trying them in order.
func FirstNonEmpty[T Namer](sources ...Source[T]) Source[T] {
	return func(ctx context.Context, state *State[T]) ([]T, error) {
		for _, source := range sources {
			classes, err := source(ctx, state)
			if err != nil {
				return nil, err
			}

			if len(classes) > 0 {
				return classes, nil
			}
		}

		return nil, nil
	}
}
