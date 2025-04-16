/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package orchestrator

import (
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/pki"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/secrets"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/users"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
)

const (
	valuesPath    = "systemRegistry.internal.orchestrator"
	SubmoduleName = "orchestrator"

	configSnapName  = "config"
	pkiSnapName     = "pki"
	secretsSnapName = "secrets"
	usersSnapName   = "users"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/system-registry/orchestrator",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       configSnapName,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-config"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var secret v1core.Secret

				err := sdk.FromUnstructured(obj, &secret)
				if err != nil {
					return "", fmt.Errorf("failed to convert config secret to struct: %v", err)
				}

				config := Params{
					Mode:       string(secret.Data["mode"]),
					ImagesRepo: string(secret.Data["imagesRepo"]),
					UserName:   string(secret.Data["username"]),
					Password:   string(secret.Data["password"]),
					TTL:        string(secret.Data["ttl"]),
				}

				return config, nil
			},
		},
		pki.KubernetsConfig(pkiSnapName),
		secrets.KubernetsConfig(secretsSnapName),
		users.KubernetsConfig(usersSnapName),
	},
},
	func(input *go_hook.HookInput) error {
		moduleValues := helpers.NewValuesAccessor[Values](input, valuesPath)
		values := moduleValues.Get()

		var (
			inputs Inputs
			err    error
		)

		inputs.Params, err = helpers.SnapshotToSingle[Params](input, configSnapName)
		if err != nil {
			if errors.Is(err, helpers.ErrNoSnapshot) {
				moduleValues.Clear()
				return nil
			}

			return fmt.Errorf("get Config snapshot error: %w", err)
		}

		inputs.PKI, err = pki.InputsFromSnapshot(input, pkiSnapName)
		if err != nil && !errors.Is(err, helpers.ErrNoSnapshot) {
			return fmt.Errorf("get PKI snapshot error: %w", err)
		}

		inputs.Secrets, err = secrets.InputsFromSnapshot(input, secretsSnapName)
		if err != nil && !errors.Is(err, helpers.ErrNoSnapshot) {
			return fmt.Errorf("get Secrets snapshot error: %w", err)
		}

		inputs.Users, err = users.InputsFromSnapshot(input, usersSnapName)
		if err != nil {
			return fmt.Errorf("get Users snapshot error: %w", err)
		}

		values.Hash, err = helpers.ComputeHash(inputs)
		if err != nil {
			return fmt.Errorf("cannot compute inputs hash: %w", err)
		}

		values.Ready, err = process(input, inputs, &values.State)
		if err != nil {
			return fmt.Errorf("cannot process: %w", err)
		}

		moduleValues.Set(values)
		return nil
	})

func process(input *go_hook.HookInput, inputs Inputs, state *State) (bool, error) {
	// TODO: this is stub code, need to write switch logic

	params := inputs.Params

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
	)

	switch params.Mode {
	case registry_const.ModeProxy:
		usersParams = users.Params{
			RO: true,
		}
		pkiEnabled = true
		secretsEnabled = true
	case registry_const.ModeDetached:
		fallthrough
	case registry_const.ModeLocal:
		usersParams = users.Params{
			RO:       true,
			RW:       true,
			Mirrorer: true,
		}
		pkiEnabled = true
		secretsEnabled = true
	case registry_const.ModeDirect:
		pkiEnabled = true
		secretsEnabled = true
	}

	if pkiEnabled {
		if state.PKI == nil {
			state.PKI = &inputs.PKI
		}

		if _, err := state.PKI.Process(input.Logger); err != nil {
			return false, fmt.Errorf("cannot process PKI: %w", err)
		}
	} else {
		state.PKI = nil
	}

	if secretsEnabled {
		if state.Secrets == nil {
			state.Secrets = &inputs.Secrets
		}

		if err := state.Secrets.Process(); err != nil {
			return false, fmt.Errorf("cannot process Secrets: %w", err)
		}
	} else {
		state.Secrets = nil
	}

	if usersParams.Any() {
		if state.Users == nil {
			state.Users = &users.State{}
		}

		if err := state.Users.Process(usersParams, inputs.Users); err != nil {
			return false, fmt.Errorf("cannot process Users: %w", err)
		}
	} else {
		state.Users = nil
	}

	state.Mode = params.Mode
	return true, nil
}
