/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"strings"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 0},
}, versionsDiscovery)

type istioVersionInfo struct {
	FullVersion string `json:"fullVersion"`
	Revision    string `json:"revision"`
	ImageSuffix string `json:"imageSuffix"`
}

// pilotV1x22x33 --> 1.22, { "fullVersion": "1.22.33", "revision": "1x22", "imageSuffix": "V1x22x33" }
func imageToIstioVersionInfo(img string) (string, istioVersionInfo) {
	imageSuffix := img[strings.Index(img, "V"):]                                   // V1x22x33
	revision := strings.ToLower(imageSuffix[:strings.LastIndex(imageSuffix, "x")]) // v1x22
	fullVersion := strings.ReplaceAll(imageSuffix[1:], "x", ".")                   // 1.22.33
	version := fullVersion[:strings.LastIndex(fullVersion, ".")]                   // 1.22
	return version, istioVersionInfo{
		FullVersion: fullVersion,
		Revision:    revision,
		ImageSuffix: imageSuffix,
	}
}

func versionsDiscovery(input *go_hook.HookInput) error {
	versionMap := make(map[string]istioVersionInfo, 0)
	for img, _ := range input.Values.Get("global.modulesImages.tags.istio").Map() {
		if strings.HasPrefix(img, "pilot") {
			version, info := imageToIstioVersionInfo(img)
			versionMap[version] = info
		}
	}
	input.Values.Set("istio.internal.versionMap", versionMap)
	return nil
}
