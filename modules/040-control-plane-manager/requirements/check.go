/*
Copyright 2022 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    10|Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package requirements

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
)

const (
	minK8sVersionRequirementKey = "controlPlaneManager:minUsedControlPlaneKubernetesVersion"

	// kubernetesVersionMigratedRequirementsKey is what a DeckhouseRelease declares under
	// requirements: (T+1 declare). The matching SaveValue key lives in hooks — see
	// hooks.KubernetesVersionMigratedRequirementKey.
	kubernetesVersionMigratedRequirementsKey = "kubernetesVersionMigrated"
)

func init() {
	f := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		desiredVersion, err := semver.NewVersion(requirementValue)
		if err != nil {
			return false, err
		}
		currentVersionStr, exists := getter.Get(minK8sVersionRequirementKey)
		if !exists {
			return false, errors.New("\nminUsedControlPlaneKubernetesVersion\n is not set")
		}
		currentVersion, err := semver.NewVersion(currentVersionStr.(string))
		if err != nil {
			return false, err
		}

		if currentVersion.LessThan(desiredVersion) {
			return false, errors.New("current kubernetes version is lower than required")
		}

		return true, nil
	}

	requirements.RegisterCheck("k8s", f)
	requirements.RegisterCheck(kubernetesVersionMigratedRequirementsKey, checkKubernetesVersionMigrated)
}

// checkKubernetesVersionMigrated blocks a DeckhouseRelease that requires ModuleConfig
// control-plane-manager to have kubernetesVersion set.
//
// requirementValue comes from the release. No release declares this key yet, so the check is inert.
//
// TODO(kubernetesVersion-deprecation): T+1 verify — stand-check with a test DeckhouseRelease
// before arming the real gate.
//
// TODO(kubernetesVersion-deprecation): T+1 declare — add kubernetesVersionMigrated: "true" to
// DeckhouseRelease requirements in the release AFTER ASAP migrate + CC strip; never co-ship with ASAP.
func checkKubernetesVersionMigrated(requirementValue string, getter requirements.ValueGetter) (bool, error) {
	requirementValue = strings.TrimSpace(requirementValue)
	if requirementValue == "" {
		return true, nil
	}
	gateEnabled, err := strconv.ParseBool(requirementValue)
	if err != nil {
		return false, fmt.Errorf("invalid %s requirement value %q: %w", kubernetesVersionMigratedRequirementsKey, requirementValue, err)
	}
	if !gateEnabled {
		return true, nil
	}

	migratedRaw, exists := getter.Get(hooks.KubernetesVersionMigratedRequirementKey)
	if !exists {
		// Hook has not published yet (startup race). Fail open — same posture as other boolean gates.
		return true, nil
	}

	migrated, ok := migratedRaw.(bool)
	if !ok {
		return false, fmt.Errorf("invalid %s value type", hooks.KubernetesVersionMigratedRequirementKey)
	}

	if migrated {
		return true, nil
	}

	return false, errors.New(
		"kubernetesVersion is not set in ModuleConfig control-plane-manager.\n" +
			"Set it before upgrading (pin a version, or Default to track the Deckhouse default):\n" +
			"  d8 k patch moduleconfig control-plane-manager --type merge -p " +
			`'{"spec":{"version":3,"settings":{"kubernetesVersion":"Default"}}}'` + "\n" +
			"If ClusterConfiguration still pins a version, copy that value into ModuleConfig instead of Default.",
	)
}
