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
)

const (
	minUbuntuVersionValuesKey           = "nodeManager:nodesMinimalOSVersionUbuntu"
	minDebianVersionValuesKey           = "nodeManager:nodesMinimalOSVersionDebian"
	requirementsUbuntuKey               = "nodesMinimalOSVersionUbuntu"
	requirementsDebianKey               = "nodesMinimalOSVersionDebian"
	unmetCloudConditionsKey             = "nodeManager:unmetCloudConditions"
	unmetCloudConditionsRequirementsKey = "unmetCloudConditions"
)

// normalizeUbuntuVersionForSemver converts Ubuntu version format to semver format: 20.04.3 -> 20.4.3, 20.04 -> 20.4.0
func normalizeUbuntuVersionForSemver(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return version
	}

	// Normalize major version
	major := strings.TrimLeft(parts[0], "0")
	if major == "" {
		major = "0"
	}

	// Normalize minor version
	minor := strings.TrimLeft(parts[1], "0")
	if minor == "" {
		minor = "0"
	}

	// Handle patch version
	patch := "0"
	if len(parts) > 2 {
		patch = strings.TrimLeft(parts[2], "0")
		if patch == "" {
			patch = "0"
		}
	}

	return major + "." + minor + "." + patch
}

func init() {
	checkRequirementUbuntuFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		return baseFuncMinVerOS(requirementValue, getter, "Ubuntu")
	}

	checkRequirementDebianFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		return baseFuncMinVerOS(requirementValue, getter, "Debian")
	}

	checkUnmetCloudConditionsFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		requirementValue = strings.TrimSpace(requirementValue)
		if requirementValue == "false" || requirementValue == "" {
			return true, nil
		}

		hasUnmetCloudConditions, exists := getter.Get(unmetCloudConditionsKey)
		if !exists {
			return true, nil
		}

		if hasUnmetCloudConditions.(bool) {
			return false, errors.New("has unmet cloud conditions, see clusteralerts for details")
		}

		return true, nil
	}

	requirements.RegisterCheck(unmetCloudConditionsRequirementsKey, checkUnmetCloudConditionsFunc)
	requirements.RegisterCheck(requirementsUbuntuKey, checkRequirementUbuntuFunc)
	requirements.RegisterCheck(requirementsDebianKey, checkRequirementDebianFunc)
}

func baseFuncMinVerOS(requirementValue string, getter requirements.ValueGetter, osImage string) (bool, error) {
	var minVersionValuesKey string

	// Normalize Ubuntu version format for semver parsing
	normalizedRequirementValue := requirementValue
	if osImage == "Ubuntu" {
		normalizedRequirementValue = normalizeUbuntuVersionForSemver(requirementValue)
	}

	desiredVersion, err := semver.NewVersion(normalizedRequirementValue)
	if err != nil {
		return false, err
	}
	switch osImage {
	case "Ubuntu":
		minVersionValuesKey = minUbuntuVersionValuesKey
	case "Debian":
		minVersionValuesKey = minDebianVersionValuesKey
	}

	currentVersionRaw, exists := getter.Get(minVersionValuesKey)
	if !exists {
		return true, nil
	}

	// Normalize Ubuntu version format for semver parsing
	normalizedCurrentVersion := currentVersionRaw.(string)
	if osImage == "Ubuntu" {
		normalizedCurrentVersion = normalizeUbuntuVersionForSemver(normalizedCurrentVersion)
	}

	currentVersion, err := semver.NewVersion(normalizedCurrentVersion)
	if err != nil {
		return false, err
	}

	if currentVersion.LessThan(desiredVersion) {
		return false, fmt.Errorf("minimal node %v OS version is lower then required", osImage)
	}

	return true, nil
}
