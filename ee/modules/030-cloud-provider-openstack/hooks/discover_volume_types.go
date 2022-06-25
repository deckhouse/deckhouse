/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "discover_volume_types",
			Crontab: "45 * * * *",
		},
	},
}, handleDiscoverVolumeTypes)

type storageClass struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func handleDiscoverVolumeTypes(input *go_hook.HookInput) error {
	err := initOpenstackEnvs(input)
	if err != nil {
		return err
	}

	var openstackVolumeTypes []string
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		openstackVolumeTypes = []string{"__DEFAULT__", "some-foo", "bar", "other-bar", "SSD R1"}
	} else {
		openstackVolumeTypes, err = getVolumeTypesArray()
		if err != nil {
			return err
		}
	}

	storageClassesMap := make(map[string]string, len(openstackVolumeTypes))

	for _, vt := range openstackVolumeTypes {
		storageClassesMap[strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(vt, "_", ""), " ", "-"))] = vt
	}

	excludes, ok := input.Values.GetOk("cloudProviderOpenstack.storageClass.exclude")
	if ok {
		for _, esc := range excludes.Array() {
			rg := regexp.MustCompile("^(" + esc.String() + ")$")
			for name := range storageClassesMap {
				if rg.MatchString(name) {
					delete(storageClassesMap, name)
				}
			}
		}
	}

	storageClasses := make([]storageClass, 0, len(storageClassesMap))
	for name, typ := range storageClassesMap {
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
