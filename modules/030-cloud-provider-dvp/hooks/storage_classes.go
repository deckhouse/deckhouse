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
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

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

func (sc StorageClass) GetName() string {
	return sc.Name
}

// The StorageClasses follow from the discovery data discover.go has already put into values, so
// this hook runs after it and never looks at the Secret itself.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 40},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "storage_classes",
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "StorageClass",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"heritage": "deckhouse",
					"module":   dvpModuleName,
				},
			},
			FilterFunc: storage_class.ApplyFilter,
		},
	},
}, handleStorageClasses)

func handleStorageClasses(_ context.Context, input *go_hook.HookInput) error {
	// On fresh install without a ModuleConfig, nodes/provider are absent; any Values.Set
	// triggers full-object schema validation which rejects the patch.
	if _, ok := input.Values.GetOk("cloudProviderDvp.provider"); !ok {
		input.Logger.Warn("cloudProviderDvp.provider not set, skipping storage classes")

		return nil
	}

	objects, err := sdkobjectpatch.UnmarshalToStruct[storagev1.StorageClass](input.Snapshots, "storage_classes")
	if err != nil {
		return fmt.Errorf("unmarshal storage_classes snapshot: %w", err)
	}

	storage_class.DeleteObjectsIf(input, objects, isObjectToDelete)

	var desired []StorageClass
	if rawDiscoveryData, ok := input.Values.GetOk("cloudProviderDvp.internal.providerDiscoveryData"); ok {
		var discoveryData cloudDataV1.DVPCloudProviderDiscoveryData
		if err := json.Unmarshal([]byte(rawDiscoveryData.Raw), &discoveryData); err != nil {
			return fmt.Errorf("decode cloudProviderDvp.internal.providerDiscoveryData: %w", err)
		}

		desired = storageClassesFromDiscoveryData(discoveryData, objects)
	} else {
		// Values patches do not survive a Deckhouse restart, so until discover.go puts the discovery
		// data back, the StorageClasses the module has created are the only source of truth left.
		if len(objects) == 0 {
			input.Logger.Warn("No discovery data and no storage classes of the module, skipping storage classes")

			return nil
		}

		desired = storageClassesFromObjects(objects)
	}

	var excludedPatterns []string
	for _, pattern := range input.Values.Get("cloudProviderDvp.storage.parameters.excludedStorageClasses").Array() {
		excludedPatterns = append(excludedPatterns, pattern.String())
	}

	desired, err = storage_class.ExcludeByAnchoredPatterns(desired, excludedPatterns)
	if err != nil {
		return fmt.Errorf("compile cloudProviderDvp.storage.parameters.excludedStorageClasses: %w", err)
	}

	storage_class.SortByName(desired)

	input.Values.Set("cloudProviderDvp.internal.storageClasses", desired)

	defaultName, found := defaultStorageClassName(desired)
	if !found {
		input.Logger.Info("No default storage class found in parent DVP cluster")
		input.Values.Remove("cloudProviderDvp.internal.defaultStorageClass")

		return nil
	}

	input.Logger.Info("Discovered default storage class from DVP cloud provider", slog.String("storage_class", defaultName))
	input.Values.Set("cloudProviderDvp.internal.defaultStorageClass", defaultName)

	return nil
}

// isObjectToDelete reports whether a StorageClass of the module has to be deleted so that the templates
// recreate it: volumeBindingMode is immutable and must be WaitForFirstConsumer.
func isObjectToDelete(object storagev1.StorageClass) bool {
	return object.VolumeBindingMode == nil || *object.VolumeBindingMode != DefaultVolumeBindingMode
}

// storageClassesFromDiscoveryData merges the StorageClasses discovered in the parent DVP cluster with the
// ones the module has already created. A class the parent cluster no longer reports is kept as it is in
// the cluster: the module created it, and dropping it here would take away storage that may still be in use.
func storageClassesFromDiscoveryData(discoveryData cloudDataV1.DVPCloudProviderDiscoveryData, objects []storagev1.StorageClass) []StorageClass {
	discovered := make(map[string]cloudDataV1.DVPStorageClass, len(discoveryData.StorageClassList))
	for _, class := range discoveryData.StorageClassList {
		if !class.IsEnabled {
			continue
		}

		discovered[storage_class.NormalizeName(class.Name)] = class
	}

	storageClasses := make([]StorageClass, 0, len(discovered)+len(objects))
	for _, object := range objects {
		if _, found := discovered[object.Name]; found {
			continue
		}

		storageClasses = append(storageClasses, storageClassFromObject(object))
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

	return storageClasses
}

func storageClassesFromObjects(objects []storagev1.StorageClass) []StorageClass {
	storageClasses := make([]StorageClass, 0, len(objects))
	for _, object := range objects {
		storageClasses = append(storageClasses, storageClassFromObject(object))
	}

	return storageClasses
}

func storageClassFromObject(object storagev1.StorageClass) StorageClass {
	reclaimPolicy := corev1.PersistentVolumeReclaimDelete
	if object.ReclaimPolicy != nil {
		reclaimPolicy = *object.ReclaimPolicy
	}

	allowVolumeExpansion := false
	if object.AllowVolumeExpansion != nil {
		allowVolumeExpansion = *object.AllowVolumeExpansion
	}

	isDefault := false
	if object.Annotations != nil {
		if val, ok := object.Annotations[StableDefaultAnnotation]; ok && strings.ToLower(val) == "true" {
			isDefault = true
		} else if val, ok := object.Annotations[BetaDefaultAnnotation]; ok && strings.ToLower(val) == "true" {
			isDefault = true
		}
	}

	return StorageClass{
		Name:                 object.Name,
		DVPStorageClass:      object.Parameters["dvpStorageClass"],
		VolumeBindingMode:    string(DefaultVolumeBindingMode),
		ReclaimPolicy:        string(reclaimPolicy),
		AllowVolumeExpansion: allowVolumeExpansion,
		IsDefault:            isDefault,
	}
}

func defaultStorageClassName(desired []StorageClass) (string, bool) {
	for _, class := range desired {
		if class.IsDefault {
			return class.Name, true
		}
	}

	return "", false
}
