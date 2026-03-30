/*
Copyright 2026 Flant JSC

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

package cluster

import (
	"fmt"
	"os"
	"strings"

	"update-observer/pkg/version"
)

type VersionSettings struct {
	Supported []string
	Automatic string
}

func LoadVersionSettingsFromEnv() (VersionSettings, error) {
	supportedVersionsEnv := os.Getenv(supportedKubernetesVersionsEnv)
	automaticVersion := os.Getenv(automaticKubernetesVersionEnv)
	if supportedVersionsEnv == "" || automaticVersion == "" {
		return VersionSettings{}, fmt.Errorf("%s or %s not found", supportedKubernetesVersionsEnv, automaticKubernetesVersionEnv)
	}

	var err error
	var nAutomaticVersion string

	if nAutomaticVersion, err = version.Normalize(automaticVersion); err != nil {
		return VersionSettings{}, err
	}

	supportedVersions := strings.Split(supportedVersionsEnv, ",")
	nSupportedVersions := make([]string, 0, len(supportedVersions))
	for _, v := range supportedVersions {
		if nV, err := version.Normalize(v); err != nil {
			return VersionSettings{}, err
		} else {
			nSupportedVersions = append(nSupportedVersions, nV)
		}
	}

	return VersionSettings{
		Supported: nSupportedVersions,
		Automatic: nAutomaticVersion,
	}, nil
}

func (s VersionSettings) Available(maxUsedVersion string) []string {
	for i, v := range s.Supported {
		if v == maxUsedVersion {
			// available versions from (maxUsed - 1) to newest
			return s.Supported[max(i-1, 0):]
		}
	}

	// maxVersion not found in supported list (shouldn't happen)
	return s.Supported
}
