/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package submodule

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

type SubmoduleConfig[TParams any] struct {
	Params  TParams `json:"params,omitempty"`
	Version string  `json:"version"`
	Enabled bool    `json:"-"`
}

func (config *SubmoduleConfig[TParams]) ComputeVersion() error {
	buf, err := json.Marshal(config.Params)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	hashBytes := sha256.Sum256(buf)
	config.Version = hex.EncodeToString(hashBytes[:])

	return nil
}

func SetSubmoduleConfig[TParams any](values go_hook.PatchableValuesCollector, name string, params TParams) (string, error) {
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

func DisableSubmodule(values go_hook.PatchableValuesCollector, name string) {
	values.Set(fmt.Sprintf("%s.%s.enabled", submodulesValuesPrefix, name), false)
	values.Remove(fmt.Sprintf("%s.%s.config", submodulesValuesPrefix, name))
}

func GetSubmoduleConfig[TParams any](values go_hook.PatchableValuesCollector, name string) SubmoduleConfig[TParams] {
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
