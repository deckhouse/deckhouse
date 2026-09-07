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

package internal

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"unicode"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/regexpset"
)

const (
	// StorageClassesSnapshotName is the name of the snapshot with the StorageClasses created by the module.
	StorageClassesSnapshotName = "storage_classes"

	StableDefaultAnnotation  = "storageclass.kubernetes.io/is-default-class"
	BetaDefaultAnnotation    = "storageclass.beta.kubernetes.io/is-default-class"
	DefaultVolumeBindingMode = storagev1.VolumeBindingWaitForFirstConsumer

	storageClassesPath         = "cloudProviderDvp.internal.storageClasses"
	defaultStorageClassPath    = "cloudProviderDvp.internal.defaultStorageClass"
	excludedStorageClassesPath = "cloudProviderDvp.storage.parameters.excludedStorageClasses"
)

// StorageClass describes a StorageClass to create in the cluster.
type StorageClass struct {
	Name                 string `json:"name"`
	DVPStorageClass      string `json:"dvpStorageClass"`
	VolumeBindingMode    string `json:"volumeBindingMode"`
	ReclaimPolicy        string `json:"reclaimPolicy"`
	AllowVolumeExpansion bool   `json:"allowVolumeExpansion"`
	IsDefault            bool   `json:"isDefault"`
}

// ApplyStorageClassFilter is a FilterFunc for the StorageClassesSnapshotName binding.
func ApplyStorageClassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	storageClass := &storagev1.StorageClass{}
	err := sdk.FromUnstructured(obj, storageClass)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubernetes object: %v", err)
	}

	return storageClass, nil
}

// HandleStorageClassesFromSnapshots keeps the StorageClasses already created by the module when
// the discovery data secret is missing. Values patches do not survive a Deckhouse restart, so the
// StorageClasses present in the cluster are the only source of truth left in that case.
func HandleStorageClassesFromSnapshots(input *go_hook.HookInput) error {
	snapshots := input.Snapshots.Get(StorageClassesSnapshotName)
	storageClasses := make([]StorageClass, 0, len(snapshots))

	for sc, err := range sdkobjectpatch.SnapshotIter[storagev1.StorageClass](snapshots) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshots: %v", StorageClassesSnapshotName, err)
		}

		deleteOldStorageClass(input, &sc)
		storageClasses = append(storageClasses, StorageClassToValue(&sc))
	}

	input.Logger.Info("Found DVP storage classes using StorageClass snapshots", slog.Any("storage_classes", storageClasses))

	return setStorageClassesValues(input, storageClasses)
}

// HandleStorageClassesFromDiscoveryData merges the StorageClasses discovered in the parent DVP
// cluster with the ones already created by the module.
func HandleStorageClassesFromDiscoveryData(
	input *go_hook.HookInput,
	dvpStorageClassList []cloudDataV1.DVPStorageClass,
) error {
	discovered := make(map[string]cloudDataV1.DVPStorageClass, len(dvpStorageClassList))

	for _, sc := range dvpStorageClassList {
		if !sc.IsEnabled {
			continue
		}

		discovered[GetStorageClassName(sc.Name)] = sc
	}

	storageClasses := make([]StorageClass, 0, len(dvpStorageClassList))
	for sc, err := range sdkobjectpatch.SnapshotIter[storagev1.StorageClass](input.Snapshots.Get(StorageClassesSnapshotName)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshots: %v", StorageClassesSnapshotName, err)
		}

		deleteOldStorageClass(input, &sc)

		if _, ok := discovered[sc.Name]; !ok {
			storageClasses = append(storageClasses, StorageClassToValue(&sc))
		}
	}

	for name, sc := range discovered {
		storageClasses = append(storageClasses, StorageClass{
			Name:                 name,
			DVPStorageClass:      sc.Name,
			VolumeBindingMode:    string(DefaultVolumeBindingMode),
			ReclaimPolicy:        sc.ReclaimPolicy,
			AllowVolumeExpansion: sc.AllowVolumeExpansion,
			IsDefault:            sc.IsDefault,
		})
	}

	input.Logger.Info(
		"Found DVP storage classes using StorageClass snapshots and discovery data",
		slog.Any("storage_classes", storageClasses),
	)

	return setStorageClassesValues(input, storageClasses)
}

