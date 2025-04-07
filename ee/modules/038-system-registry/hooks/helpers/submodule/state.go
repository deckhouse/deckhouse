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
	Data    TData  `json:"data,omitempty"`
}

func SetSubmoduleState[TData any](values go_hook.PatchableValuesCollector, name string, value SubmoduleState[TData]) {
	values.Set(fmt.Sprintf("%s.%s.state", submodulesValuesPrefix, name), value)
}

func GetSubmoduleState[TData any](values go_hook.PatchableValuesCollector, name string) SubmoduleState[TData] {
	value := values.Get(fmt.Sprintf("%s.%s.state", submodulesValuesPrefix, name))

	var ret SubmoduleState[TData]
	if !value.IsObject() {
		return ret
	}

	_ = json.Unmarshal([]byte(value.Raw), &ret)
	return ret
}

func RemoveSubmoduleState(values go_hook.PatchableValuesCollector, name string) {
	values.Remove(fmt.Sprintf("%s.%s.state", submodulesValuesPrefix, name))
}
