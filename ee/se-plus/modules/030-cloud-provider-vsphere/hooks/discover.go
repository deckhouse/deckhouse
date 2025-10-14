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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
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
					"module":   "cloud-provider-vsphere",
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
	sc := &storage.StorageClass{}
	err := sdk.FromUnstructured(obj, sc)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubernetes object: %v", err)
	}

	return sc, nil
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

		for snapshotSc, err := range objectpatch.SnapshotIter[storage.StorageClass](scSnaps) {
			if err != nil {
				return fmt.Errorf("failed to iterate over storage classes: %v", err)
			}

			var zones []string
			for _, t := range snapshotSc.AllowedTopologies {
				for _, m := range t.MatchLabelExpressions {
					if m.Key == "failure-domain.beta.kubernetes.io/zone" {
						zones = append(zones, m.Values...)
					}
				}
			}
			slices.Sort(zones)
			zones = slices.Compact(zones)

			sc := storageClass{
				Name:         snapshotSc.Name,
				Zones:        zones,
				DatastoreURL: snapshotSc.Parameters["DatastoreURL"],
			}

			if spName, found := snapshotSc.Parameters["StoragePolicyName"]; found {
				sc.StoragePolicyName = spName
			}

			storageClasses = append(storageClasses, sc)
		}
		input.Logger.Info("found vSphere storage classes using storage_classes snapshots", slog.Any("storage_classes", storageClasses))
		input.Values.Set("cloudProviderVsphere.internal.storageClasses", storageClasses)
		return nil
	}

	secret := new(v1.Secret)
	if err := ddSnaps[0].UnmarshalTo(secret); err != nil {
		return fmt.Errorf("failed to unmarshal secret: %v", err)
	}
	discoveryDataJSON := secret.Data["discovery-data.json"]

	if _, err := config.ValidateDiscoveryData(&discoveryDataJSON, []string{"/deckhouse/ee/se-plus/candi/cloud-providers/vsphere/openapi"}); err != nil {
		return fmt.Errorf("failed to validate 'discovery-data.json' from 'd8-cloud-provider-discovery-data' secret: %v", err)
	}

	var discoveryData cloudDataV1.VsphereCloudDiscoveryData
	if err := json.Unmarshal(discoveryDataJSON, &discoveryData); err != nil {
		return fmt.Errorf("failed to unmarshal 'discovery-data.json' from 'd8-cloud-provider-discovery-data' secret: %v", err)
	}

	input.Values.Set("cloudProviderVsphere.internal.providerDiscoveryData", discoveryData)
	handleDiscoveryDataVolumeTypes(input, discoveryData.Datastores, discoveryData.StoragePolicies)

	return nil
}

func handleDiscoveryDataVolumeTypes(input *go_hook.HookInput, zonedDataStores []cloudDataV1.VsphereDatastore, storagePolicies []cloudDataV1.VsphereStoragePolicy) {
	classExcludes := input.Values.Get("cloudProviderVsphere.storageClass.exclude")

	lenStorageClasses := len(zonedDataStores)
	if len(storagePolicies) > 0 {
		lenStorageClasses *= len(storagePolicies)
	}
	storageClasses := make([]storageClass, 0, lenStorageClasses)

	for _, ds := range zonedDataStores {
		var excluded bool
		for _, esc := range classExcludes.Array() {
			rg := regexp.MustCompile("^(" + esc.String() + ")$")
			if rg.MatchString(getStorageClassName(ds.Name)) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		storageClasses = append(storageClasses, storageClass{
			Name:          getStorageClassName(ds.Name),
			Path:          ds.InventoryPath,
			Zones:         ds.Zones,
			DatastoreType: ds.DatastoreType,
			DatastoreURL:  ds.DatastoreURL,
		})

		for _, sp := range storagePolicies {
			storageClasses = append(storageClasses, storageClass{
				Name:              getStorageClassName(fmt.Sprintf("%s-%s", ds.Name, sp.Name)),
				Path:              ds.InventoryPath,
				Zones:             ds.Zones,
				DatastoreType:     ds.DatastoreType,
				DatastoreURL:      ds.DatastoreURL,
				StoragePolicyName: sp.Name,
			})
		}
	}

	sort.SliceStable(storageClasses, func(i, j int) bool {
		return storageClasses[i].Name < storageClasses[j].Name
	})

	input.Logger.Info("Found vSphere storage classes using cloud_provider_discovery_data", slog.Any("data", storageClasses))
	input.Values.Set("cloudProviderVsphere.internal.storageClasses", storageClasses)
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
	// must start and end with an alphanumeric character
	return strings.Trim(strings.Map(mapFn, value), "-.")
}

type storageClass struct {
	Name              string   `json:"name"`
	Path              string   `json:"path"`
	Zones             []string `json:"zones"`
	DatastoreType     string   `json:"datastoreType"`
	DatastoreURL      string   `json:"datastoreURL"`
	StoragePolicyName string   `json:"storagePolicyName"`
}
