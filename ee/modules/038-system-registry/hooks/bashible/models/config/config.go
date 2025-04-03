/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

import (
	"errors"
	"fmt"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/helpers"
	registry_models "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const (
	ConfigSpecLocation = "systemRegistry.internal.bashible.config"
)

type ConfigModel registry_models.BashibleConfigSecret

func Get(input *go_hook.HookInput) (*ConfigModel, error) {
	var ret ConfigModel
	err := helpers.UnmarshalInputValue(input, ConfigSpecLocation, &ret)
	if errors.Is(err, helpers.InputValueNotExist) {
		return nil, nil
	}
	return &ret, err
}

func Remove(input *go_hook.HookInput) {
	obj := input.Values.Get(ConfigSpecLocation)

	if obj.Exists() {
		input.Values.Remove(ConfigSpecLocation)
	}
}

func Set(input *go_hook.HookInput, cfg ConfigModel) {
	input.Values.Set(ConfigSpecLocation, cfg)
}

func SetIfNotExist(input *go_hook.HookInput, cfg ConfigModel) {
	obj := input.Values.Get(ConfigSpecLocation)

	if !obj.Exists() {
		input.Values.Set(ConfigSpecLocation, cfg)
	}
}

func FilterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ret v1core.Secret
	if err := sdk.FromUnstructured(obj, &ret); err != nil {
		return nil, fmt.Errorf("failed to convert %s to secret struct: %w", obj.GetName(), err)
	}
	return ret, nil
}

func ExtractFromSnapSecret(snaps []go_hook.FilterResult) (*ConfigModel, error) {
	if len(snaps) == 0 {
		return nil, nil
	}
	if snaps[0] == nil {
		return nil, nil
	}
	secret := snaps[0].(v1core.Secret)

	// Check if field is empty
	rawConfig := secret.Data["config"]
	if len(rawConfig) == 0 {
		return nil, nil
	}

	var ret ConfigModel
	if err := yaml.Unmarshal(rawConfig, ret); err != nil {
		return nil, fmt.Errorf("failed to parse registry bashible config secret: %w", err)
	}
	return &ret, nil
}
