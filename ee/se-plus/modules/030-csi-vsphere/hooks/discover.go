/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	objectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cloud_provider_discovery_data",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cloud-provider-discovery-data"},
			},
			FilterFunc: applyCloudProviderDiscoveryDataSecretFilter,
		},
		{
			Name:       "storage_classes",
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "StorageClass",
			FilterFunc: applyStorageClassFilter,
			LabelSelector: &meta.LabelSelector{
				MatchLabels: map[string]string{
					"heritage": "deckhouse",
					"module":   "csi-vsphere",
				},
			},
		},
	},
}, handleCloudProviderDiscoveryDataSecret)

func applyCloudProviderDiscoveryDataSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubernetes object: %v", err)
	}

	return secret, nil
}

func applyStorageClassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	storageClass := &storage.StorageClass{}
	err := sdk.FromUnstructured(obj, storageClass)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubernetes object: %v", err)
	}

	return storageClass, nil
}

func handleCloudProviderDiscoveryDataSecret(_ context.Context, input *go_hook.HookInput) error {
	ddSnaps := input.Snapshots.Get("cloud_provider_discovery_data")
	if len(ddSnaps) == 0 {
		input.Logger.Warn("failed to find secret 'd8-cloud-provider-discovery-data' in namespace 'kube-system'")

		scSnaps := input.Snapshots.Get("storage_classes")
		if len(scSnaps) == 0 {
			input.Logger.Warn("failed to find storage classes for vSphere provisioner")
			return nil
		}

		storageClasses := make([]storageClass, 0, len(scSnaps))

		for sc, err := range objectpatch.SnapshotIter[storage.StorageClass](scSnaps) {
			if err != nil {
				return fmt.Errorf("failed to iterate over storage classes: %v", err)
			}

			var zones []string
			for _, t := range sc.AllowedTopologies {
				for _, m := range t.MatchLabelExpressions {
					if m.Key == "failure-domain.beta.kubernetes.io/zone" {
						zones = append(zones, m.Values...)
					}
				}
			}
			slices.Sort(zones)
			zones = slices.Compact(zones)

			storageClasses = append(storageClasses, storageClass{
				Name:         sc.Name,
				Zones:        zones,
				DatastoreURL: sc.Parameters["DatastoreURL"],
			})
		}
		input.Logger.Info("found vSphere storage classes using storage_classes snapshots", slog.Any("storage_classes", storageClasses))
		input.Values.Set("csiVsphere.internal.storageClasses", storageClasses)
		return nil
	}

	secret := new(v1.Secret)
	err := ddSnaps[0].UnmarshalTo(secret)
	if err != nil {
		return fmt.Errorf("failed to unmarshal secret: %v", err)
	}
	discoveryDataJSON := secret.Data["discovery-data.json"]

	var discoveryData cloudDataV1.VsphereCloudDiscoveryData
	err = json.Unmarshal(discoveryDataJSON, &discoveryData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'discovery-data.json' from 'd8-cloud-provider-discovery-data' secret: %v", err)
	}

	input.Values.Set("csiVsphere.internal.providerDiscoveryData", discoveryData)

	if err := handleDiscoveryDataVolumeTypes(input, discoveryData.Datastores); err != nil {
		return err
	}

	return nil
}

func handleDiscoveryDataVolumeTypes(input *go_hook.HookInput, zonedDataStores []cloudDataV1.VsphereDatastore) error {
	storageClassStorageDomain := make(map[string]cloudDataV1.VsphereDatastore)

	for _, ds := range zonedDataStores {
		storageClassStorageDomain[getStorageClassName(ds.Name)] = ds
	}

	classExcludes, ok := input.Values.GetOk("csiVsphere.storageClass.exclude")
	if ok {
		for _, esc := range classExcludes.Array() {
			rg := regexp.MustCompile("^(" + esc.String() + ")$")
			for class := range storageClassStorageDomain {
				if rg.MatchString(class) {
					delete(storageClassStorageDomain, class)
				}
			}
		}
	}

	storageClassSnapshots := make(map[string]storage.StorageClass)
	sclasses, err := objectpatch.UnmarshalToStruct[storage.StorageClass](input.Snapshots, "storage_classes")
	if err != nil {
		return fmt.Errorf("failed to unmarshal storage_classes snapshot: %w", err)
	}

	for _, s := range sclasses {
		storageClassSnapshots[s.Name] = s
	}

	storageClasses := make([]storageClass, 0, len(zonedDataStores))
	for name, domain := range storageClassStorageDomain {
		sc := storageClass{
			Name:          name,
			Path:          domain.InventoryPath,
			Zones:         domain.Zones,
			DatastoreType: domain.DatastoreType,
			DatastoreURL:  domain.DatastoreURL,
		}
		storageClasses = append(storageClasses, sc)
	}

	sort.SliceStable(storageClasses, func(i, j int) bool {
		return storageClasses[i].Name < storageClasses[j].Name
	})

	input.Logger.Info("Found vSphere storage classes using cloud_provider_discovery_data", slog.Any("data", storageClasses))
	input.Values.Set("csiVsphere.internal.storageClasses", storageClasses)
	return nil
}

// Get StorageClass name from Volume type name to match Kubernetes restrictions from https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
func getStorageClassName(value string) string {
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

type storageClass struct {
	Name          string   `json:"name"`
	Path          string   `json:"path"`
	Zones         []string `json:"zones"`
	DatastoreType string   `json:"datastoreType"`
	DatastoreURL  string   `json:"datastoreURL"`
}
