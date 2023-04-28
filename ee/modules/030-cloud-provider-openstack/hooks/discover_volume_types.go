/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"regexp"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const volumeTypesCatalogSnapshot = "volume-types-catalog"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       volumeTypesCatalogSnapshot,
			ApiVersion: "deckhouse.io/v1",
			Kind:       "VolumeTypesCatalog",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-provider-openstack"},
				},
			},
			FilterFunc: applyFilter,
		},
	},
}, handleDiscoverVolumeTypes)

func applyFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var catalog VolumeTypesCatalog

	err := sdk.FromUnstructured(obj, &catalog)
	if err != nil {
		return nil, err
	}

	volumeTypes := make(map[string]string, len(catalog.VolumeTypes))

	for _, volumeType := range catalog.VolumeTypes {
		volumeTypes[volumeType.Name] = volumeType.Type
	}

	return volumeTypes, nil
}

type storageClass struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func handleDiscoverVolumeTypes(input *go_hook.HookInput) error {
	err := initOpenstackEnvs(input)
	if err != nil {
		return err
	}

	var volumeTypes map[string]string

	snapshot := input.Snapshots[volumeTypesCatalogSnapshot]

	if len(snapshot) > 0 {
		volumeTypes = snapshot[0].(map[string]string)
	} else {
		return nil
	}

	excludes, ok := input.Values.GetOk("cloudProviderOpenstack.storageClass.exclude")
	if ok {
		for _, esc := range excludes.Array() {
			rg := regexp.MustCompile("^(" + esc.String() + ")$")
			for name := range volumeTypes {
				if rg.MatchString(name) {
					delete(volumeTypes, name)
				}
			}
		}
	}

	storageClasses := make([]storageClass, 0, len(volumeTypes))
	for name, typ := range volumeTypes {
		sc := storageClass{
			Type: typ,
			Name: name,
		}
		storageClasses = append(storageClasses, sc)
	}

	sort.SliceStable(storageClasses, func(i, j int) bool {
		return storageClasses[i].Name < storageClasses[j].Name
	})

	input.Values.Set("cloudProviderOpenstack.internal.storageClasses", storageClasses)

	def, ok := input.Values.GetOk("cloudProviderOpenstack.storageClass.default")
	if ok {
		input.Values.Set("cloudProviderOpenstack.internal.defaultStorageClass", def.String())
	} else {
		input.Values.Remove("cloudProviderOpenstack.internal.defaultStorageClass")
	}

	return nil
}

type VolumeType struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Parameters map[string]any
}

type VolumeTypesCatalog struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	VolumeTypes []VolumeType `json:"volumeTypes"`
}
