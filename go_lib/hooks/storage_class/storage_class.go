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
	"fmt"
	"log/slog"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/regexpset"
)

// Namer is a StorageClass entry of a module: anything that knows the name of the StorageClass it stands for.
type Namer interface {
	GetName() string
}

// ApplyFilter is a FilterFunc for a StorageClass binding.
func ApplyFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	storageClass := &storagev1.StorageClass{}
	if err := sdk.FromUnstructured(obj, storageClass); err != nil {
		return nil, fmt.Errorf("convert StorageClass: %w", err)
	}

	return storageClass, nil
}

// ExcludeByAnchoredPatterns returns the entries whose name matches none of the regular expressions.
// Every pattern is anchored as ^(pattern)$, so a plain name stays an exact name: `fast` excludes `fast`
// and not `ultra-fast-ssd`. The result is always a new slice.
func ExcludeByAnchoredPatterns[T Namer](classes []T, patterns []string) ([]T, error) {
	anchored := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		anchored = append(anchored, "^("+pattern+")$")
	}

	excludes, err := regexpset.New(anchored...)
	if err != nil {
		return nil, err
	}

	filtered := make([]T, 0, len(classes))
	for _, class := range classes {
		if !excludes.Match(class.GetName()) {
			filtered = append(filtered, class)
		}
	}

	return filtered, nil
}

// MergeOverridingByName returns base with every entry replaced, in its place, by the override with
// exactly the same name; the remaining overrides are appended in their order. A name overrides lists
// more than once yields a single entry, the last one. Neither input is changed.
//
// Names are compared exactly, not as patterns: provider class names are often prefixes of each other
// (network-ssd and network-ssd-nonreplicated).
func MergeOverridingByName[T Namer](base, overrides []T) []T {
	pending := make(map[string]T, len(overrides))
	for _, class := range overrides {
		pending[class.GetName()] = class
	}

	merged := make([]T, 0, len(base)+len(overrides))
	for _, class := range base {
		name := class.GetName()
		if override, found := pending[name]; found {
			merged = append(merged, override)
			delete(pending, name)

			continue
		}

		merged = append(merged, class)
	}

	for _, class := range overrides {
		name := class.GetName()
		if override, found := pending[name]; found {
			merged = append(merged, override)
			delete(pending, name)
		}
	}

	return merged
}

// SortByName orders the entries by name in place, keeping the relative order of equal names.
func SortByName[T Namer](classes []T) {
	sort.SliceStable(classes, func(i, j int) bool {
		return classes[i].GetName() < classes[j].GetName()
	})
}

// DeleteChangedObjects deletes every StorageClass object that is still desired but whose parameters, as
// objectToDesired reads them back into an entry, differ from the desired entry with the same name. The
// templates then recreate it: StorageClass parameters are immutable. An object that is no longer desired
// is left alone.
func DeleteChangedObjects[T interface {
	Namer
	comparable
}](input *go_hook.HookInput, objects []storagev1.StorageClass, desired []T, objectToDesired func(storagev1.StorageClass) T) {
	desiredByName := make(map[string]T, len(desired))
	for _, class := range desired {
		desiredByName[class.GetName()] = class
	}

	isChangedFunc := func(object storagev1.StorageClass) bool {
		class, found := desiredByName[object.Name]
		return found && objectToDesired(object) != class
	}

	DeleteObjectsIf(input, objects, isChangedFunc)
}

// DeleteObjectsIf deletes every StorageClass object shouldDelete selects, so that the
// templates recreate the ones still wanted: most StorageClass fields are immutable.
func DeleteObjectsIf(input *go_hook.HookInput, objects []storagev1.StorageClass, shouldDeleteFunc func(storagev1.StorageClass) bool) {
	for _, object := range objects {
		if !shouldDeleteFunc(object) {
			continue
		}

		input.Logger.Info("Deleting storage class", slog.String("storage_class", object.Name))
		input.PatchCollector.Delete("storage.k8s.io/v1", "StorageClass", "", object.Name)
	}
}