// setStorageClassesValues is the single place where StorageClasses reach the module values, so
// every source of StorageClasses is filtered by `storage.parameters.excludedStorageClasses`.
func setStorageClassesValues(input *go_hook.HookInput, storageClasses []StorageClass) error {
	excludes, err := regexpset.NewFromValues(input.Values, excludedStorageClassesPath)
	if err != nil {
		return fmt.Errorf("failed to compile '%s': %v", excludedStorageClassesPath, err)
	}

	filtered := make([]StorageClass, 0, len(storageClasses))
	for _, sc := range storageClasses {
		if excludes.Match(sc.Name) {
			input.Logger.Info(
				"Excluding storage class because it matches storage.parameters.excludedStorageClasses",
				slog.String("storage_class", sc.Name),
			)

			continue
		}

		filtered = append(filtered, sc)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})

	input.Values.Set(storageClassesPath, filtered)

	// Find and set default StorageClass in module internal values
	var defaultSC string
	for _, sc := range filtered {
		if sc.IsDefault {
			defaultSC = sc.Name
			break
		}
	}

	if defaultSC == "" {
		input.Logger.Info("No default storage class found in parent DVP cluster")
		input.Values.Remove(defaultStorageClassPath)

		return nil
	}

	input.Values.Set(defaultStorageClassPath, defaultSC)
	input.Logger.Info("Discovered default storage class from DVP cloud provider", slog.String("storage_class", defaultSC))

	return nil
}

// GetStorageClassName converts a DVP StorageClass name into a Kubernetes object name that satisfies
// the restrictions from https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
func GetStorageClassName(value string) string {
	mapFn := func(r rune) rune {
		if r >= 'a' && r <= 'z' ||
			r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' ||
			r == '-' || r == '.' {
			return unicode.ToLower(r)
		} else if r == ' ' {
			return '-'
		}
		return rune(-1)
	}

	// a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.'
	value = strings.Map(mapFn, value)

	// must start and end with an alphanumeric character
	return strings.Trim(value, "-.")
}

// StorageClassToValue converts a StorageClass object into its module values representation.
func StorageClassToValue(sc *storagev1.StorageClass) StorageClass {
	reclaimPolicy := corev1.PersistentVolumeReclaimDelete
	if sc.ReclaimPolicy != nil {
		reclaimPolicy = *sc.ReclaimPolicy
	}

	allowVolumeExpansion := false
	if sc.AllowVolumeExpansion != nil {
		allowVolumeExpansion = *sc.AllowVolumeExpansion
	}

	isDefault := false
	if sc.Annotations != nil {
		if val, ok := sc.Annotations[StableDefaultAnnotation]; ok && strings.ToLower(val) == "true" {
			isDefault = true
		} else if val, ok := sc.Annotations[BetaDefaultAnnotation]; ok && strings.ToLower(val) == "true" {
			isDefault = true
		}
	}

	return StorageClass{
		Name:                 sc.Name,
		DVPStorageClass:      sc.Parameters["dvpStorageClass"],
		VolumeBindingMode:    string(DefaultVolumeBindingMode),
		ReclaimPolicy:        string(reclaimPolicy),
		AllowVolumeExpansion: allowVolumeExpansion,
		IsDefault:            isDefault,
	}
}

func deleteOldStorageClass(input *go_hook.HookInput, sc *storagev1.StorageClass) {
	if sc.VolumeBindingMode != nil && *sc.VolumeBindingMode == DefaultVolumeBindingMode {
		return
	}

	input.Logger.Info(
		"Deleting storage class because volumeBindingMode must be WaitForFirstConsumer.",
		slog.String("storage_class", sc.Name),
	)
	input.PatchCollector.Delete("storage.k8s.io/v1", "StorageClass", "", sc.Name)
}
