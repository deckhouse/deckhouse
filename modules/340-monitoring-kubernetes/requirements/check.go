/*
Copyright 2024 Flant JSC

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

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/modules/340-monitoring-kubernetes/hooks"
)

func init() {
	f := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		desiredVersion, err := semver.NewVersion(requirementValue)
		if err != nil {
			return false, err
		}
		unavailabelVersionStr, exists := getter.Get(hooks.AutoK8sVersion)
		if !exists {
			return true, nil
		}
		unavailabelVersion, err := semver.NewVersion(unavailabelVersionStr.(string))
		if err != nil {
			return false, err
		}

		if !desiredVersion.LessThan(unavailabelVersion) {
			if reason, exists := getter.Get(hooks.AutoK8sReason); exists {
				return false, fmt.Errorf("k8s version is not available because outdated versions of resources are used: %v", reason)
			}

			return false, errors.New("k8s version is not available because outdated versions of resources are used")
		}

		return true, nil
	}

	requirements.RegisterCheck("autoK8sVersion", f)
}
