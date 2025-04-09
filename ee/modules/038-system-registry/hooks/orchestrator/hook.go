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
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/users"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	configSnapName = "config"
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

				config := registryConfig{
					Mode:       string(secret.Data["mode"]),
					ImagesRepo: string(secret.Data["imagesRepo"]),
					UserName:   string(secret.Data["username"]),
					Password:   string(secret.Data["password"]),
					TTL:        string(secret.Data["ttl"]),
				}

				return config, nil
			},
		},
	},
}, func(input *go_hook.HookInput) error {
	config, err := helpers.SnapshotToSingle[registryConfig](input, configSnapName)
	if err != nil {
		return fmt.Errorf("cannot get registry config: %w", err)
	}

	var (
		usersParams  users.Params
		usersVersion string
	)

	mode := registry_const.ToModeType(config.Mode)
	switch mode {
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
			return fmt.Errorf("cannot set users params: %w", err)
		}
	} else {
		submodule.DisableSubmodule(input, "users")
		usersVersion = "disabled"
	}

	log.Warn(
		"Params set",
		"mode", config.Mode,
		"config", config,
		"users_params", usersParams,
		"users_version", usersVersion,
	)

	return nil
})
