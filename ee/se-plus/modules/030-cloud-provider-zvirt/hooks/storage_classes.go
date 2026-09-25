/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	storagev1 "k8s.io/api/storage/v1"

	"github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal"
	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/hooks/storage_class"
)

const (
	// storageDomainParameter is the StorageClass parameter the templates render the zVirt storage
	// domain into. It is what lets the classes already in the cluster stand in for the discovery data
	// after a Deckhouse restart, when values patches are gone.
	storageDomainParameter = "storageDomainName"
)

// StorageClass is one StorageClass in the module values.
type StorageClass struct {
	Name                 string `json:"name"`
	StorageDomain        string `json:"storageDomain"`
	AllowVolumeExpansion bool   `json:"allowVolumeExpansion"`
}

func (sc StorageClass) GetName() string {
	return sc.Name
}

// The StorageClasses follow from the discovery data discover.go has already put into values, so
// this hook runs after it and never looks at the Secret itself.
var _ = storage_class.RegisterHook[StorageClass](
	storage_class.Config{
		Order:      30,
		ModuleName: internal.ModuleName,
	},
	storage_class.Append(collectStorageClasses),
	storage_class.StopIfEmpty[StorageClass](),
	storage_class.Exclude[StorageClass]("cloudProviderZvirt.storage.parameters.excludedStorageClasses"),
	storage_class.SortByName[StorageClass](),
	storage_class.Publish[StorageClass]("cloudProviderZvirt.internal.storageClasses"),
)

// collectStorageClasses turns the discovered storage domains into StorageClasses, falling back to
// the classes already in the cluster while the discovery data is not in values yet.
func collectStorageClasses(ctx context.Context, state *storage_class.State[StorageClass]) ([]StorageClass, error) {
	rawDiscoveryData, ok := state.Input.Values.GetOk("cloudProviderZvirt.internal.providerDiscoveryData")
	if !ok {
		return storage_class.FromExisting(storageClassFromExisting)(ctx, state)
	}

	var discoveryData cloudDataV1.ZvirtCloudProviderDiscoveryData
	if err := json.Unmarshal([]byte(rawDiscoveryData.Raw), &discoveryData); err != nil {
		return nil, fmt.Errorf("decode cloudProviderZvirt.internal.providerDiscoveryData: %w", err)
	}

	// A class that already exists keeps its allowVolumeExpansion: the discovery data does not
	// carry it, and recreating the class with the default would be a change nobody asked for.
	existingByName := make(map[string]storagev1.StorageClass, len(state.Actual))
	for _, storageClassObject := range state.Actual {
		existingByName[storageClassObject.Name] = storageClassObject
	}

	storageClasses := make([]StorageClass, 0, len(discoveryData.StorageDomains))
	for _, domain := range discoveryData.StorageDomains {
		if !domain.IsEnabled {
			continue
		}

		name := storage_class.NormalizeStorageClassName(domain.Name)

		allowVolumeExpansion := true
		if existingObject, found := existingByName[name]; found && existingObject.AllowVolumeExpansion != nil {
			allowVolumeExpansion = *existingObject.AllowVolumeExpansion
		}

		storageClasses = append(storageClasses, StorageClass{
			Name:                 name,
			StorageDomain:        domain.Name,
			AllowVolumeExpansion: allowVolumeExpansion,
		})
	}

	return storageClasses, nil
}

// storageClassFromExisting rebuilds a StorageClass entry from the object in the cluster.
func storageClassFromExisting(storageClassObject storagev1.StorageClass) StorageClass {
	allowVolumeExpansion := true
	if storageClassObject.AllowVolumeExpansion != nil {
		allowVolumeExpansion = *storageClassObject.AllowVolumeExpansion
	}

	return StorageClass{
		Name:                 storageClassObject.Name,
		StorageDomain:        storageClassObject.Parameters[storageDomainParameter],
		AllowVolumeExpansion: allowVolumeExpansion,
	}
}
