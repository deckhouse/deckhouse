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
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	minUbuntuVersionValuesKey = "nodeManager:nodesMinimalOSVersionUbuntu"
	requirementsKey           = "nodesMinimalOSVersionUbuntu"
	containerdRequirementsKey = "containerdOnAllNodes"
	hasNodesWithDocker        = "nodeManager:hasNodesWithDocker"
)

func init() {
	checkRequirementFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		desiredVersion, err := semver.NewVersion(requirementValue)
		if err != nil {
			return false, err
		}
		currentVersionRaw, exists := getter.Get(minUbuntuVersionValuesKey)
		if !exists {
			return true, nil
		}
		currentVersion, err := semver.NewVersion(currentVersionRaw.(string))
		if err != nil {
			return false, err
		}

		if currentVersion.LessThan(desiredVersion) {
			return false, errors.New("minimal node Ubuntu OS version is lower then required")
		}

		return true, nil
	}

	checkContainerdRequirementFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		requirementValue = strings.TrimSpace(requirementValue)
		if requirementValue == "false" || requirementValue == "" {
			return true, nil
		}

		hasDocker, exists := getter.Get(hasNodesWithDocker)
		if !exists {
			return true, nil
		}

		if hasDocker.(bool) {
			return false, errors.New("has nodes with Docker CRI or defaultCRI is Docker")
		}

		return true, nil
	}
	requirements.RegisterCheck(requirementsKey, checkRequirementFunc)
	requirements.RegisterCheck(containerdRequirementsKey, checkContainerdRequirementFunc)
}
