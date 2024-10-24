/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package requirements

import (
	"errors"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	metallbConfigurationStatusKey             = "metallb:ConfigurationStatus"
	metallbConfigurationStatusRequirementsKey = "metallbHasStandardConfiguration"
)

func init() {
	checkRequirementConfigurationStatus := func(_ string, getter requirements.ValueGetter) (bool, error) {
		configurationStatusRaw, exists := getter.Get(metallbConfigurationStatusKey)
		if !exists {
			return true, nil
		}

		configurationStatus := configurationStatusRaw.(string)
		switch {
		case configurationStatus == "nsMismatch":
			return false, errors.New(
				"metallb: all L2Advertisement must be in the d8-metallb namespace",
			)
		case configurationStatus == "nodeSelectorsMismatch":
			return false, errors.New(
				"metallb: nodeSelectors in L2Advertisement must contain only " +
					"one matchLabels (not matchExpressions)",
			)
		case configurationStatus == "addressPollsMismatch":
			return false, errors.New(
				"metallb: there should not be layer2 and bgp pools in the cluster at the same time",
			)
		}
		return true, nil
	}
	requirements.RegisterCheck(metallbConfigurationStatusRequirementsKey, checkRequirementConfigurationStatus)
}
