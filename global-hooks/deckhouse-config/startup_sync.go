/*
Copyright 2022 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	d8cfg_v1alpha1 "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

/**
This hook switches deckhouse-controller to use a number of typed ModuleConfig custom
resources and a managed ConfigMap/deckhouse-generated-config-do-not-edit object
instead of one untyped and unmanaged ConfigMap/deckhouse object.
*/

// Use order:1 to run before all global hooks.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 1},
}, dependency.WithExternalDependencies(migrateOrSyncModuleConfigs))

// migrateOrSyncModuleConfigs runs on deckhouse-controller startup
// as early as possible and do two things:
//   - migrates deckhouse-controller from configuration via ConfigMap/deckhouse
//     that is managed by deckhouse and by user to configuration via
//     ModuleConfig objects that managed by user so can be stored in Git.
//   - synchronize ModuleConfig objects content to intermediate
//     ConfigMap/deckhouse-generated-config-do-not-edit.
func migrateOrSyncModuleConfigs(input *go_hook.HookInput, dc dependency.Container) error {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	// Phase 1: "Deployment should use generated ConfigMap"
	// Migrate to generated ConfigMap:
	// - Create a copy of ConfigMap/deckhouse in ConfigMap/deckhouse-generated-config-do-not-edit.
	// - Add annotation deckhouse.io/should-create-deckhouse-configs to create ModuleConfig resources after restart.
	// - Update deploy/deckhouse to use new ConfigMap.
	// NOTE: deployment is migrated first to restart early and prevent losing log messages
	//       about creating ModuleConfig resources.
	addonOperatorCM := os.Getenv("ADDON_OPERATOR_CONFIG_MAP")
	if addonOperatorCM != d8config.GeneratedConfigMapName {
		input.LogEntry.Infof("Deployment/deckhouse uses cm/deckhouse. Copy data to cm/%s and update deployment/deckhouse to use it.", d8config.GeneratedConfigMapName)
		return migrateToGeneratedConfigMap(input, kubeClient)
		// Deckhouse will restart after applying patches.
	}

	// Phase 2: "Migrate ConfigMap to ModuleConfig objects".
	// If ConfigMap/deckhouse-generated-config-do-not-edit has annotation:
	// - Create ModuleConfig resources for each module section and for global section.
	// - Add patch to remove annotation from ConfigMap.
	hasGeneratedCM := true
	generatedCM, err := d8config.GetGeneratedConfigMap(kubeClient)
	if err != nil {
		if !k8errors.IsNotFound(err) {
			return fmt.Errorf("get generated ConfigMap: %v", err)
		}
		// NotFound error is occurred.
		hasGeneratedCM = false
	}
	if hasGeneratedCM {
		_, shouldMigrate := generatedCM.GetAnnotations()[d8config.AnnoMigrationInProgress]
		if shouldMigrate {
			input.LogEntry.Infof("Migrate Configmap to ModuleConfig resources.")
			return createInitialModuleConfigs(input, generatedCM.Data)
		}
	}

	// Phase 3: "Normal mode".
	// Sync existing ModuleConfig resources to ConfigMap/deckhouse-generated-config-do-not-edit.
	input.LogEntry.Infof("Sync ModuleConfig resources to generated ConfigMap.")
	allConfigs, err := d8config.GetAllConfigs(kubeClient)
	if err != nil {
		return fmt.Errorf("get all settings: %v", err)
	}
	input.LogEntry.Infof("Generated cm: %v, module configs: %d. Run sync.", hasGeneratedCM, len(allConfigs))
	return syncModuleConfigs(input, generatedCM, allConfigs)
}

// migrateToGeneratedConfigMap creates cm/deckhouse copy with an additional annotation
// and update deploy/deckhouse to use this new cm.
// Note: it creates an empty cm if cm/deckhouse is not found.
func migrateToGeneratedConfigMap(input *go_hook.HookInput, kubeClient k8s.Client) error {
	cm, err := d8config.GetDeckhouseConfigMap(kubeClient)
	if err != nil && !k8errors.IsNotFound(err) {
		return fmt.Errorf("get ConfigMap/%s: %v", d8config.DeckhouseConfigMapName, err)
	}

	data := map[string]string{}
	if cm != nil {
		data = cm.Data
	}

	newCm := d8config.GeneratedConfigMap(data)
	newCm.SetAnnotations(map[string]string{d8config.AnnoMigrationInProgress: "true"})

	input.PatchCollector.Create(newCm, object_patch.UpdateIfExists())

	modifyDeckhouseDeploymentToUseGeneratedConfigMap(input.PatchCollector, d8config.GeneratedConfigMapName)

	return nil
}

