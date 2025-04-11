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

type Config[TParams any] struct {
	Params  TParams `json:"params,omitempty"`
	Version string  `json:"version"`
	Enabled bool    `json:"-"`
}

func (config *Config[TParams]) ComputeVersion() error {
	version, err := helpers.ComputeHash(config.Params)

	if err != nil {
		return err
	}

	config.Version = version
	return nil
}

type configAccessor[TParams any] struct {
	values valuesAccessor
}

func (accessor configAccessor[TParams]) Set(params TParams) (string, error) {
	value := Config[TParams]{
		Params:  params,
		Enabled: true,
	}

	if err := value.ComputeVersion(); err != nil {
		return "", fmt.Errorf("compute version error: %w", err)
	}

	accessor.values.Set("enabled", true)
	accessor.values.Set("config", value)

	return value.Version, nil
}

func (accessor configAccessor[TParams]) Disable() {
	accessor.values.Remove("config")
	accessor.values.Set("enabbled", false)
}

func (accessor configAccessor[TParams]) Get() Config[TParams] {
	enabled := accessor.values.Get("enabled").Bool()

	var ret Config[TParams]

	if !enabled {
		return ret
	}

	value := accessor.values.Get("config")
	if !value.IsObject() {
		ret.Enabled = true
		return ret
	}

	_ = json.Unmarshal([]byte(value.Raw), &ret)

	ret.Enabled = true
	return ret
}

type ConfigAccessor[TParams any] interface {
	Set(params TParams) (string, error)
	Get() Config[TParams]
	Disable()
}

func NewConfigAccessor[TParams any](input *go_hook.HookInput, submoduleName string) ConfigAccessor[TParams] {
	return configAccessor[TParams]{
		values: valuesAccessor{
			input:    input,
			basePath: fmt.Sprintf("%s.%s", submodulesValuesPrefix, submoduleName),
		},
	}
}
