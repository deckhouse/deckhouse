/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package input

import (
	"errors"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/helpers"
	common_models "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/models"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
)

const (
	InputSpecLocation = "systemRegistry.internal.bashible.input"
)

type InputModel struct {
	Version  string                  `json:"version"`
	Mode     registry_const.ModeType `json:"mode"`
	Proxy    *ProxyInputModel        `json:"proxy,omitempty"`
	Detached *DetachedInputModel     `json:"detached,omitempty"`
}

type ProxyInputModel struct {
	CA   common_models.CertModel `json:"ca"`
	User common_models.UserModel `json:"user"`
}

type DetachedInputModel struct {
	CA   common_models.CertModel `json:"ca"`
	User common_models.UserModel `json:"user"`
}

func Get(input *go_hook.HookInput) (*InputModel, error) {
	var ret InputModel
	err := helpers.UnmarshalInputValue(input, InputSpecLocation, &ret)
	if errors.Is(err, helpers.ErrInputValueNotExist) {
		return nil, nil
	}
	return &ret, err
}

func Remove(input *go_hook.HookInput) {
	obj := input.Values.Get(InputSpecLocation)

	if obj.Exists() {
		input.Values.Remove(InputSpecLocation)
	}
}

func Set(input *go_hook.HookInput, cfg InputModel) {
	input.Values.Set(InputSpecLocation, cfg)
}

func SetIfNotExist(input *go_hook.HookInput, cfg InputModel) {
	obj := input.Values.Get(InputSpecLocation)

	if !obj.Exists() {
		input.Values.Set(InputSpecLocation, cfg)
	}
}
