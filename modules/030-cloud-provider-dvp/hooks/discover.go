/*
Copyright 2025 Flant JSC

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
					"module":   "cloud-provider-dvp",
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

func handleCloudProviderDiscoveryDataSecret(input *go_hook.HookInput) error {
	if len(input.Snapshots["cloud_provider_discovery_data"]) == 0 {
		input.Logger.Warn("failed to find secret 'd8-cloud-provider-discovery-data' in namespace 'kube-system'")

		if len(input.Snapshots["storage_classes"]) == 0 {
			input.Logger.Warn("failed to find storage classes for dvp provisioner")

			return nil
		}

		storageClassesSnapshots := input.Snapshots["storage_classes"]

		storageClasses := make([]storageClass, 0, len(storageClassesSnapshots))

		for _, storageClassSnapshot := range storageClassesSnapshots {
			sc := storageClassSnapshot.(*storage.StorageClass)

			storageClasses = append(storageClasses, storageClass{
				Name:            sc.Name,
				DVPStorageClass: sc.Parameters["dvpStorageClass"],
			})
		}
		input.Logger.Info("Found DVP storage classes using StorageClass snapshots: %v", storageClasses)

		setStorageClassesValues(input, storageClasses)

		return nil
	}

	secret := input.Snapshots["cloud_provider_discovery_data"][0].(*v1.Secret)

	discoveryDataJSON := secret.Data["discovery-data.json"]

	_, err := config.ValidateDiscoveryData(&discoveryDataJSON, []string{"/deckhouse/candi/cloud-providers/dvp/openapi"})
	if err != nil {
		return fmt.Errorf("failed to validate 'discovery-data.json' from 'd8-cloud-provider-discovery-data' secret: %v", err)
	}

	var discoveryData cloudDataV1.DVPCloudProviderDiscoveryData
	err = json.Unmarshal(discoveryDataJSON, &discoveryData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'discovery-data.json' from 'd8-cloud-provider-discovery-data' secret: %v", err)
	}

	input.Values.Set("cloudProviderDvp.internal.providerDiscoveryData", discoveryData)

	handleDiscoveryDataVolumeTypes(input, discoveryData.StorageClassList)

	return nil
}

func handleDiscoveryDataVolumeTypes(
	input *go_hook.HookInput,
	dvpStorageClassList []cloudDataV1.DVPStorageClass,
) {
	dvpstorageClass := make(map[string]cloudDataV1.DVPStorageClass, len(dvpStorageClassList))

	for _, sc := range dvpStorageClassList {
		if !sc.IsEnabled {
			continue
		}

		dvpstorageClass[getStorageClassName(sc.Name)] = sc
	}

	classExcludes, ok := input.Values.GetOk("cloudProviderDvp.storageClass.exclude")
	if ok {
		for _, esc := range classExcludes.Array() {
			rg := regexp.MustCompile("^(" + esc.String() + ")$")
			for class := range dvpstorageClass {
				if rg.MatchString(class) {
					delete(dvpstorageClass, class)
				}
			}
		}
	}

	storageClassSnapshots := make(map[string]*storage.StorageClass)
	for _, snapshot := range input.Snapshots["storage_classes"] {
		s := snapshot.(*storage.StorageClass)
		storageClassSnapshots[s.Name] = s
	}

	storageClasses := make([]storageClass, 0, len(dvpStorageClassList))
	for name, domain := range dvpstorageClass {
		sc := storageClass{
			Name:            name,
			DVPStorageClass: domain.Name,
		}
		storageClasses = append(storageClasses, sc)
	}

	sort.SliceStable(storageClasses, func(i, j int) bool {
		return storageClasses[i].Name < storageClasses[j].Name
	})

	input.Logger.Info("Found DVP storage classes using StorageClass snapshots, StorageClasses from discovery data: %v", storageClasses)

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
	input.Values.Set("cloudProviderDvp.internal.storageClasses", storageClasses)
}

type storageClass struct {
	Name            string `json:"name"`
	DVPStorageClass string `json:"dvpStorageClass"`
}