func createInitialModuleConfigs(input *go_hook.HookInput, cmData map[string]string) error {
	// Create ModuleConfig objects from ConfigMap data.
	configs, msgs, err := d8config.Service().Transformer().ConfigMapToModuleConfigList(cmData)
	if err != nil {
		return err
	}

	for _, msg := range msgs {
		input.LogEntry.Infof(msg)
	}

	properCfgs := make([]*d8cfg_v1alpha1.ModuleConfig, 0)

	for _, cfg := range configs {
		res := d8config.Service().ConfigValidator().ConvertToLatest(cfg)
		// Log conversion error and create ModuleConfig as-is.
		// Ignore this ModuleConfig when update generated ConfigMap.
		if res.HasError() {
			input.LogEntry.Errorf("Auto-created ModuleConfig/%s will be ignored. The module section in the generated ConfigMap is invalid: %v", cfg.GetName(), res.Error)
			continue
		}
		// Update spec.settings to converted settings.
		cfg.Spec.Settings = res.Settings
		cfg.Spec.Version = res.Version
		properCfgs = append(properCfgs, cfg)
	}

	for _, cfg := range configs {
		input.LogEntry.Infof("Creating ModuleConfig/%s", cfg.GetName())
		input.PatchCollector.Create(cfg, object_patch.UpdateIfExists())
	}

	// Recreate ConfigMap from ModuleConfig objects to clean-up deprecated module sections.
	newData, err := d8config.Service().Transformer().ModuleConfigListToConfigMap(properCfgs)
	if err != nil {
		return err
	}
	cm := d8config.GeneratedConfigMap(newData)
	input.LogEntry.Infof("Re-create ConfigMap/%s after migration", cm.Name)
	input.PatchCollector.Create(cm, object_patch.UpdateIfExists())

	return nil
}

// modifyDeckhouseDeploymentToUseGeneratedConfigMap patches container in deploy/deckhouse to use new generated ConfigMap for config values.
func modifyDeckhouseDeploymentToUseGeneratedConfigMap(patchCollector *object_patch.PatchCollector, cmName string) {
	modify := func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var depl appsv1.Deployment
		err := sdk.FromUnstructured(u, &depl)
		if err != nil {
			return nil, err
		}

		for i, container := range depl.Spec.Template.Spec.Containers {
			// Detect if container has ADDON_OPERATOR_CONFIG_MAP env
			// to ignore possible non-deckhouse containers.
			cmEnvIdx := -1
			for i, envVar := range container.Env {
				if envVar.Name == "ADDON_OPERATOR_CONFIG_MAP" {
					cmEnvIdx = i
				}
			}

			if cmEnvIdx >= 0 {
				depl.Spec.Template.Spec.Containers[i].Env[cmEnvIdx].Value = cmName
				break
			}
		}

		return sdk.ToUnstructured(&depl)
	}

	patchCollector.Filter(modify, "apps/v1", "Deployment", d8config.DeckhouseNS, "deckhouse")
}

// syncModuleConfigs updates generated ConfigMap using ModuleConfig resources.
func syncModuleConfigs(input *go_hook.HookInput, generatedCM *v1.ConfigMap, allConfigs []*d8cfg_v1alpha1.ModuleConfig) error {
	properCfgs := make([]*d8cfg_v1alpha1.ModuleConfig, 0)

	for _, cfg := range allConfigs {
		res := d8config.Service().ConfigValidator().Validate(cfg)
		// Conversion or validation error. Log error and ignore this ModuleConfig.
		if res.HasError() {
			input.LogEntry.Errorf("Invalid ModuleConfig/%s will be ignored due to validation error: %v", cfg.GetName(), res.Error)
			continue
		}
		cfg.Spec.Settings = res.Settings
		cfg.Spec.Version = res.Version

		// Note: this message appears only on startup.
		input.LogEntry.Debugf("ModuleConfig/%s is valid", cfg.GetName())
		properCfgs = append(properCfgs, cfg)
	}

	cmData, err := d8config.Service().Transformer().ModuleConfigListToConfigMap(properCfgs)
	if err != nil {
		return err
	}
	regeneratedCM := d8config.GeneratedConfigMap(cmData)
	input.LogEntry.Infof("Re-creating Config/%s on sync", regeneratedCM.Name)
	input.PatchCollector.Create(regeneratedCM, object_patch.UpdateIfExists())

	// Return if source cm was empty.
	if generatedCM == nil || len(generatedCM.Data) == 0 {
		return nil
	}

	// Log deleted sections in source CM.
	fields := make([]string, 0)
	for name := range generatedCM.Data {
		if _, has := cmData[name]; !has {
			fields = append(fields, name)
		}
	}
	sort.Strings(fields)
	input.LogEntry.Warnf("Remove fields [%s] from cm/%s", strings.Join(fields, ", "), regeneratedCM.Name)

	return nil
}
