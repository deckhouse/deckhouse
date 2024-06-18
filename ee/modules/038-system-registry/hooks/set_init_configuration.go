/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	resources_v1 "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/internal/resources/v1"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"reflect"
	"k8s.io/utils/pointer"
)

const (
	initSecretName          = "system-registry-init-configuration"
	initSecretNamespace     = "d8-system"
	initSecretSnapshotsName = "init_secret_config"

	registryModuleConfigName          = "system-registry"
	registryModuleConfigSnapshotsName = "registry_module_config"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/system-registry/bootstrap_init_cfg",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       initSecretSnapshotsName,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{initSecretName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{initSecretNamespace},
				},
			},

			FilterFunc: applyInitSecretDataFilter,
		},
		{
			Name:       registryModuleConfigSnapshotsName,
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{registryModuleConfigName},
			},
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc: applyMCSettingsFilter,
		},
	},
}, setInitConfigurationToMC)

func applyMCSettingsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if obj == nil {
		return nil, nil
	}

	var moduleConfig v1alpha1.ModuleConfig
	err := sdk.FromUnstructured(obj, &moduleConfig)
	if err != nil {
		return nil, err
	}
	return moduleConfig, nil
}

func applyInitSecretDataFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if obj == nil {
		return nil, nil
	}

	var initSecretData resources_v1.InitSecretData
	err := sdk.FromUnstructured(obj, &initSecretData)
	if err != nil {
		return nil, err
	}
	return initSecretData, nil
}

func setInitConfigurationToMC(input *go_hook.HookInput) error {
	initSecretData, err := getInitSecretDataFromSnapshots(input)
	if err != nil {
		return err
	}
	moduleConfig, err := getMCSettingsFromSnapshots(input)
	if err != nil {
		return err
	}

	// If secret not exist or empty - nothing to do
	if initSecretData == nil || initSecretData.Data == nil {
		return nil
	}

	// If secret exist - create/update Module Config
	if moduleConfig != nil {
		err = updateMCWithInitSecretData(input, initSecretData, moduleConfig)
	} else {
		err = createMCWithInitSecretData(input, initSecretData)
	}
	if err != nil {
		return err
	}

	// Delete secret after
	deleteInitSecretData(input)
	return nil
}

func getInitSecretDataFromSnapshots(input *go_hook.HookInput) (*resources_v1.InitSecretData, error) {
	snap := input.Snapshots[initSecretSnapshotsName]
	if len(snap) == 0 {
		return nil, nil
	}
	if snap[0] == nil {
		return nil, nil
	}
	secretData, ok := snap[0].(resources_v1.InitSecretData)
	if !ok {
		return nil, fmt.Errorf("error converting secret '%s' to structure '%s'", initSecretName, reflect.TypeOf(secretData).Name())
	}
	return &secretData, nil
}

func getMCSettingsFromSnapshots(input *go_hook.HookInput) (*v1alpha1.ModuleConfig, error) {
	snap := input.Snapshots[registryModuleConfigSnapshotsName]
	if len(snap) == 0 {
		return nil, nil
	}
	if snap[0] == nil {
		return nil, nil
	}
	mcSettings, ok := snap[0].(v1alpha1.ModuleConfig)
	if !ok {
		return nil, fmt.Errorf("error converting module config '%s' to structure '%s'", initSecretName, reflect.TypeOf(mcSettings).Name())
	}
	return &mcSettings, nil
}

func createMCWithInitSecretData(input *go_hook.HookInput, initSecretData *resources_v1.InitSecretData) error {
	newModuleConfig, err := resources_v1.NewModuleConfigByInitSecret(initSecretData)
	if err != nil {
		return err
	}
	newModuleConfig.SetResourceVersion("")
	input.PatchCollector.Create(newModuleConfig, object_patch.UpdateIfExists())
	return nil
}

func updateMCWithInitSecretData(input *go_hook.HookInput, initSecretData *resources_v1.InitSecretData, moduleConfig *v1alpha1.ModuleConfig) error {
	err := resources_v1.PrepareModuleConfigByInitSettings(moduleConfig, initSecretData)
	if err != nil {
		return err
	}
	moduleConfig.SetResourceVersion("")
	input.PatchCollector.Create(moduleConfig, object_patch.UpdateIfExists())
	return nil
}

func deleteInitSecretData(input *go_hook.HookInput) {
	input.PatchCollector.Delete("v1", "Secret", initSecretNamespace, initSecretName, object_patch.InBackground())
}
