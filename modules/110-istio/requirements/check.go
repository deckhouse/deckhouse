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
	requirementsKey               = "istioVer"
	k8sKey                        = "k8s"
	minVersionValuesKey           = "istio:minimalVersion"
	isK8sVersionAutomaticKey      = "istio:isK8sVersionAutomatic"
	istioToK8sCompatibilityMapKey = "istio:istioToK8sCompatibilityMap"
)

func init() {
	checkMinimalIstioVersionFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		minimalIstioVersion, err := semver.NewVersion(requirementValue)
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

		if currentVersion.LessThan(minimalIstioVersion) {
			return false, fmt.Errorf("installed Istio version '%s' is lower than required", currentVersionStr)
		}

		return true, nil
	}

	checkIstioAndK8sVersionsCompatibility := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		comingK8sVersion := requirementValue

		currentMinIstioVersionRaw, exists := getter.Get(minVersionValuesKey)
		if !exists {
			return true, nil
		}
		currentMinIstioVersionStr := currentMinIstioVersionRaw.(string)

		isAtomaticK8sVerRaw, exists := getter.Get(isK8sVersionAutomaticKey)
		if !exists {
			return true, nil
		}
		isAtomaticK8sVer := isAtomaticK8sVerRaw.(bool)
		// Only if k8s version is set to Automatic in cluster
		if !isAtomaticK8sVer {
			return true, nil
		}

		compatibilityMapRaw, exists := getter.Get(istioToK8sCompatibilityMapKey)
		if !exists {
			return true, nil
		}
		compatibilityMap, ok := compatibilityMapRaw.(map[string][]string)
		if !ok {
			return true, nil
		}

		if k8sVersions, ok := compatibilityMap[currentMinIstioVersionStr]; ok {
			for _, k8sVersion := range k8sVersions {
				// If k8s version is in compatibility list
				if comingK8sVersion == k8sVersion {
					return true, nil
				}
			}
			return false, fmt.Errorf("after update kubernetes version '%s' will be incompatible with Istio version '%s'", comingK8sVersion, currentMinIstioVersionStr)
		}

		return true, nil
	}

	requirements.RegisterCheck(requirementsKey, checkMinimalIstioVersionFunc)
	requirements.RegisterCheck(k8sKey, checkIstioAndK8sVersionsCompatibility)
}
