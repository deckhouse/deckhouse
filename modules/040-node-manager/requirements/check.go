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
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	minUbuntuVersionValuesKey = "nodeManager:nodesMinimalOSVersionUbuntu"
	minDebianVersionValuesKey = "nodeManager:nodesMinimalOSVersionDebian"
	requirementsUbuntuKey     = "nodesMinimalOSVersionUbuntu"
	requirementsDebianKey     = "nodesMinimalOSVersionDebian"
)

func init() {
	checkRequirementUbuntuFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		return baseFuncMinVerOS(requirementValue, getter, "Ubuntu")
	}

	checkRequirementDebianFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		return baseFuncMinVerOS(requirementValue, getter, "Debian")
	}

	requirements.RegisterCheck(requirementsUbuntuKey, checkRequirementUbuntuFunc)
	requirements.RegisterCheck(requirementsDebianKey, checkRequirementDebianFunc)
}

func baseFuncMinVerOS(requirementValue string, getter requirements.ValueGetter, osImage string) (bool, error) {
	var minVersionValuesKey string
	desiredVersion, err := semver.NewVersion(requirementValue)
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
	currentVersion, err := semver.NewVersion(currentVersionRaw.(string))
	if err != nil {
		return false, err
	}

	if currentVersion.LessThan(desiredVersion) {
		return false, fmt.Errorf("minimal node %v OS version is lower then required", osImage)
	}

	return true, nil
}
