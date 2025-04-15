/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package orchestrator

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers/submodule"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/pki"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/secrets"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/users"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
)

const (
	configSnapName = "config"
	pkiSnapName    = "pki"
	SubmoduleName  = "orchestrator"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        fmt.Sprintf("/modules/system-registry/submodule-%s", SubmoduleName),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              pkiSnapName,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: helpers.NamespaceSelector,
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					"registry-pki",
				},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var secret v1core.Secret

				err := sdk.FromUnstructured(obj, &secret)
				if err != nil {
					return "", fmt.Errorf("failed to convert secret \"%v\" to struct: %v", obj.GetName(), err)
				}

				ret := pki.State{
					CA:    pki.SecretDataToCertModel(secret, "ca"),
					Token: pki.SecretDataToCertModel(secret, "token"),
				}

				return ret, nil
			},
		},
	},
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

		var (
			inputs Inputs
			err    error
		)

		if inputs.PKI, err = helpers.SnapshotToSingle[pki.State](input, pkiSnapName); err != nil {
			// TODO: remove
			input.Logger.Warn("Get PKI snapshot error", "error", err)
		}

		ready, err := process(input, config.Params, inputs, &state.Data)
		if err != nil {
			return fmt.Errorf("cannot process: %w", err)
		}

		hash, err := helpers.ComputeHash(inputs)
		if err != nil {
			return fmt.Errorf("cannot compute inputs hash: %w", err)
		}

		state.Ready = ready
		state.Hash = hash

		moduleState.Set(state)
		return nil
	})

func process(input *go_hook.HookInput, params Params, inputs Inputs, state *State) (bool, error) {
	// TODO: this is stub code, need to write switch logic

	if params.Mode == "" {
		params.Mode = registry_const.ModeUnmanaged
	}

	if params.Mode != state.Mode {
		input.Logger.Warn(
			"Mode change",
			"old_mode", state.Mode,
			"new_mode", params.Mode,
		)
	}

	var (
		usersParams    users.Params
		pkiEnabled     bool
		secretsEnabled bool
		err            error
	)

	switch params.Mode {
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

	secretsConfig := submodule.NewConfigAccessor[secrets.Params](input, secrets.SubmoduleName)
	usersConfig := submodule.NewConfigAccessor[users.Params](input, users.SubmoduleName)

	if pkiEnabled {
		if state.PKI == nil || state.PKI.CA == nil {
			state.PKI = &inputs.PKI
		}

		_, err := state.PKI.Process(input.Logger)
		if err != nil {
			return false, fmt.Errorf("cannot process PKI: %w", err)
		}
	} else {
		state.PKI = nil
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

	state.Mode = params.Mode
	return true, nil
}
