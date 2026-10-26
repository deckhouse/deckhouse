/*
Copyright 2021 Flant JSC

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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/hooks/storage_class"
	"github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal"
)

type StorageClass struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	BlockSize string `json:"blockSize,omitempty"`
}

func (sc StorageClass) GetName() string {
	return sc.Name
}

var defaultStorageClasses = []StorageClass{
	{
		Name: "network-hdd",
		Type: "network-hdd",
	},
	{
		Name: "network-ssd",
		Type: "network-ssd",
	},
	{
		Name: "network-ssd-nonreplicated",
		Type: "network-ssd-nonreplicated",
	},
	{
		Name: "network-ssd-io-m3",
		Type: "network-ssd-io-m3",
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "storage_classes",
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "StorageClass",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"heritage": "deckhouse",
					"module":   internal.ModuleName,
				},
			},
			FilterFunc: storage_class.ApplyFilter,
		},
	},
}, handleStorageClasses)

// handleStorageClasses publishes the default disk types, with provisionedStorageClasses adding classes
// or overriding a default one by its exact name, and excludedStorageClasses applied after that, so it
// filters the provisioned classes too. StorageClass parameters are immutable, so a class whose
// parameters were changed in the module configuration is deleted and recreated by the templates.
func handleStorageClasses(_ context.Context, input *go_hook.HookInput) error {
	var provisioned []StorageClass
	if raw, ok := input.Values.GetOk("cloudProviderYandex.storage.parameters.provisionedStorageClasses"); ok {
		if err := json.Unmarshal([]byte(raw.Raw), &provisioned); err != nil {
			return fmt.Errorf("decode cloudProviderYandex.storage.parameters.provisionedStorageClasses: %w", err)
		}
	}

	desired := storage_class.MergeOverridingByName(defaultStorageClasses, provisioned)

	var excludedPatterns []string
	for _, pattern := range input.Values.Get("cloudProviderYandex.storage.parameters.excludedStorageClasses").Array() {
		excludedPatterns = append(excludedPatterns, pattern.String())
	}

	desired, err := storage_class.ExcludeByAnchoredPatterns(desired, excludedPatterns)
	if err != nil {
		return fmt.Errorf("compile cloudProviderYandex.storage.parameters.excludedStorageClasses: %w", err)
	}

	storage_class.SortByName(desired)

	input.Values.Set("cloudProviderYandex.internal.storageClasses", desired)

	objects, err := sdkobjectpatch.UnmarshalToStruct[storagev1.StorageClass](input.Snapshots, "storage_classes")
	if err != nil {
		return fmt.Errorf("unmarshal storage_classes snapshot: %w", err)
	}

	storage_class.DeleteChangedObjects(input, objects, desired, storageClassFromObject)

	return nil
}

// storageClassFromObject reads a rendered StorageClass back into the form the module publishes.
func storageClassFromObject(object storagev1.StorageClass) StorageClass {
	return StorageClass{
		Name:      object.Name,
		Type:      object.Parameters["typeID"],
		BlockSize: object.Parameters["blockSize"],
	}
}
