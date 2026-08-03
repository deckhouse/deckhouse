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

	"control-plane-manager/internal/controllers/update-observer/pkg/version"
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

// Available returns the versions the cluster is allowed to move to, published as
// status.availableVersions in the d8-cluster-kubernetes ConfigMap.
//
// The result is not only informational: the ModuleConfig admission webhook in deckhouse-controller
// rejects a kubernetesVersion that is not a member of this list
// (deckhouse-controller/pkg/apis/deckhouse.io/validation/validate_control_plane_manager.go).
// Changing the formula therefore changes what users are allowed to set — keep that webhook in mind,
// and note that it also enforces a maxUsed floor of its own, so the two must not contradict.
func (s VersionSettings) Available(maxUsedVersion string) []string {
	for i, v := range s.Supported {
		if v == maxUsedVersion {
			// available versions from (maxUsed - 1) to newest
			return s.Supported[max(i-1, 0):]
		}
	}

	// maxUsedVersion is not in the supported list. Normally that means it is older than everything
	// this release ships (the cluster started long ago and the version was dropped since), and the
	// whole list is an upgrade relative to it, so returning it unfiltered is safe.
	//
	// It can also mean the opposite — maxUsed is *newer* than anything supported, e.g. after a
	// Deckhouse downgrade or an edition switch. Then this list no longer encodes "no more than one
	// minor below maxUsed", and membership alone would permit a deep downgrade. That case is caught
	// by the maxUsed floor check in the admission webhook referenced above, which runs in addition
	// to membership rather than instead of it.
	return s.Supported
}
