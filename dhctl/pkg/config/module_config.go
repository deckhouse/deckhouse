/*
Copyright 2023 Flant JSC

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

package config

import (
	"github.com/iancoleman/strcase"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

const (
	ModuleConfigKind     = "ModuleConfig"
	ModuleConfigResource = "moduleconfigs"
	ModuleConfigGroup    = "deckhouse.io"
	ModuleConfigVersion  = "v1alpha1"
)

var (
	// ModuleConfigGVR GroupVersionResource
	ModuleConfigGVR = schema.GroupVersionResource{
		Group:    ModuleConfigGroup,
		Version:  ModuleConfigVersion,
		Resource: ModuleConfigResource,
	}
)

// ModuleConfig is a configuration for module or for global config values.
type ModuleConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleConfigSpec `json:"spec"`
}

// SettingsValues empty interface in needed to handle DeepCopy generation. DeepCopy does not work with unnamed empty interfaces
type SettingsValues map[string]interface{}

type ModuleConfigSpec struct {
	Version  int            `json:"version,omitempty"`
	Settings SettingsValues `json:"settings,omitempty"`
	Enabled  *bool          `json:"enabled,omitempty"`
}

func buildModuleConfig(
	schemasStore *SchemaStore,
	moduleName string,
	isEnabled bool,
	settings map[string]any,
) (*ModuleConfig, error) {
	mc := &ModuleConfig{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   ModuleConfigGroup,
		Version: ModuleConfigVersion,
		Kind:    ModuleConfigKind,
	})

	moduleConfigName := strcase.ToKebab(moduleName)

	mc.SetName(moduleConfigName)

	v := schemasStore.GetModuleConfigVersion(moduleConfigName)

	mc.Spec.Enabled = &isEnabled
	mc.Spec.Version = v
	if len(settings) > 0 {
		mc.Spec.Settings = settings
	}

	doc, err := yaml.Marshal(mc)
	if err != nil {
		return nil, err
	}

	_, err = schemasStore.Validate(&doc)
	if err != nil {
		return nil, err
	}

	return mc, nil
}
