/*
Copyright 2024 Flant JSC

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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

/* Migration:
This migration implements global hook which migrate `storageClass` to `modules.storageClass` in `global` ModuleConfig.
If `global.storageClass` doesn't exist, migration skipped.
If `global.modules.storageClass` exists, migration skipped.
*/

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 20},
}, dependency.WithExternalDependencies(globalStorageClassMigrate))

func globalStorageClassMigrate(input *go_hook.HookInput, dc dependency.Container) error {
	const globalModuleName = "global"

	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	globalModuleConfigs, err := kubeCl.Dynamic().Resource(config.ModuleConfigGVR).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.name=" + globalModuleName})
	if errors.IsNotFound(err) || len(globalModuleConfigs.Items) == 0 {
		input.LogEntry.Info("`global` ModuleConfig does not exist, skipping migration")
		return nil
	}

	if err != nil {
		return err
	}

	globalModuleConfig := globalModuleConfigs.Items[0]

	globalStorageClass, globalConfigExists, err := unstructured.NestedString(globalModuleConfig.UnstructuredContent(), "spec", "settings", "storageClass")
	if err != nil {
		return err
	}

	if !globalConfigExists {
		input.LogEntry.Info("Property `global.storageClass` does not exist, skipping migration")
		return nil
	}

	_, modulesConfigExists, err := unstructured.NestedString(globalModuleConfig.UnstructuredContent(), "spec", "settings", "modules", "storageClass")
	if err != nil {
		return err
	}

	if modulesConfigExists {
		input.LogEntry.Info("Property `global.modules.storageClass` already exists. Just remove `global.storageClass` and skipping migration")

		patch := map[string]any{
			"spec": map[string]any{
				"settings": map[string]any{
					"storageClass": nil,
				},
			},
		}

		input.PatchCollector.MergePatch(patch, config.ModuleConfigGroup+"/"+config.ModuleConfigVersion, config.ModuleConfigKind, "", globalModuleName)

		return nil
	}

	// move `global.storageClass` to `global.modules.storageClass`
	patch := map[string]any{
		"spec": map[string]any{
			"settings": map[string]any{
				"storageClass": nil,
				"modules": map[string]any{
					"storageClass": globalStorageClass,
				},
			},
		},
	}

	input.LogEntry.Warn("Move `global.storageClass` to `global.modules.storageClass`")

	input.PatchCollector.MergePatch(patch, config.ModuleConfigGroup+"/"+config.ModuleConfigVersion, config.ModuleConfigKind, "", globalModuleName)

	return nil
}
