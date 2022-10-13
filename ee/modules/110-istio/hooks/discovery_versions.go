/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
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

type versionMapType map[string]istioVersion

type istioVersion struct {
	FullVersion string `json:"fullVersion"`
	Revision    string `json:"revision"`
	ImageSuffix string `json:"imageSuffix"`
	version     string
}

func (vm versionMapType) GetVersionByRevision(rev string) string {
	for ver, istioVerInfo := range vm {
		if istioVerInfo.Revision == rev {
			return ver
		}
	}
	return ""
}

func (vm versionMapType) GetFullVersionByRevision(rev string) string {
	for _, istioVerInfo := range vm {
		if istioVerInfo.Revision == rev {
			return istioVerInfo.FullVersion
		}
	}
	return ""
}

func (vm versionMapType) GetAllVersions() []string {
	versions := make([]string, len(vm))
	for ver := range vm {
		versions = append(versions, ver)
	}
	return versions
}

func versionMapStrToVersionMapType(versionMapRaw string) versionMapType {
	versionMap := make(versionMapType)
	json.Unmarshal([]byte(versionMapRaw), &versionMap)
	return versionMap
}

// pilotV1x22x33 --> { "fullVersion": "1.22.33", "revision": "v1x22", "imageSuffix": "V1x22x33", version: "1.22"}
func imageToIstioVersion(img string) (*istioVersion, error) {
	re := regexp.MustCompile(imageRegex)
	match := re.FindStringSubmatch(img)
	if len(match) != 4 { // img, major, minor, patch
		return nil, fmt.Errorf("can not parse image alias %s", img)
	}
	major := match[re.SubexpIndex("major")]
	minor := match[re.SubexpIndex("minor")]
	patch := match[re.SubexpIndex("patch")]
	return &istioVersion{
		version:     fmt.Sprintf(versionTemplate, major, minor),
		FullVersion: fmt.Sprintf(fullVersionTemplate, major, minor, patch),
		Revision:    fmt.Sprintf(revisionTemplate, major, minor),
		ImageSuffix: fmt.Sprintf(imageSuffixTemplate, major, minor, patch),
	}, nil
}

func versionsDiscovery(input *go_hook.HookInput) error {
	versionMap := make(map[string]istioVersion, 0)
	for img := range input.Values.Get("global.modulesImages.tags.istio").Map() {
		info, err := imageToIstioVersion(img)
		if err != nil {
			continue
		}
		versionMap[info.version] = *info
	}
	input.Values.Set("istio.internal.versionMap", versionMap)
	return nil
}
