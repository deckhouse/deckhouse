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

// LoadConfigurationFromEnv builds the declared configuration this controller writes into data.spec
// of the cluster ConfigMap. Every field comes from the container environment, rendered by
// modules/040-control-plane-manager/templates/daemonset.yaml out of values the hooks publish —
// this controller resolves nothing itself and reads nothing back from data.spec to build it.
//
// Every field is mandatory and every malformed value is an error rather than a silent default.
// A default here would be written straight into the ConfigMap and from there read by
// node-controller, the release requirements check and two admission webhooks: a guessed version
// would look exactly like a declared one. Failing instead leaves the previous, correct content in
// place and requeues.
func LoadConfigurationFromEnv() (*Configuration, error) {
	desiredVersion, err := requiredVersionFromEnv(desiredKubernetesVersionEnv)
	if err != nil {
		return nil, err
	}

	maxUsedVersion, err := requiredVersionFromEnv(maxUsedKubernetesVersionEnv)
	if err != nil {
		return nil, err
	}

	updateMode := UpdateMode(strings.TrimSpace(os.Getenv(kubernetesUpdateModeEnv)))
	switch updateMode {
	case UpdateModeAutomatic, UpdateModeManual:
	default:
		return nil, fmt.Errorf("invalid %s: %q, want %q or %q",
			kubernetesUpdateModeEnv, updateMode, UpdateModeAutomatic, UpdateModeManual)
	}

	return &Configuration{
		DesiredVersion: desiredVersion,
		UpdateMode:     updateMode,
		MaxUsedVersion: maxUsedVersion,
	}, nil
}

func requiredVersionFromEnv(name string) (string, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return "", fmt.Errorf("%s is not set", name)
	}

	normalized, err := version.Normalize(raw)
	if err != nil {
		return "", fmt.Errorf("invalid %s %q: %w", name, raw, err)
	}

	return normalized, nil
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
