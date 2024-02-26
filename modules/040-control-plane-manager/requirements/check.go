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

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const minK8sVersionRequirementKey = "controlPlaneManager:minUsedControlPlaneKubernetesVersion"

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
}
