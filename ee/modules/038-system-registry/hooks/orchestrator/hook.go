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
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/pki"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/users"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
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
		moduleConfig := submodule.NewConfigAccessor[Params](input, submoduleName)
		moduleState := submodule.NewStateAccessor[State](input, submoduleName)

		config := moduleConfig.Get()
		state := moduleState.Get()

		if !config.Enabled {
			// TODO
			moduleState.Clear()
			return nil
		}

		ready, err := process(input, config.Params, &state.Data)
		if err != nil {
			return fmt.Errorf("cannot process: %w", err)
		}

		state.Ready = ready
		moduleState.Set(state)
		return nil
	})

func process(input *go_hook.HookInput, params Params, state *State) (bool, error) {
	// TODO: this is stub code, need to write switch logic

	if params.Mode != state.TargetMode {
		input.Logger.Warn(
			"Target mode change",
			"old_mode", state.TargetMode,
			"new_mode", params.Mode,
		)

		state.TargetMode = params.Mode
	}

	var (
		usersParams users.Params
		pkiEnabled  bool
		err         error
	)

	switch state.TargetMode {
	case registry_const.ModeProxy:
		usersParams = users.Params{
			"ro",
			"rw",
		}
		pkiEnabled = true
	case registry_const.ModeDetached:
		fallthrough
	case registry_const.ModeLocal:
		usersParams = users.Params{
			"ro",
			"rw",
			"mirror-puller",
			"mirror-pusher",
		}
		pkiEnabled = true
	case registry_const.ModeDirect:
		pkiEnabled = true
	}

	pkiConfig := submodule.NewConfigAccessor[pki.Params](input, pki.SubmoduleName)

	if pkiEnabled {
		state.PKIVersion, err = pkiConfig.Set(pki.Params{})
		if err != nil {
			return false, fmt.Errorf("cannot set PKI params: %w", err)
		}
	} else {
		state.PKIVersion = ""
		pkiConfig.Disable()
	}

	usersConfig := submodule.NewConfigAccessor[users.Params](input, users.SubmoduleName)
	if len(usersParams) > 0 {
		state.UsersVersion, err = usersConfig.Set(usersParams)
		if err != nil {
			return false, fmt.Errorf("cannot set users params: %w", err)
		}
	} else {
		usersConfig.Disable()
		state.UsersVersion = ""
	}

	state.Mode = state.TargetMode
	return true, nil
}
