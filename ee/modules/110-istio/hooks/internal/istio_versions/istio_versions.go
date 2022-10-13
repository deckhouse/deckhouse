/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package istio_versions

import "encoding/json"

type IstioVersionsMapType map[string]IstioVersionInfo

type IstioVersionInfo struct {
	FullVersion string `json:"fullVersion"`
	Revision    string `json:"revision"`
	ImageSuffix string `json:"imageSuffix"`
}

func (vm IstioVersionsMapType) GetVersionByRevision(rev string) string {
	for ver, istioVerInfo := range vm {
		if istioVerInfo.Revision == rev {
			return ver
		}
	}
	return ""
}

func (vm IstioVersionsMapType) GetFullVersionByRevision(rev string) string {
	for _, istioVerInfo := range vm {
		if istioVerInfo.Revision == rev {
			return istioVerInfo.FullVersion
		}
	}
	return ""
}

func (vm IstioVersionsMapType) GetAllVersions() []string {
	versions := make([]string, len(vm))
	for ver := range vm {
		versions = append(versions, ver)
	}
	return versions
}

func VersionMapStrToVersionMapType(versionMapRaw string) IstioVersionsMapType {
	versionMap := make(IstioVersionsMapType)
	json.Unmarshal([]byte(versionMapRaw), &versionMap)
	return versionMap
}
