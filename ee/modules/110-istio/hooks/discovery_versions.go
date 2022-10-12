/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"regexp"
)

const (
	versionTemplate     = "%s.%s"
	fullVersionTemplate = "%s.%s.%s"
	revisionTemplate    = "v%sx%s"
	imageSuffixTemplate = "V%sx%sx%s"
	imageRegex          = `pilotV(?P<major>\d+)x(?P<minor>\d+)x(?P<patch>\d+)`
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 0},
}, versionsDiscovery)

type istioVersionInfo struct {
	FullVersion string `json:"fullVersion"`
	Revision    string `json:"revision"`
	ImageSuffix string `json:"imageSuffix"`
	version     string
}

// pilotV1x22x33 --> { "fullVersion": "1.22.33", "revision": "1x22", "imageSuffix": "V1x22x33", version: "1.22"}
func imageToIstioVersionInfo(img string) (*istioVersionInfo, error) {
	re, err := regexp.Compile(imageRegex)
	if err != nil {
		return nil, err
	}
	match := re.FindStringSubmatch(img)
	if len(match) != 4 { // img, major, minor, patch
		return nil, fmt.Errorf("can not parse image alias %s", img)
	}
	major := match[re.SubexpIndex("major")]
	minor := match[re.SubexpIndex("minor")]
	patch := match[re.SubexpIndex("patch")]
	return &istioVersionInfo{
		version:     fmt.Sprintf(versionTemplate, major, minor),
		FullVersion: fmt.Sprintf(fullVersionTemplate, major, minor, patch),
		Revision:    fmt.Sprintf(revisionTemplate, major, minor),
		ImageSuffix: fmt.Sprintf(imageSuffixTemplate, major, minor, patch),
	}, nil
}

func versionsDiscovery(input *go_hook.HookInput) error {
	versionMap := make(map[string]istioVersionInfo, 0)
	for img, _ := range input.Values.Get("global.modulesImages.tags.istio").Map() {
		info, err := imageToIstioVersionInfo(img)
		if err != nil {
			continue
		}
		versionMap[info.version] = *info

	}
	input.Values.Set("istio.internal.versionMap", versionMap)
	return nil
}
