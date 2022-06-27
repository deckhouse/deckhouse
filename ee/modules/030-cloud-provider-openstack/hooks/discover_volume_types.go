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
	"unicode"

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
		openstackVolumeTypes = []string{"__DEFAULT__", "some-foo", "bar", "other-bar", "SSD R1", "-Xx$&? -foo", " YY fast SSD -foo."}
	} else {
		openstackVolumeTypes, err = getVolumeTypesArray()
		if err != nil {
			return err
		}
	}

	storageClassesMap := make(map[string]string, len(openstackVolumeTypes))

	for _, vt := range openstackVolumeTypes {
		storageClassesMap[getStorageClassName(vt)] = vt
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

// Get StorageClass name from Volume type name to match Kubernetes restrictions from https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
func getStorageClassName(value string) string {
	mapFn := func(r rune) rune {
		if r >= 'a' && r <= 'z' ||
			r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' ||
			r == '-' || r == '.' {
			return unicode.ToLower(r)
		}
		return rune(-1)
	}

	// a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.'
	value = strings.Map(mapFn, value)

	// must start and end with an alphanumeric character
	return strings.Trim(value, "-.")
}
