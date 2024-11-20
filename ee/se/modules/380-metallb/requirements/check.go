/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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

		if configurationStatus, ok := configurationStatusRaw.(string); ok {
			if configurationStatus == "Misconfigured" {
				return false, errors.New(
					"[metallb] cluster misconfigured, see ClusterAlerts for details",
				)
			}
		}
		return true, nil
	}
	requirements.RegisterCheck(metallbConfigurationStatusRequirementsKey, checkRequirementConfigurationStatus)
}
