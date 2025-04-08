/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bashible

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/helpers"
	common_models "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/models"
	bashible_input "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/models/input"
	hooks_helpers "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
)

func BashibleInputHook(order float64, queue string) bool {
	const (
		snapCA     = "CA"
		snapUserRO = "userRO"
	)

	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: order},
		Queue:        queue,
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       snapCA,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"registry-pki"},
				},
				NamespaceSelector: helpers.NamespaceSelector,
				FilterFunc:        common_models.FilterCertModelSecret("registry-ca"),
			},
			{
				Name:       snapUserRO,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"registry-user-ro"},
				},
				NamespaceSelector: helpers.NamespaceSelector,
				FilterFunc:        common_models.FilterUserSecret,
			},
		},
	}, func(hookInput *go_hook.HookInput) error {
		// Get mode from moduleConfig
		mode := hooks_helpers.GetMode(hookInput)
		bashibleInputModel := bashible_input.InputModel{
			Mode: mode,
		}
		CA := common_models.ExtractFromSnapCertModel(hookInput.Snapshots[snapCA])
		User := common_models.ExtractFromSnapUserModel(hookInput.Snapshots[snapUserRO])

		switch mode {
		case registry_const.ModeProxy:
			if CA == nil || User == nil {
				// User or PKI are missing.
				// We can't return an error here, as it might indicate that the manager hasn't fully started yet.
				// Returning an error could prevent the manager from starting.
				// Instead, attempt to restore the current configuration if it exists.
				hookInput.Logger.Warn("Registry user secrets or registry PKI secrets are missing for registry mode \"%s\"", mode)
				return nil
			}
			bashibleInputModel.Proxy = &bashible_input.ProxyInputModel{
				CA:   *CA,
				User: *User,
			}
		case registry_const.ModeDetached:
			if CA == nil || User == nil {
				// User or PKI are missing.
				// We can't return an error here, as it might indicate that the manager hasn't fully started yet.
				// Returning an error could prevent the manager from starting.
				// Instead, attempt to restore the current configuration if it exists.
				hookInput.Logger.Warn("Registry user secrets or registry PKI secrets are missing for registry mode \"%s\"", mode)
				return nil
			}
			bashibleInputModel.Detached = &bashible_input.DetachedInputModel{
				CA:   *CA,
				User: *User,
			}
		}

		// Generate a version hash for the bashible input model
		hash, err := hashStruct(bashibleInputModel)
		if err != nil {
			return err
		}
		bashibleInputModel.Version = hash

		// Save the generated bashible input model
		bashible_input.Set(hookInput, bashibleInputModel)
		return nil
	})
}

func hashStruct(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return computeHash(data), nil
}

func computeHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
