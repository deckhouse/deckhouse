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
	requirementIstioMinimalVersionKey = "istioMinimalVersion"
	requirementDefaultK8sKey          = "k8s"
	minVersionValuesKey               = "istio:minimalVersion"
	isK8sVersionAutomaticKey          = "istio:isK8sVersionAutomatic"
	istioToK8sCompatibilityMapKey     = "istio:istioToK8sCompatibilityMap"
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

	checkIstioAndK8sVersionsCompatibilityFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		comingDefaultK8sVersionSemver, err := semver.NewVersion(requirementValue)
		if err != nil {
			return false, err
		}
		comingDefaultK8sVersion := fmt.Sprintf("%d.%d", comingDefaultK8sVersionSemver.Major(), comingDefaultK8sVersionSemver.Minor())

		comingDefaultK8sVersion := requirementValue
		currentMinIstioVersionRaw, exists := getter.Get(minVersionValuesKey)
		if !exists {
			return false, fmt.Errorf("%s key is not registred", minVersionValuesKey)
		}
		currentMinIstioVersionStr := currentMinIstioVersionRaw.(string)

		isAtomaticK8sVerRaw, exists := getter.Get(isK8sVersionAutomaticKey)
		if !exists {
			return false, fmt.Errorf("%s key is not registred", isK8sVersionAutomaticKey)
		}
		isAtomaticK8sVer := isAtomaticK8sVerRaw.(bool)
		// Only if k8s version is set to Automatic in cluster
		if !isAtomaticK8sVer {
			return true, nil
		}

		compatibilityMapRaw, exists := getter.Get(istioToK8sCompatibilityMapKey)
		if !exists {
			return false, fmt.Errorf("%s key is not registred", istioToK8sCompatibilityMapKey)
		}
		compatibilityMap, ok := compatibilityMapRaw.(map[string][]string)
		if !ok {
			return false, fmt.Errorf("%s key format is incorrect", istioToK8sCompatibilityMapKey)
		}

		k8sVersions, ok := compatibilityMap[currentMinIstioVersionStr]
		if !ok {
			return false, fmt.Errorf("can't find compatible k8s versions for Istio v%s", currentMinIstioVersionStr)
		}
		for _, k8sVersion := range k8sVersions {
			// If k8s version is in compatibility list
			if comingDefaultK8sVersion == k8sVersion {
				return true, nil
			}
		}
		return false, fmt.Errorf("in coming release the default kubernetes version '%s' will be incompatible with Istio version '%s'", comingDefaultK8sVersion, currentMinIstioVersionStr)
	}

	requirements.RegisterCheck(requirementIstioMinimalVersionKey, checkMinimalIstioVersionFunc)
	requirements.RegisterCheck(requirementDefaultK8sKey, checkIstioAndK8sVersionsCompatibilityFunc)
}
