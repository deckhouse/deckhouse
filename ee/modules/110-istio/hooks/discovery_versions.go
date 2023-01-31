/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal/istio_versions"
)

const (
	versionTemplate     = "%s.%s"
	fullVersionTemplate = "%s.%s.%s"
	revisionTemplate    = "v%sx%s"
	imageSuffixTemplate = "V%sx%sx%s"
	imageRegex          = `pilotV(?P<major>\d+)x(?P<minor>\d+)x(?P<patch>\d+)` // regex https://regex101.com/r/ESilDG/1
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 0},
}, versionsDiscovery)

type IstioVersion struct {
	info    istio_versions.IstioVersionInfo
	version string
}

// pilotV1x22x33 --> { "fullVersion": "1.22.33", "revision": "v1x22", "imageSuffix": "V1x22x33", Version: "1.22"}
func imageToIstioVersion(img string) (*IstioVersion, error) {
	re := regexp.MustCompile(imageRegex)
	match := re.FindStringSubmatch(img)
	if len(match) != 4 { // img, major, minor, patch
		return nil, fmt.Errorf("can not parse image alias %s", img)
	}
	major := match[re.SubexpIndex("major")]
	minor := match[re.SubexpIndex("minor")]
	patch := match[re.SubexpIndex("patch")]
	return &IstioVersion{
		version: fmt.Sprintf(versionTemplate, major, minor),
		info: istio_versions.IstioVersionInfo{
			FullVersion: fmt.Sprintf(fullVersionTemplate, major, minor, patch),
			Revision:    fmt.Sprintf(revisionTemplate, major, minor),
			ImageSuffix: fmt.Sprintf(imageSuffixTemplate, major, minor, patch),
			IsReady:     false,
		},
	}, nil
}

func versionsDiscovery(input *go_hook.HookInput) error {
	versionMap := make(map[string]istio_versions.IstioVersionInfo, 0)
	for img := range input.Values.Get("global.modulesImages.tags.istio").Map() {
		ver, err := imageToIstioVersion(img)
		if err != nil {
			continue
		}
		versionMap[ver.version] = ver.info
	}
	fmt.Println(versionMap)
	input.Values.Set("istio.internal.versionMap", versionMap)
	return nil
}
