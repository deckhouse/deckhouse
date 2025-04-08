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
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers/submodule"
	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/users"
)

const (
	userSecretsSnap = "user-secrets"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/system-registry/users",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              userSecretsSnap,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: namespaceSelector,
			//ExecuteHookOnSynchronization: ptr.Bool(false),
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

				ret := helpers.KeyValue[string, users.User]{
					Key:   secret.Name,
					Value: user,
				}

				return ret, nil
			},
		},
	},
}, func(input *go_hook.HookInput) error {
	state := submodule.GetSubmoduleState[State](input, "users")
	config := submodule.GetSubmoduleConfig[Params](input, "users")

	if !config.Enabled {
		submodule.RemoveSubmoduleState(input, "users")
		return nil
	}

	secretUsers, err := helpers.SnapshotToMap[string, users.User](input, userSecretsSnap)
	if err != nil {
		return fmt.Errorf("canot get users from secrets: %w", err)
	}

	stateUsers := state.Data
	state.Data = make(State)

	hash, err := helpers.ComputeHash(config, secretUsers)
	if err != nil {
		return fmt.Errorf("cannot compute hash: %w", err)
	}

	state.Hash = hash
	state.Version = config.Version

	//TODO: remove
	input.Logger.Warn(
		"Users reconcile",
		"state",
		stateUsers, "secrets",
		secretUsers, "hash", state.Hash,
	)

	for _, name := range config.Params {
		if !isValidUserName(name) {
			return fmt.Errorf("user name \"%v\" is invalid", name)
		}

		key := userSecretName(name)

		user, ok := stateUsers[key]
		if !ok || !user.IsValid() {
			user, ok = secretUsers[key]
		}

		if !ok || !user.IsValid() {
			user = users.User{
				UserName: name,
			}

			if err := user.GenerateNewPassword(); err != nil {
				return fmt.Errorf("cannot generate user \"%v\" password: %w", name, err)
			}
		}

		if !user.IsPasswordHashValid() {
			if err := user.UpdatePasswordHash(); err != nil {
				return fmt.Errorf("cannot update user \"%v\" password hash: %w", name, err)
			}
		}

		state.Data[key] = user
	}

	submodule.SetSubmoduleState(input, "users", state)

	return nil
})

type Params []string
type State map[string]users.User
