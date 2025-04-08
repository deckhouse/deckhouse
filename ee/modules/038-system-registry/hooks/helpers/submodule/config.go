/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package submodule

import (
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
)

type SubmoduleConfig[TParams any] struct {
	Params  TParams `json:"params,omitempty"`
	Version string  `json:"version"`
	Enabled bool    `json:"-"`
}

func (config *SubmoduleConfig[TParams]) ComputeVersion() error {
	version, err := helpers.ComputeHash(config.Params)

	if err != nil {
		return err
	}

	config.Version = version
	return nil
}

func SetSubmoduleConfig[TParams any](input *go_hook.HookInput, name string, params TParams) (string, error) {
	values := input.Values

	value := SubmoduleConfig[TParams]{
		Params:  params,
		Enabled: true,
	}

	if err := value.ComputeVersion(); err != nil {
		return "", fmt.Errorf("compute version error: %w", err)
	}

	values.Set(fmt.Sprintf("%s.%s.config", submodulesValuesPrefix, name), value)
	values.Set(fmt.Sprintf("%s.%s.enabled", submodulesValuesPrefix, name), true)

	return value.Version, nil
}

func DisableSubmodule(input *go_hook.HookInput, name string) {
	values := input.Values

	values.Set(fmt.Sprintf("%s.%s.enabled", submodulesValuesPrefix, name), false)
	values.Remove(fmt.Sprintf("%s.%s.config", submodulesValuesPrefix, name))
}

func GetSubmoduleConfig[TParams any](input *go_hook.HookInput, name string) SubmoduleConfig[TParams] {
	values := input.Values

	enabled := values.Get(fmt.Sprintf("%s.%s.enabled", submodulesValuesPrefix, name)).Bool()

	var ret SubmoduleConfig[TParams]

	if !enabled {
		return ret
	}

	value := values.Get(fmt.Sprintf("%s.%s.config", submodulesValuesPrefix, name))

	if !value.IsObject() {
		ret.Enabled = true
		return ret
	}

	_ = json.Unmarshal([]byte(value.Raw), &ret)

	ret.Enabled = true
	return ret
}
