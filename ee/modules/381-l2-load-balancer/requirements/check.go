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
	l2LoadBalancerModuleDeprecatedKey                = "l2LoadBalancer:isModuleEnabled"
	l2LoadBalancerModuleDeprecatedKeyRequirementsKey = "l2LoadBalancerModuleEnabled"
)

func init() {
	checkRequirementIsModuleEnabled := func(_ string, getter requirements.ValueGetter) (bool, error) {
		isModuleIsEnabled, exists := getter.Get(l2LoadBalancerModuleDeprecatedKey)
		if exists && isModuleIsEnabled.(bool) {
			return false, errors.New("the L2LoadBalancer module is deprecated and will be removed in a future release. Use MetalLB module in L2 mode")
		}
		return true, nil
	}

	requirements.RegisterCheck(l2LoadBalancerModuleDeprecatedKeyRequirementsKey, checkRequirementIsModuleEnabled)
}
