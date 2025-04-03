/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package status

import (
	"errors"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/helpers"
)

const (
	StatusSpecLocation = "systemRegistry.internal.bashible.status"
)

type Status struct {
	Ready   bool                  `json:"ready"`
	Version string                `json:"version"`
	Nodes   map[string]NodeStatus `json:"node"`
}

type NodeStatus struct {
	Ready   bool   `json:"ready"`
	Version string `json:"version"`
}

func Get(input *go_hook.HookInput) (*Status, error) {
	var ret Status
	err := helpers.UnmarshalInputValue(input, StatusSpecLocation, &ret)
	if errors.Is(err, helpers.ErrInputValueNotExist) {
		return nil, nil
	}
	return &ret, err
}

func Remove(input *go_hook.HookInput) {
	obj := input.Values.Get(StatusSpecLocation)

	if obj.Exists() {
		input.Values.Remove(StatusSpecLocation)
	}
}

func Set(input *go_hook.HookInput, cfg Status) {
	input.Values.Set(StatusSpecLocation, cfg)
}

func SetIfNotExist(input *go_hook.HookInput, cfg Status) {
	obj := input.Values.Get(StatusSpecLocation)

	if !obj.Exists() {
		input.Values.Set(StatusSpecLocation, cfg)
	}
}
