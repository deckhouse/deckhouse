/*
Copyright 2025 Flant JSC

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

package checker

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

func SetParams(input *go_hook.HookInput, params Params) error {
	if err := params.Validate(); err != nil {
		return err
	}

	accessor := helpers.NewValuesAccessor[Params](input, valuesParamsPath)
	accessor.Set(params)

	return nil
}

func GetParams(_ context.Context, input *go_hook.HookInput) Params {
	accessor := helpers.NewValuesAccessor[Params](input, valuesParamsPath)
	return accessor.Get()
}

func GetStatus(_ context.Context, input *go_hook.HookInput) Status {
	accessor := helpers.NewValuesAccessor[Status](input, valuesStatePath)
	status := accessor.Get()
	return status
}
