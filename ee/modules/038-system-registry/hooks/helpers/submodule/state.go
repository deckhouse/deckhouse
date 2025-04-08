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

type SubmoduleState[TData any] struct {
	Version string `json:"version"`
	Hash    string `json:"hash"`
	Data    TData  `json:"data,omitempty"`
}

func SetSubmoduleState[TData any](input *go_hook.HookInput, name string, value SubmoduleState[TData]) {
	values := input.Values
	values.Set(fmt.Sprintf("%s.%s.state", submodulesValuesPrefix, name), value)
}

func GetSubmoduleState[TData any](input *go_hook.HookInput, name string) SubmoduleState[TData] {
	values := input.Values
	value := values.Get(fmt.Sprintf("%s.%s.state", submodulesValuesPrefix, name))

	var ret SubmoduleState[TData]
	if !value.IsObject() {
		return ret
	}

	_ = json.Unmarshal([]byte(value.Raw), &ret)
	return ret
}

func RemoveSubmoduleState(input *go_hook.HookInput, name string) {
	values := input.Values
	values.Remove(fmt.Sprintf("%s.%s.state", submodulesValuesPrefix, name))
}
