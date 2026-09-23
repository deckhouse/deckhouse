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
	storagev1 "k8s.io/api/storage/v1"

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

// The disk types are known up front. provisionedStorageClasses adds classes or overrides a default
// one by its exact name, and excludedStorageClasses applies after that, so it filters the
// provisioned classes too. StorageClass parameters are immutable, so a class whose parameters were
// changed in the module configuration is deleted and recreated by the templates.
var _ = storage_class.RegisterHook(
	storage_class.Config{
		Order:      20,
		ModuleName: internal.ModuleName,
	},
	storage_class.Append(storage_class.Static(defaultStorageClasses...)),
	storage_class.OverrideByName(storage_class.FromValues[StorageClass]("cloudProviderYandex.storage.parameters.provisionedStorageClasses")),
	storage_class.Exclude[StorageClass]("cloudProviderYandex.storage.parameters.excludedStorageClasses"),
	storage_class.SortByName[StorageClass](),
	storage_class.Publish[StorageClass]("cloudProviderYandex.internal.storageClasses"),
	storage_class.PruneModified(convertStorageClass),
)

// convertStorageClass reads a rendered StorageClass back into the form the module publishes.
func convertStorageClass(sc storagev1.StorageClass) StorageClass {
	return StorageClass{
		Name:      sc.Name,
		Type:      sc.Parameters["typeID"],
		BlockSize: sc.Parameters["blockSize"],
	}
}
