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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	nodeservices "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/node-services"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/pki"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/users"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
)

const (
	valuesPath    = "systemRegistry.internal.orchestrator"
	SubmoduleName = "orchestrator"

	configSnapName       = "config"
	stateSnapName        = "state"
	pkiSnapName          = "pki"
	secretsSnapName      = "secrets"
	usersSnapName        = "users"
	nodeServicesSnapName = "node-services"
)

func getKubernetesConfigs() []go_hook.KubernetesConfig {
	ret := []go_hook.KubernetesConfig{
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
					return nil, fmt.Errorf("failed to convert config secret to struct: %v", err)
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
		{
			Name:       stateSnapName,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-state"},
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
					return nil, fmt.Errorf("failed to convert config secret to struct: %v", err)
				}

				stateData, ok := secret.Data["state"]
				if !ok {
					return nil, nil
				}

				return stateData, nil
			},
		},
		pki.KubernetsConfig(pkiSnapName),
		users.KubernetsConfig(usersSnapName),
	}

	ret = append(ret, nodeservices.KubernetsConfig(nodeServicesSnapName)...)

	return ret
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/system-registry/orchestrator",
	Kubernetes:   getKubernetesConfigs(),
},
	handle,
)

func handle(input *go_hook.HookInput) error {
	moduleValues := helpers.NewValuesAccessor[Values](input, valuesPath)
	values := moduleValues.Get()

	var (
		inputs Inputs
		err    error
	)

	if values.State.Mode == "" {
		input.Logger.Info("State not initialized, trying restore from secret")

		stateData, err := helpers.SnapshotToSingle[[]byte](input, stateSnapName)
		if err == nil {
			if err = yaml.Unmarshal(stateData, &values.State); err != nil {
				err = fmt.Errorf("cannot unmarhsal YAML: %w", err)
			}
		}

		if err != nil {
			input.Logger.Warn(
				"Cannot restore state from secret, will initialize new",
				"error", err,
			)

			values.State = State{
				Mode: registry_const.ModeUnmanaged,
			}
		} else {
			input.Logger.Info("State successfully restored from secret")
		}
	}

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

	inputs.Users, err = users.InputsFromSnapshot(input, usersSnapName)
	if err != nil {
		return fmt.Errorf("get Users snapshot error: %w", err)
	}

	inputs.NodeServices, err = nodeservices.InputsFromSnapshot(input, nodeServicesSnapName)
	if err != nil {
		return fmt.Errorf("get NodeServices snapshots error: %w", err)
	}

	values.Hash, err = helpers.ComputeHash(inputs)
	if err != nil {
		return fmt.Errorf("cannot compute inputs hash: %w", err)
	}

	values.ProcessResult, err = process(input, inputs, &values.State)
	if err != nil {
		return fmt.Errorf("cannot process: %w", err)
	}

	moduleValues.Set(values)
	return nil
}

func process(input *go_hook.HookInput, inputs Inputs, state *State) (ProcessResult, error) {
	// TODO: this is stub code, need to write switch logic

	var result ProcessResult

	readyCondition := metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             ConditionReasonProcessing,
		ObservedGeneration: inputs.Params.Generation,
	}

	result.SetCondition(readyCondition)

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
			return result, fmt.Errorf("cannot process PKI: %w", err)
		}
	} else {
		state.PKI = nil
	}

	if secretsEnabled {
		if state.Secrets == nil {
			state.Secrets = &inputs.Secrets
		}

		if err := state.Secrets.Process(); err != nil {
			return result, fmt.Errorf("cannot process Secrets: %w", err)
		}
	} else {
		state.Secrets = nil
	}

	if usersParams.Any() {
		if state.Users == nil {
			state.Users = &users.State{}
		}

		if err := state.Users.Process(usersParams, inputs.Users); err != nil {
			return result, fmt.Errorf("cannot process Users: %w", err)
		}
	} else {
		state.Users = nil
	}

	state.Mode = params.Mode

	readyCondition.Status = metav1.ConditionTrue
	readyCondition.Reason = ""
	result.SetCondition(readyCondition)

	return result, nil
}
