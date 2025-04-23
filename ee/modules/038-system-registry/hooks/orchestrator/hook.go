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
					Scheme:     string(secret.Data["scheme"]),
					CA:         string(secret.Data["ca"]),
					Generation: secret.Generation,
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

	// TODO: extract ingress CA for local mode
	inputs.NodeServices, err = nodeservices.InputsFromSnapshot(input, nodeServicesSnapName)
	if err != nil {
		return fmt.Errorf("get NodeServices snapshots error: %w", err)
	}

	values.Hash, err = helpers.ComputeHash(inputs)
	if err != nil {
		return fmt.Errorf("cannot compute inputs hash: %w", err)
	}

	err = values.State.process(input.Logger, inputs)
	if err != nil {
		return fmt.Errorf("cannot process: %w", err)
	}

	moduleValues.Set(values)
	return nil
}
