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

package istio_versions

import "encoding/json"

type IstioVersionsMap map[string]IstioVersionInfo

type IstioVersionInfo struct {
	FullVersion string `json:"fullVersion"`
	Revision    string `json:"revision"`
	ImageSuffix string `json:"imageSuffix"`
	IsReady     bool   `json:"isReady"`
}

func (vm IstioVersionsMap) GetVersionByRevision(rev string) string {
	for ver, istioVerInfo := range vm {
		if istioVerInfo.Revision == rev {
			return ver
		}
	}
	return ""
}

func (vm IstioVersionsMap) GetVersionByFullVersion(fullVer string) string {
	for ver, istioVerInfo := range vm {
		if istioVerInfo.FullVersion == fullVer {
			return ver
		}
	}
	return ""
}

func (vm IstioVersionsMap) IsFullVersionReady(fullVer string) bool {
	for _, istioVerInfo := range vm {
		if istioVerInfo.FullVersion == fullVer {
			return istioVerInfo.IsReady
		}
	}
	return false
}

func (vm IstioVersionsMap) IsRevisionSupported(rev string) bool {
	for _, istioVerInfo := range vm {
		if istioVerInfo.Revision == rev {
			return true
		}
	}
	return false
}

func (vm IstioVersionsMap) SetRevisionStatus(rev string, isReady bool) {
	for ver, istioVerInfo := range vm {
		if istioVerInfo.Revision == rev {
			istioVerInfo.IsReady = isReady
			vm[ver] = istioVerInfo
		}
	}
}

func (vm IstioVersionsMap) GetFullVersionByRevision(rev string) string {
	for _, istioVerInfo := range vm {
		if istioVerInfo.Revision == rev {
			return istioVerInfo.FullVersion
		}
	}
	return ""
}

func (vm IstioVersionsMap) GetAllVersions() []string {
	versions := make([]string, len(vm))
	for ver := range vm {
		versions = append(versions, ver)
	}
	return versions
}

func VersionMapJSONToVersionMap(versionMapRaw string) IstioVersionsMap {
	versionMap := make(IstioVersionsMap)
	_ = json.Unmarshal([]byte(versionMapRaw), &versionMap)
	return versionMap
}
