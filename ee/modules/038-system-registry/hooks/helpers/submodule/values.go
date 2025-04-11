/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package submodule

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/tidwall/gjson"
)

type valuesAccessor struct {
	input    *go_hook.HookInput
	basePath string
}

func (values valuesAccessor) Set(key string, value any) {
	values.input.Values.Set(
		fmt.Sprintf("%s.%s", values.basePath, key),
		value,
	)
}

func (values valuesAccessor) Get(key string) gjson.Result {
	return values.input.Values.Get(
		fmt.Sprintf("%s.%s", values.basePath, key),
	)
}

func (values valuesAccessor) Remove(key string) {
	values.input.Values.Remove(
		fmt.Sprintf("%s.%s", values.basePath, key),
	)
}
