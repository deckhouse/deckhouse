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
	"log/slog"

	storagev1 "k8s.io/api/storage/v1"
)

// PrunePredictor reports whether an existing StorageClass has to be deleted. desired is the entry with
// the same name, nil when the module no longer wants the class.
type PrunePredictor[T Namer] func(actual storagev1.StorageClass, desired *T) bool

// Publish writes the desired set to the values path, an empty set included.
func Publish[T Namer](path string) Step[T] {
	return func(_ context.Context, state *State[T]) error {
		state.Input.Logger.Info(
			"Publishing storage classes",
			slog.String("module", state.ModuleName),
			slog.Any("storage_classes", state.Desired),
		)

		desired := state.Desired
		if desired == nil {
			desired = []T{}
		}

		state.Input.Values.Set(path, desired)

		return nil
	}
}

// PublishDefault writes the selected default StorageClass to the values path, or removes the path when
// nothing is selected.
func PublishDefault[T Namer](isDefault func(T) bool, path string) Step[T] {
	return func(_ context.Context, state *State[T]) error {
		for _, class := range state.Desired {
			if !isDefault(class) {
				continue
			}

			state.Input.Logger.Info("Discovered default storage class", slog.String("storage_class", class.GetName()))
			state.Input.Values.Set(path, class.GetName())
			return nil
		}

		state.Input.Logger.Info("No default storage class found")
		state.Input.Values.Remove(path)
		return nil
	}
}

// Prune deletes the existing StorageClasses the predicate selects, so that the templates recreate
// the ones still wanted: most StorageClass fields are immutable.
func Prune[T Namer](shouldPrune PrunePredictor[T]) Step[T] {
	return func(_ context.Context, state *State[T]) error {
		desiredByName := make(map[string]T, len(state.Desired))
		for _, class := range state.Desired {
			desiredByName[class.GetName()] = class
		}

		for _, storageClass := range state.Actual {
			var wanted *T
			if class, found := desiredByName[storageClass.Name]; found {
				wanted = &class
			}

			if !shouldPrune(storageClass, wanted) {
				continue
			}

			state.Input.Logger.Info("Deleting storage class", slog.String("storage_class", storageClass.Name))
			state.Input.PatchCollector.Delete("storage.k8s.io/v1", "StorageClass", "", storageClass.Name)
		}

		return nil
	}
}

// PruneModified deletes an existing StorageClass that is still wanted but whose parameters, as convert
// reads them back into an entry, differ from the desired ones. A class that is no longer wanted is left
// alone.
func PruneModified[T interface {
	Namer
	comparable
}](convert func(storagev1.StorageClass) T) Step[T] {
	return Prune(ModifiedPredictor(convert))
}

// ModifiedPredictor is the predicate of PruneModified, for modules that combine it with their own.
func ModifiedPredictor[T interface {
	Namer
	comparable
}](convert func(storagev1.StorageClass) T) PrunePredictor[T] {
	return func(actual storagev1.StorageClass, desired *T) bool {
		if desired == nil {
			return false
		}

		return convert(actual) != *desired
	}
}
