/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package secrets

import (
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers/submodule"
	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

const (
	snapName      = "secrets"
	SubmoduleName = "secrets"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        fmt.Sprintf("/modules/system-registry/submodule-%s", SubmoduleName),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              snapName,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: helpers.NamespaceSelector,
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					"registry-secrets",
				},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var secret v1core.Secret

				err := sdk.FromUnstructured(obj, &secret)
				if err != nil {
					return "", fmt.Errorf("failed to convert secret \"%v\" to struct: %v", obj.GetName(), err)
				}

				ret := State{
					HTTP: string(secret.Data["http"]),
				}

				return ret, nil
			},
		},
	},
}, func(input *go_hook.HookInput) error {
	moduleConfig := submodule.NewConfigAccessor[any](input, SubmoduleName)
	moduleState := submodule.NewStateAccessor[State](input, SubmoduleName)

	config := moduleConfig.Get()

	if !config.Enabled {
		moduleState.Clear()
		return nil
	}

	var err error

	state := moduleState.Get()
	state.Version = config.Version

	secretData, _ := helpers.SnapshotToSingle[State](input, snapName)

	if state.Hash, err = helpers.ComputeHash(secretData); err != nil {
		return fmt.Errorf("cannot compute data hash: %w", err)
	}

	data := state.Data

	if strings.TrimSpace(data.HTTP) == "" {
		data.HTTP = secretData.HTTP
	}

	if strings.TrimSpace(data.HTTP) == "" {
		if randomValue, err := pki.GenerateRandomSecret(); err == nil {
			data.HTTP = randomValue
		} else {
			return fmt.Errorf("cannot generate HTTP secret: %w", err)
		}
	}

	state.Data = data
	state.Ready = true
	moduleState.Set(state)
	return nil
})

type State struct {
	HTTP string `json:"http,omitempty"`
}

type Params struct {
}
