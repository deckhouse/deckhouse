/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package submodule

import (
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

type State[TData any] struct {
	Ready   bool   `json:"ready"`
	Version string `json:"version"`
	Hash    string `json:"hash"`
	Data    TData  `json:"data,omitempty"`
}

type stateAccessor[TData any] struct {
	values valuesAccessor
}

func (accessor stateAccessor[TData]) Set(value State[TData]) {
	accessor.values.Set("state", value)
}

func (accessor stateAccessor[TData]) Get() State[TData] {
	value := accessor.values.Get("state")

	var ret State[TData]
	if !value.IsObject() {
		return ret
	}

	_ = json.Unmarshal([]byte(value.Raw), &ret)
	return ret
}

func (accessor stateAccessor[TData]) Clear() {
	accessor.values.Remove("state")
}

type StateAccessor[TData any] interface {
	Set(value State[TData])
	Get() State[TData]
	Clear()
}

func NewStateAccessor[TData any](input *go_hook.HookInput, submoduleName string) StateAccessor[TData] {
	return stateAccessor[TData]{
		values: valuesAccessor{
			input:    input,
			basePath: fmt.Sprintf("%s.%s", submodulesValuesPrefix, submoduleName),
		},
	}
}
