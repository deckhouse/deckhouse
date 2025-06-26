/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package checker

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
)

func SetParams(input *go_hook.HookInput, params Params) {
	accessor := helpers.NewValuesAccessor[Params](input, valuesParamsPath)
	accessor.Set(params)
}

func GetParams(input *go_hook.HookInput) Params {
	accessor := helpers.NewValuesAccessor[Params](input, valuesParamsPath)
	return accessor.Get()
}

func GetStatus(input *go_hook.HookInput) Status {
	accessor := helpers.NewValuesAccessor[Status](input, valuesStatePath)
	status := accessor.Get()
	return status
}
