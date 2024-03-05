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

package requirements

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	requirementsKey     = "istioVer"
	minVersionValuesKey = "istio:minimalVersion"
)

func init() {
	checkRequirementFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		desiredVersion, err := semver.NewVersion(requirementValue)
		if err != nil {
			return false, err
		}
		currentVersionRaw, exists := getter.Get(minVersionValuesKey)
		if !exists {
			return true, nil
		}
		currentVersionStr := currentVersionRaw.(string)
		currentVersion, err := semver.NewVersion(currentVersionStr)
		if err != nil {
			return false, err
		}

		if currentVersion.LessThan(desiredVersion) {
			return false, fmt.Errorf("installed Istio version '%s' is lower than required", currentVersionStr)
		}

		return true, nil
	}

	requirements.RegisterCheck(requirementsKey, checkRequirementFunc)
}
