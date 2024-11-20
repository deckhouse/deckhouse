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
	cniConfigurationSettledKey             = "cniConfigurationSettled"
	cniConfigurationSettledRequirementsKey = "cniConfigurationSettled"
)

func init() {
	checkCNIConfigurationSettledFunc := func(_ string, getter requirements.ValueGetter) (bool, error) {
		cniConfigurationSettledStatusRaw, exists := getter.Get(cniConfigurationSettledKey)
		if !exists {
			return true, nil
		}

		if cniConfigurationSettledStatus, ok := cniConfigurationSettledStatusRaw.(string); ok {
			if cniConfigurationSettledStatus == "false" {
				return false, errors.New(
					"A problem has been found in the CNI configuration, see ClusterAlerts for details",
				)
			}
		}
		return true, nil
	}
	requirements.RegisterCheck(cniConfigurationSettledRequirementsKey, checkCNIConfigurationSettledFunc)
}
