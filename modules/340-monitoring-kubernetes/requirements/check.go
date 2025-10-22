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
	"fmt"
	"strings"

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
		deprecatesK8sVersionsRaw, exists := getter.Get(hooks.K8sVersionsWithDeprecations)
		if !exists {
			return true, nil
		}

		deprecatedK8sVersions := deprecatesK8sVersionsRaw.(string)
		if deprecatedK8sVersions == "initial" {
			return false, fmt.Errorf("checking for deprecated resources")
		}

		if deprecatedK8sVersions == "" {
			return true, nil
		}

		arr := strings.Split(deprecatedK8sVersions, ",")
		for _, k8VersionStr := range arr {
			k8Version := semver.MustParse(k8VersionStr)
			if k8Version.LessThan(desiredVersion) || k8Version.Equal(desiredVersion) {
				return false, fmt.Errorf("k8s version is not available because deprecated resources are used. Check alerts for details")
			}
		}

		return true, nil
	}

	requirements.RegisterCheck("autoK8sVersion", f)
}
