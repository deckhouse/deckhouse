/*
Copyright 2023 Flant JSC

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
	"fmt"
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib/istio_versions"
)

const (
	versionTemplate     = "%s.%s"
	fullVersionTemplate = "%s.%s.%s"
	revisionTemplate    = "v%sx%s"
	imageSuffixTemplate = "V%sx%sx%s"
	imageRegex          = `pilotV(?P<major>\d+)x(?P<minor>\d+)x(?P<patch>\d+)` // regex https://regex101.com/r/ESilDG/1
	versionMapPath      = "istio.internal.versionMap"
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
	for img := range input.Values.Get("global.modulesImages.digests.istio").Map() {
		ver, err := imageToIstioVersion(img)
		if err != nil {
			continue
		}
		versionMap[ver.version] = ver.info
	}
	input.Values.Set(versionMapPath, versionMap)
	return nil
}
