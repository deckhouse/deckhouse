/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package users

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
	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/users"
)

const (
	snapName      = "user-secrets"
	SubmoduleName = "users"
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
					userSecretName("ro"),
					userSecretName("rw"),
					userSecretName("mirror-puller"),
					userSecretName("mirror-pusher"),
				},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var secret v1core.Secret

				err := sdk.FromUnstructured(obj, &secret)
				if err != nil {
					return "", fmt.Errorf("failed to convert secret \"%v\" to struct: %v", obj.GetName(), err)
				}

				if !strings.HasPrefix(secret.Name, userSecretNamePrefix) {
					return nil, nil
				}

				var user users.User
				err = user.DecodeSecretData(secret.Data)
				if err != nil {
					return nil, nil
				}

				ret := helpers.NewKeyValue(secret.Name, user)
				return ret, nil
			},
		},
	},
}, func(input *go_hook.HookInput) error {
	moduleState := submodule.NewStateAccessor[State](input, SubmoduleName)
	moduleConfig := submodule.NewConfigAccessor[Params](input, SubmoduleName)

	state := moduleState.Get()
	config := moduleConfig.Get()

	if !config.Enabled {
		moduleState.Clear()
		return nil
	}

	inputs, err := helpers.SnapshotToMap[string, users.User](input, snapName)
	if err != nil {
		return fmt.Errorf("canot get users from secrets: %w", err)
	}

	state.Hash, err = helpers.ComputeHash(moduleConfig, inputs)
	if err != nil {
		return fmt.Errorf("cannot compute hash: %w", err)
	}

	err = state.Data.Process(config.Params, inputs)
	if err != nil {
		return fmt.Errorf("cannot process users: %w", err)
	}

	state.Version = config.Version
	state.Ready = true

	moduleState.Set(state)

	return nil
})
