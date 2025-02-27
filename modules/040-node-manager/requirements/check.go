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
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	containerdRequirementsKey = "containerdOnAllNodes"
	hasNodesWithDocker        = "nodeManager:hasNodesWithDocker"
)

func init() {
	checkContainerdRequirementFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		requirementValue = strings.TrimSpace(requirementValue)
		if requirementValue == "false" || requirementValue == "" {
			return true, nil
		}

		hasDocker, exists := getter.Get(hasNodesWithDocker)
		if !exists {
			return true, nil
		}

		if hasDocker.(bool) {
			return false, errors.New("has nodes with Docker CRI or defaultCRI is Docker")
		}

		return true, nil
	}
	requirements.RegisterCheck(containerdRequirementsKey, checkContainerdRequirementFunc)
}
