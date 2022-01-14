/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"errors"

	"github.com/blang/semver"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

func init() {
	f := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		desiredVersion, err := semver.Parse(requirementValue)
		if err != nil {
			return false, err
		}

		currentVersionStr := getter.Get("global.discovery.kubernetesVersion").String()
		currentVersion, err := semver.Parse(currentVersionStr)
		if err != nil {
			return false, err
		}

		if currentVersion.GE(desiredVersion) {
			return true, nil
		}

		return false, errors.New("current kubernetes version is lower then required")
	}

	requirements.Register("k8s", f)
}
