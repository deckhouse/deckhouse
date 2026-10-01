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
	installedVersionsValuesKey        = "istio:installedVersions"
	isK8sVersionAutomaticKey          = "istio:isK8sVersionAutomatic"
	istioToK8sCompatibilityMapKey     = "istio:istioToK8sCompatibilityMap"
)

func init() {
	checkMinimalIstioVersionFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		minimalIstioVersion, err := semver.NewVersion(requirementValue)
		if err != nil {
			return false, err
		}
		installedIstioVersions, exists, err := getInstalledIstioVersions(getter)
		if err != nil {
			return false, err
		}
		if !exists {
			return true, nil
		}

		for _, installedVersionStr := range installedIstioVersions {
			installedVersion, err := semver.NewVersion(installedVersionStr)
			if err != nil {
				return false, err
			}
			if installedVersion.LessThan(minimalIstioVersion) {
				return false, fmt.Errorf("installed Istio version '%s' is lower than required", installedVersionStr)
			}
		}

		return true, nil
	}

	checkIstioAndK8sVersionsCompatibilityFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		comingDefaultK8sVersionSemver, err := semver.NewVersion(requirementValue)
		if err != nil {
			return false, err
		}
		comingDefaultK8sVersion := fmt.Sprintf("%d.%d", comingDefaultK8sVersionSemver.Major(), comingDefaultK8sVersionSemver.Minor())

		installedIstioVersions, exists, err := getInstalledIstioVersions(getter)
		if err != nil {
			return false, err
		}
		if !exists {
			return true, nil
		}

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
			return false, fmt.Errorf("%s key format is incorrect", istioToK8sCompatibilityMapKey)
		}

		for _, istioVersion := range installedIstioVersions {
			k8sVersions, ok := compatibilityMap[istioVersion]
			if !ok {
				return false, fmt.Errorf("can't find compatible k8s versions for Istio v%s", istioVersion)
			}
			compatible := false
			for _, k8sVersion := range k8sVersions {
				if comingDefaultK8sVersion == k8sVersion {
					compatible = true
					break
				}
			}
			if !compatible {
				return false, fmt.Errorf("in coming release the default kubernetes version '%s' will be incompatible with Istio version '%s'", comingDefaultK8sVersion, istioVersion)
			}
		}
		return true, nil
	}

	requirements.RegisterCheck(requirementIstioMinimalVersionKey, checkMinimalIstioVersionFunc)
	requirements.RegisterCheck(requirementDefaultK8sKey, checkIstioAndK8sVersionsCompatibilityFunc)
}

// getInstalledIstioVersions returns the Major.Minor versions of every Istio
// control plane in the cluster, as discovery_operator_versions_to_install.go
// saves them.
func getInstalledIstioVersions(getter requirements.ValueGetter) ([]string, bool, error) {
	raw, exists := getter.Get(installedVersionsValuesKey)
	if !exists {
		return nil, false, nil
	}

	versions, ok := raw.([]string)
	if !ok {
		return nil, false, fmt.Errorf("%s key format is incorrect", installedVersionsValuesKey)
	}
	return versions, true, nil
}
