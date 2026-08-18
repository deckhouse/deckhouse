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

// Every field comes from the container environment, rendered by templates/daemonset.yaml; nothing is
// resolved here. A malformed value is an error, not a default: a guessed version would be written
// into the ConfigMap and read back as if declared.
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

// Not only informational: the ModuleConfig admission webhook rejects a kubernetesVersion outside this
// list, so changing the formula changes what operators may set.
func (s VersionSettings) Available(maxUsedVersion string) []string {
	for i, v := range s.Supported {
		if v == maxUsedVersion {
			// available versions from (maxUsed - 1) to newest
			return s.Supported[max(i-1, 0):]
		}
	}

	// Not in the supported list: usually older than everything this release ships. It can also be
	// newer, after a Deckhouse downgrade — the admission floor check catches that case.
	return s.Supported
}
