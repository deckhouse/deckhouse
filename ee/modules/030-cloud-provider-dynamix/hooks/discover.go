/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
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

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

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
					"module":   "cloud-provider-dynamix",
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
	if len(input.Snapshots.Get("cloud_provider_discovery_data")) == 0 {
		input.Logger.Warn("failed to find secret 'd8-cloud-provider-discovery-data' in namespace 'kube-system'")

		if len(input.Snapshots.Get("storage_classes")) == 0 {
			input.Logger.Warn("failed to find storage classes for dynamix provisioner")

			return nil
		}

		storageClassesSnapshots := input.Snapshots.Get("storage_classes")

		storageClasses := make([]storageClass, 0, len(storageClassesSnapshots))

		for sc, err := range sdkobjectpatch.SnapshotIter[storage.StorageClass](storageClassesSnapshots) {
			if err != nil {
				return fmt.Errorf("failed to iterate over storage classes: %v", err)
			}

			allowVolumeExpansion := true
			if sc.AllowVolumeExpansion != nil {
				allowVolumeExpansion = *sc.AllowVolumeExpansion
			}

			storageClasses = append(storageClasses, storageClass{
				Name:                 sc.Name,
				StorageEndpoint:      sc.Parameters["storageEndpoint"],
				Pool:                 sc.Parameters["pool"],
				AllowVolumeExpansion: allowVolumeExpansion,
			})
		}

		setStorageClassesValues(input, storageClasses)

		return nil
	}

	secret := new(v1.Secret)

	snaps := input.Snapshots.Get("cloud_provider_discovery_data")
	if len(snaps) == 0 {
		return fmt.Errorf("cloud_provider_discovery_data snapshot is empty")
	}

	err := snaps[0].UnmarshalTo(secret)
	if err != nil {
		return fmt.Errorf("failed to unmarshal secret: %w", err)
	}

	discoveryDataJSON := secret.Data["discovery-data.json"]

	_, err = config.ValidateDiscoveryData(&discoveryDataJSON, []string{"/deckhouse/ee/candi/cloud-providers/dynamix/openapi"})
	if err != nil {
		return fmt.Errorf("failed to validate 'discovery-data.json' from 'd8-cloud-provider-discovery-data' secret: %w", err)
	}

	var discoveryData cloudDataV1.DynamixCloudProviderDiscoveryData
	err = json.Unmarshal(discoveryDataJSON, &discoveryData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'discovery-data.json' from 'd8-cloud-provider-discovery-data' secret: %v", err)
	}

	input.Values.Set("cloudProviderDynamix.internal.providerDiscoveryData", discoveryData)

	handleDiscoveryDataVolumeTypes(input, discoveryData.StorageEndpoints)

	return nil
}

func handleDiscoveryDataVolumeTypes(input *go_hook.HookInput, volumeTypes []cloudDataV1.DynamixStorageEndpoint) {
	volumeTypesMap := make(map[string]storageClass, len(volumeTypes))

	for _, volumeType := range volumeTypes {
		if !volumeType.IsEnabled {
			continue
		}

		if len(volumeType.Pools) == 0 {
			continue
		}

		volumeTypesMap[getStorageClassName(volumeType.Name)] = storageClass{
			Name:                 getStorageClassName(volumeType.Name),
			StorageEndpoint:      volumeType.Name,
			Pool:                 volumeType.Pools[0],
			AllowVolumeExpansion: true,
		}
	}

	excludes, ok := input.Values.GetOk("cloudProviderDynamix.storageClass.exclude")
	if ok {
		for _, esc := range excludes.Array() {
			rg := regexp.MustCompile("^(" + esc.String() + ")$")
			for name := range volumeTypesMap {
				if rg.MatchString(name) {
					delete(volumeTypesMap, name)
				}
			}
		}
	}

	storageClasses := make([]storageClass, 0, len(volumeTypes))
	for name, sp := range volumeTypesMap {
		sc := storageClass{
			StorageEndpoint:      sp.StorageEndpoint,
			Pool:                 sp.Pool,
			Name:                 name,
			AllowVolumeExpansion: sp.AllowVolumeExpansion,
		}
		storageClasses = append(storageClasses, sc)
	}

	sort.SliceStable(storageClasses, func(i, j int) bool {
		return storageClasses[i].Name < storageClasses[j].Name
	})

	setStorageClassesValues(input, storageClasses)
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

func setStorageClassesValues(input *go_hook.HookInput, storageClasses []storageClass) {
	input.Values.Set("cloudProviderDynamix.internal.storageClasses", storageClasses)
}

type storageClass struct {
	Name                 string `json:"name"`
	StorageEndpoint      string `json:"storageEndpoint"`
	Pool                 string `json:"pool"`
	AllowVolumeExpansion bool   `json:"allowVolumeExpansion"`
}
