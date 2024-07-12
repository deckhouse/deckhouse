/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
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
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
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
					"module":   "cloud-provider-openstack",
				},
			},
		},
	},
}, handleCloudProviderDiscoveryDataSecret)

func applyCloudProviderDiscoveryDataSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret = &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubernetes object: %v", err)
	}

	return secret, nil
}

func applyStorageClassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var storageClass = &storage.StorageClass{}
	err := sdk.FromUnstructured(obj, storageClass)
	if err != nil {
		return nil, fmt.Errorf("failed to convert kubernetes object: %v", err)
	}

	return storageClass, nil
}

func handleCloudProviderDiscoveryDataSecret(input *go_hook.HookInput) error {
	if len(input.Snapshots["cloud_provider_discovery_data"]) == 0 {
		input.LogEntry.Warn("failed to find secret 'd8-cloud-provider-discovery-data' in namespace 'kube-system'")

		if len(input.Snapshots["storage_classes"]) == 0 {
			input.LogEntry.Warn("failed to find storage classes for 'cinder.csi.openstack.org' provisioner")

			return nil
		}

		storageClassesSnapshots := input.Snapshots["storage_classes"]

		storageClasses := make([]storageClass, 0, len(storageClassesSnapshots))

		for _, storageClassSnapshot := range storageClassesSnapshots {
			sc := storageClassSnapshot.(*storage.StorageClass)

			storageClasses = append(storageClasses, storageClass{
				Name: sc.Name,
				Type: sc.Parameters["type"],
			})
		}

		setStorageClassesValues(input.Values, storageClasses)

		return nil
	}

	secret := input.Snapshots["cloud_provider_discovery_data"][0].(*v1.Secret)

	discoveryDataJSON := secret.Data["discovery-data.json"]

	_, err := config.ValidateDiscoveryData(&discoveryDataJSON, []string{"/deckhouse/ee/candi/cloud-providers/openstack/openapi"})
	if err != nil {
		return fmt.Errorf("failed to validate 'discovery-data.json' from 'd8-cloud-provider-discovery-data' secret: %v", err)
	}

	var discoveryData v1alpha1.OpenStackCloudProviderDiscoveryData
	err = json.Unmarshal(discoveryDataJSON, &discoveryData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'discovery-data.json' from 'd8-cloud-provider-discovery-data' secret: %v", err)
	}

	input.Values.Set("cloudProviderOpenstack.internal.discoveryData", discoveryData)

	handleDiscoveryDataVolumeTypes(input, discoveryData.VolumeTypes)

	return nil
}

func handleDiscoveryDataVolumeTypes(input *go_hook.HookInput, volumeTypes []v1alpha1.OpenStackCloudProviderDiscoveryDataVolumeType) {
	volumeTypesMap := make(map[string]string, len(volumeTypes))

	for _, volumeType := range volumeTypes {
		volumeTypesMap[getStorageClassName(volumeType.Name)] = volumeType.Name
	}

	excludes, ok := input.Values.GetOk("cloudProviderOpenstack.storageClass.exclude")
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
	for name, typ := range volumeTypesMap {
		sc := storageClass{
			Type: typ,
			Name: name,
		}
		storageClasses = append(storageClasses, sc)
	}

	sort.SliceStable(storageClasses, func(i, j int) bool {
		return storageClasses[i].Name < storageClasses[j].Name
	})

	setStorageClassesValues(input.Values, storageClasses)
}

func setStorageClassesValues(values *go_hook.PatchableValues, storageClasses []storageClass) {
	values.Set("cloudProviderOpenstack.internal.storageClasses", storageClasses)

	def, ok := values.GetOk("cloudProviderOpenstack.storageClass.default")
	if ok {
		values.Set("cloudProviderOpenstack.internal.defaultStorageClass", def.String())
	} else {
		values.Remove("cloudProviderOpenstack.internal.defaultStorageClass")
	}
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
	Name string `json:"name"`
	Type string `json:"type"`
}
