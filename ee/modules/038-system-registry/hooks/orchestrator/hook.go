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
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/secrets"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/users"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
)

const (
	configSnapName = "config"
	SubmoduleName  = "orchestrator"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        fmt.Sprintf("/modules/system-registry/submodule-%s", SubmoduleName),
},
	func(input *go_hook.HookInput) error {
		moduleConfig := submodule.NewConfigAccessor[Params](input, SubmoduleName)
		moduleState := submodule.NewStateAccessor[State](input, SubmoduleName)

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

	if params.Mode == "" {
		params.Mode = registry_const.ModeUnmanaged
	}

	if params.Mode != state.TargetMode {
		input.Logger.Warn(
			"Target mode change",
			"old_mode", state.TargetMode,
			"new_mode", params.Mode,
		)

		state.TargetMode = params.Mode
	}

	var (
		usersParams    users.Params
		pkiEnabled     bool
		secretsEnabled bool
		err            error
	)

	switch state.TargetMode {
	case registry_const.ModeProxy:
		usersParams = users.Params{
			"ro",
			"rw",
		}
		pkiEnabled = true
		secretsEnabled = true
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
		secretsEnabled = true
	case registry_const.ModeDirect:
		pkiEnabled = true
		secretsEnabled = true
	}

	pkiConfig := submodule.NewConfigAccessor[pki.Params](input, pki.SubmoduleName)
	secretsConfig := submodule.NewConfigAccessor[secrets.Params](input, secrets.SubmoduleName)
	usersConfig := submodule.NewConfigAccessor[users.Params](input, users.SubmoduleName)

	if pkiEnabled {
		state.PKIVersion, err = pkiConfig.Set(pki.Params{})
		if err != nil {
			return false, fmt.Errorf("cannot set PKI params: %w", err)
		}
	} else {
		state.PKIVersion = ""
		pkiConfig.Disable()
	}

	if secretsEnabled {
		state.SecretsVersion, err = secretsConfig.Set(secrets.Params{})
		if err != nil {
			return false, fmt.Errorf("cannot set Secrets params: %w", err)
		}
	} else {
		state.SecretsVersion = ""
		secretsConfig.Disable()
	}

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
