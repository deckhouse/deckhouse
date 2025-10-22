/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// TODO: Delete this file and the corresponding requirements in 'release.yaml' after version 1.67.
package requirements

import (
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	metallbConfigurationStatusRequirementsKey = "metallbHasStandardConfiguration"
)

func init() {
	checkRequirementConfigurationStatus := func(_ string, _ requirements.ValueGetter) (bool, error) {
		return true, nil
	}
	requirements.RegisterCheck(metallbConfigurationStatusRequirementsKey, checkRequirementConfigurationStatus)
}
