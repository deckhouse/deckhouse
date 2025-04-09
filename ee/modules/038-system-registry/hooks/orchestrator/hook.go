/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package orchestrator

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers/submodule"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/users"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	configSnapName = "config"
	submoduleName  = "orchestrator"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/system-registry/orchestrator",
},
	func(input *go_hook.HookInput) error {
		config := submodule.GetSubmoduleConfig[Params](input, submoduleName)
		state := submodule.GetSubmoduleState[State](input, submoduleName)

		if !config.Enabled {
			//TODO
			submodule.RemoveSubmoduleState(input, "orchestrator")
			return nil
		}

		ready, err := process(input, config.Params, &state.Data)
		if err != nil {
			return fmt.Errorf("cannot process: %w", err)
		}

		state.Ready = ready
		submodule.SetSubmoduleState(input, submoduleName, state)
		return nil
	})

func process(input *go_hook.HookInput, params Params, state *State) (bool, error) {
	if params.Mode != state.TargetMode {
		input.Logger.Warn(
			"Target mode change",
			"old_mode", state.TargetMode,
			"new_mode", params.Mode,
		)

		state.TargetMode = params.Mode
	}

	var (
		usersParams  users.Params
		usersVersion string
		err          error
	)

	switch state.TargetMode {
	case registry_const.ModeProxy:
		usersParams = users.Params{
			"ro",
			"rw",
		}
	case registry_const.ModeDetached:
		fallthrough
	case registry_const.ModeLocal:
		usersParams = users.Params{
			"ro",
			"rw",
			"mirrorer-puller",
			"mirrorer-pusher",
		}
	}

	if len(usersParams) > 0 {
		usersVersion, err = submodule.SetSubmoduleConfig(input, "users", usersParams)
		if err != nil {
			return false, fmt.Errorf("cannot set users params: %w", err)
		}
	} else {
		submodule.DisableSubmodule(input, "users")
		usersVersion = "disabled"
	}

	log.Warn(
		"Users params set",
		"config", params,
		"params", usersParams,
		"version", usersVersion,
	)

	state.Mode = state.TargetMode
	return true, nil
}
