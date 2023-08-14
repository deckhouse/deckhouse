// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bootstrap

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/maputil"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

// ConvertInitConfigurationToModuleConfigs turns InitConfiguration into a set of ModuleConfig's.
// At first, it creates a mandatory "deckhouse" ModuleConfig,
// then it checks for any module configuration overrides within InitConfiguration.configOverrides.
// If it detects such overrides, it converts them into ModuleConfig resources as well.
// Finally, it unlocks further bootstrap by allowing modules hooks to run with created ModuleConfig's.
func ConvertInitConfigurationToModuleConfigs(
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
) error {
	initConfiguration := metaConfig.DeckhouseConfig
	err := log.Process("bootstrap", "Converting InitConfiguration to a set of ModuleConfig's", func() error {
		// "deckhouse" ModuleConfig is mandatory.
		dhSettings := maputil.Filter(
			map[string]any{
				"bundle":         initConfiguration.Bundle,
				"logLevel":       initConfiguration.LogLevel,
				"registryCA":     initConfiguration.RegistryCA,
				"releaseChannel": initConfiguration.ReleaseChannel,
			},
			filterNonZeroValues[string, any],
		)
		dhModuleConfig := buildUnstructuredModuleConfigWithOverrides("deckhouse", true, dhSettings)
		if err := createModuleConfig(kubeCl.Dynamic(), dhModuleConfig); err != nil {
			return fmt.Errorf("create ModuleConfig: %w", err)
		}

		modulesEnabledStatuses := computeModuleEnabledStatuses(initConfiguration.ConfigOverrides)
		for moduleName, moduleEnabled := range modulesEnabledStatuses {
			configOverride := initConfiguration.ConfigOverrides[moduleName]
			settings := map[string]any{}
			if configOverride != nil {
				var settingsIsDict bool
				settings, settingsIsDict = configOverride.(map[string]any)
				if !settingsIsDict {
					return fmt.Errorf("invalid configOverride, expected a dictionary, got %T", configOverride)
				}
			}

			mc := buildUnstructuredModuleConfigWithOverrides(moduleName, moduleEnabled, settings)

			if err := createModuleConfig(kubeCl.Dynamic(), mc); err != nil {
				return fmt.Errorf("create ModuleConfig: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("InitConfiguration conversion failed: %w", err)
	}

	err = log.Process("bootstrap", "Unlock bootstrap process", func() error {
		if err := unlockBootstrapProcess(kubeCl); err != nil {
			return fmt.Errorf("unlockBootstrapProcess: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// computeModuleEnabledStatuses loops over a InitConfiguration.deckhouse.configOverrides structure,
// figuring out which ModuleConfig's should be enabled or disabled.
// Returns a mapping of module name and a if it should be enabled or not.
func computeModuleEnabledStatuses(configOverrides map[string]any) map[string]bool {
	modulesEnabledStatuses := make(map[string]bool)
	for key, override := range configOverrides {
		moduleName := strings.TrimSuffix(key, "Enabled")
		overrideIsEnableProperty := key != moduleName

		if overrideIsEnableProperty {
			enabled, isBool := override.(bool)
			if isBool {
				modulesEnabledStatuses[moduleName] = enabled
			} else {
				// Avoid enabling module if it's config is malformed
				modulesEnabledStatuses[moduleName] = false
			}
			continue
		}

		if _, enableStatusAlreadyDefined := modulesEnabledStatuses[moduleName]; !enableStatusAlreadyDefined {
			// Enabling module by default if it has no %moduleName%Enabled property, skipping otherwise
			modulesEnabledStatuses[moduleName] = true
		}
	}

	return modulesEnabledStatuses
}

// buildUnstructuredModuleConfigWithOverrides creates ModuleConfig object as unstructured.Unstructured.
func buildUnstructuredModuleConfigWithOverrides(moduleName string, isEnabled bool, settings map[string]any) *unstructured.Unstructured {
	moduleConfigName := strcase.ToKebab(moduleName)
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1alpha1",
		Kind:    "ModuleConfig",
	})
	mc.SetName(moduleConfigName)

	// Errors are impossible here.
	_ = unstructured.SetNestedField(mc.Object, isEnabled, "spec", "enabled")
	_ = unstructured.SetNestedField(mc.Object, int64(1), "spec", "version")
	if len(settings) > 0 {
		_ = unstructured.SetNestedMap(mc.Object, settings, "spec", "settings")
	}

	return mc
}

// createModuleConfig creates unstructured ModuleConfig within the cluster.
func createModuleConfig(kubeDynamicApi dynamic.Interface, mc *unstructured.Unstructured) error {
	moduleConfigName := mc.GetName()
	loop := retry.NewLoop(fmt.Sprintf("Create %q ModuleConfig", moduleConfigName), 15, time.Second*10)
	err := loop.Run(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := kubeDynamicApi.Resource(schema.GroupVersionResource{
			Group:    "deckhouse.io",
			Version:  "v1alpha1",
			Resource: "moduleconfigs",
		}).Create(ctx, mc, v1.CreateOptions{})
		return err
	})
	if err != nil {
		return fmt.Errorf("cannot create %q ModuleConfig: %w", moduleConfigName, err)
	}
	return nil
}

// unlockBootstrapProcess deletes deckhouse-bootstrap-lock ConfigMap that prevents module hooks from executing from d8-system namespace.
func unlockBootstrapProcess(kubeCl *client.KubernetesClient) error {
	loop := retry.NewSilentLoop("Unlock bootstrap process after ModuleConfig's creation", 25, time.Second*5)
	err := loop.Run(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		return kubeCl.CoreV1().
			ConfigMaps("d8-system").
			Delete(ctx, "deckhouse-bootstrap-lock", v1.DeleteOptions{})
	})
	if err != nil {
		return fmt.Errorf("cannot delete deckhouse-bootstrap-lock ConfigMap: %w", err)
	}

	return nil
}

func filterNonZeroValues[K comparable, V any](_ K, v V) bool {
	return !reflect.ValueOf(v).IsZero()
}
