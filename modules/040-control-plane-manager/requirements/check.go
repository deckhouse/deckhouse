/*
Copyright 2022 Flant JSC

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

package requirements

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
)

const (
	minK8sVersionRequirementKey = "controlPlaneManager:minUsedControlPlaneKubernetesVersion"

	// kubernetesVersionMigratedRequirementsKey is what a DeckhouseRelease declares under
	// requirements: (T+2). The matching SaveValue key lives in hooks — see
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

// checkKubernetesVersionMigrated blocks a DeckhouseRelease that requires the ClusterConfiguration
// kubernetesVersion field to have been moved to ModuleConfig control-plane-manager.
//
// requirementValue comes from the release; "true" (or anything other than ""/"false") means the
// gate is armed. Until T+2 no release declares this key, so the check is inert.
func checkKubernetesVersionMigrated(requirementValue string, getter requirements.ValueGetter) (bool, error) {
	requirementValue = strings.TrimSpace(requirementValue)
	if requirementValue == "" || requirementValue == "false" {
		return true, nil
	}

	migratedRaw, exists := getter.Get(hooks.KubernetesVersionMigratedRequirementKey)
	if !exists {
		// Hook has not published yet (startup race). Fail open — same posture as other boolean
		// gates; by T+2 the value will have been published for two minors.
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
		"kubernetesVersion is still pinned in ClusterConfiguration and has not been migrated " +
			"to ModuleConfig control-plane-manager.\n" +
			"Migrate it first:\n" +
			"  d8 k patch moduleconfig control-plane-manager --type merge -p \"$(cat <<EOF\n" +
			"{\"spec\": {\"version\": 3, \"settings\": {\"kubernetesVersion\": \"$(d8 k -n kube-system get secret d8-cluster-configuration -o jsonpath='{.data.cluster-configuration\\.yaml}' | base64 -d | grep kubernetesVersion | awk '{print $2}')\"}}}\n" +
			"EOF\n" +
			")\"\n" +
			"Then remove kubernetesVersion from ClusterConfiguration via `d8 system edit cluster-configuration`.",
	)
}
