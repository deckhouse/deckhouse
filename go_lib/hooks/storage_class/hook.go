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
	"fmt"
	"log/slog"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/regexpset"
)

type Collector[T any] func(input *go_hook.HookInput, existing []storagev1.StorageClass) ([]T, error)
type SkipPredictor func(input *go_hook.HookInput) bool
type DefaultPredictor[T any] func(T) bool
type PrunePredictor[T any] func(actual storagev1.StorageClass, desired *T) bool

type Config[T any] struct {
	Order           float64
	ModuleName      string
	ModuleValuesKey string

	ExcludeStorageClassesValuesPath string
	DefaultStorageClassValuesPath   string
	StorageClassesValuesPath        string

	NameOfFunc    func(T) string
	SkipIfFunc    SkipPredictor
	IsDefaultFunc DefaultPredictor[T]
	PruneIfFunc   PrunePredictor[T]
	CollectFunc   Collector[T]
}

func RegisterHook[T any](cfg Config[T]) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: cfg.Order},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "storage_classes",
				ApiVersion: "storage.k8s.io/v1",
				Kind:       "StorageClass",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"heritage": "deckhouse",
						"module":   cfg.ModuleName,
					},
				},
				FilterFunc: filterStorageClassResult,
			},
		},
	}, handleStorageClassesFunc(cfg))
}

func filterStorageClassResult(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	storageClass := &storagev1.StorageClass{}
	if err := sdk.FromUnstructured(obj, storageClass); err != nil {
		return nil, fmt.Errorf("convert StorageClass: %w", err)
	}

	return storageClass, nil
}

func handleStorageClassesFunc[T any](cfg Config[T]) func(context.Context, *go_hook.HookInput) error {
	return func(_ context.Context, input *go_hook.HookInput) error {
		if cfg.SkipIfFunc != nil && cfg.SkipIfFunc(input) {
			return nil
		}

		actualStorageClasses, err := getStorageClassesFromSnapshot(input)
		if err != nil {
			return err
		}

		desiredStorageClasses, err := cfg.CollectFunc(input, actualStorageClasses)
		if err != nil {
			return err
		}

		if len(desiredStorageClasses) == 0 {
			return nil
		}

		desiredStorageClasses, err = excludeStorageClasses(cfg, input, desiredStorageClasses)
		if err != nil {
			return err
		}

		sort.SliceStable(desiredStorageClasses, func(i, j int) bool {
			return cfg.NameOfFunc(desiredStorageClasses[i]) < cfg.NameOfFunc(desiredStorageClasses[j])
		})

		input.Logger.Info(
			"Publishing storage classes",
			slog.String("module", cfg.ModuleName),
			slog.Any("storage_classes", desiredStorageClasses),
		)

		input.Values.Set(cfg.StorageClassesValuesPath, desiredStorageClasses)

		setDefaultStorageClass(cfg, input, desiredStorageClasses)

		pruneStorageClasses(cfg, input, desiredStorageClasses, actualStorageClasses)

		return nil
	}
}

func setDefaultStorageClass[T any](cfg Config[T], input *go_hook.HookInput, desired []T) {
	if cfg.IsDefaultFunc == nil {
		return
	}

	for _, class := range desired {
		if !cfg.IsDefaultFunc(class) {
			continue
		}

		name := cfg.NameOfFunc(class)
		input.Values.Set(cfg.DefaultStorageClassValuesPath, name)
		input.Logger.Info("Discovered default storage class", slog.String("storage_class", name))

		return
	}

	input.Logger.Info("No default storage class found")
	input.Values.Remove(cfg.DefaultStorageClassValuesPath)
}

func pruneStorageClasses[T any](
	cfg Config[T],
	input *go_hook.HookInput,
	desired []T,
	actual []storagev1.StorageClass,
) {
	if cfg.PruneIfFunc == nil {
		return
	}

	desiredByName := make(map[string]T, len(desired))
	for _, class := range desired {
		name := cfg.NameOfFunc(class)
		desiredByName[name] = class
	}

	for _, storageClass := range actual {
		var wanted *T
		if class, found := desiredByName[storageClass.Name]; found {
			wanted = &class
		}

		if !cfg.PruneIfFunc(storageClass, wanted) {
			continue
		}

		input.Logger.Info("Deleting storage class", slog.String("storage_class", storageClass.Name))
		input.PatchCollector.Delete("storage.k8s.io/v1", "StorageClass", "", storageClass.Name)
	}
}

func newExcludePatterns(patterns []string) (regexpset.RegExpSet, error) {
	anchored := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		anchored = append(anchored, "^("+pattern+")$")
	}

	set, err := regexpset.New(anchored...)
	if err != nil {
		return nil, err
	}

	return set, nil
}

func getExcludesFromValues(values sdkpkg.PatchableValuesCollector, excludedStorageClassesPath string) (regexpset.RegExpSet, error) {
	rawPatterns := values.Get(excludedStorageClassesPath).Array()
	patterns := make([]string, 0, len(rawPatterns))
	for _, pattern := range rawPatterns {
		patterns = append(patterns, pattern.String())
	}

	excludes, err := newExcludePatterns(patterns)
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", excludedStorageClassesPath, err)
	}

	return excludes, nil
}

func excludeStorageClasses[T any](
	cfg Config[T],
	input *go_hook.HookInput,
	classes []T,
) ([]T, error) {
	if cfg.ExcludeStorageClassesValuesPath == "" {
		return classes, nil
	}

	filtered := make([]T, 0, len(classes))

	excludePatterns, err := getExcludesFromValues(input.Values, cfg.ExcludeStorageClassesValuesPath)
	if err != nil {
		return nil, err
	}

	for _, class := range classes {
		name := cfg.NameOfFunc(class)
		if excludePatterns.Match(name) {
			input.Logger.Info(
				"Excluding storage class because it matches",
				slog.String("storage_class", name),
			)

			continue
		}

		filtered = append(filtered, class)
	}

	return filtered, nil
}

func getStorageClassesFromSnapshot(input *go_hook.HookInput) ([]storagev1.StorageClass, error) {
	storageClasses, err := sdkobjectpatch.UnmarshalToStruct[storagev1.StorageClass](input.Snapshots, "storage_classes")
	if err != nil {
		return nil, fmt.Errorf("unmarshal storage_classes snapshot: %w", err)
	}

	return storageClasses, nil
}
