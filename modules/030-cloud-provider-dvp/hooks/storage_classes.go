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

package hooks

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"

	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/hooks/storage_class"
)

const (
	StableDefaultAnnotation  = "storageclass.kubernetes.io/is-default-class"
	BetaDefaultAnnotation    = "storageclass.beta.kubernetes.io/is-default-class"
	DefaultVolumeBindingMode = storagev1.VolumeBindingWaitForFirstConsumer
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

// The StorageClasses follow from the discovery data discover.go has already put into values, so
// this hook runs after it and never looks at the Secret itself.
var _ = storage_class.RegisterHook(
	storage_class.Config[StorageClass]{
		ModuleName:      dvpModuleName,
		ModuleValuesKey: "cloudProviderDvp",
		Order:           40,

		ExcludeStorageClassesValuesPath: "cloudProviderDvp.storage.parameters.excludedStorageClasses",
		StorageClassesValuesPath:        "cloudProviderDvp.internal.storageClasses",
		DefaultStorageClassValuesPath:   "cloudProviderDvp.internal.defaultStorageClass",

		NameOfFunc:    func(class StorageClass) string { return class.Name },
		IsDefaultFunc: func(class StorageClass) bool { return class.IsDefault },
		SkipIfFunc:    shouldStorageClassesHandlingBeSkipped,
		CollectFunc:   collectStorageClasses,
		PruneIfFunc:   shouldStorageClassBePruned,
	},
)

func shouldStorageClassesHandlingBeSkipped(input *go_hook.HookInput) bool {
	if _, ok := input.Values.GetOk("cloudProviderDvp.provider"); !ok {
		input.Logger.Warn("cloudProviderDvp.provider not set, skipping storage classes")
		return true
	}

	return false
}

// shouldStorageClassBePruned reports whether the StorageClass has to be deleted so that the
// templates recreate it.
func shouldStorageClassBePruned(actual storagev1.StorageClass, desired *StorageClass) bool {
	if actual.VolumeBindingMode == nil || *actual.VolumeBindingMode != DefaultVolumeBindingMode {
		return true
	}

	return storage_class.NewModifiedPrunePredictor(convertStorageClass)(actual, desired)
}

// collectStorageClasses merges the StorageClasses discovered in the parent DVP cluster with the
// ones the module has already created, falling back to the latter while the discovery data is not
// in values yet.
func collectStorageClasses(input *go_hook.HookInput, existing []storagev1.StorageClass) ([]StorageClass, error) {
	rawDiscoveryData, ok := input.Values.GetOk("cloudProviderDvp.internal.providerDiscoveryData")
	if !ok {
		return storage_class.NewConvertingCollector(convertStorageClass)(input, existing)
	}

	var discoveryData cloudDataV1.DVPCloudProviderDiscoveryData
	if err := json.Unmarshal([]byte(rawDiscoveryData.Raw), &discoveryData); err != nil {
		return nil, fmt.Errorf("decode cloudProviderDvp.internal.providerDiscoveryData: %w", err)
	}

	discovered := make(map[string]cloudDataV1.DVPStorageClass, len(discoveryData.StorageClassList))
	for _, class := range discoveryData.StorageClassList {
		if !class.IsEnabled {
			continue
		}

		discovered[storage_class.NormalizeStorageClassName(class.Name)] = class
	}

	// A class the parent cluster no longer reports is kept as it is in the cluster: the module
	// created it, and dropping it here would take away storage that may still be in use.
	storageClasses := make([]StorageClass, 0, len(discovered)+len(existing))
	for _, storageClassObject := range existing {
		if _, found := discovered[storageClassObject.Name]; found {
			continue
		}

		storageClasses = append(storageClasses, convertStorageClass(storageClassObject))
	}

	for name, class := range discovered {
		storageClasses = append(storageClasses, StorageClass{
			Name:                 name,
			DVPStorageClass:      class.Name,
			VolumeBindingMode:    string(DefaultVolumeBindingMode),
			ReclaimPolicy:        class.ReclaimPolicy,
			AllowVolumeExpansion: class.AllowVolumeExpansion,
			IsDefault:            class.IsDefault,
		})
	}

	return storageClasses, nil
}

func convertStorageClass(sc storagev1.StorageClass) StorageClass {
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
