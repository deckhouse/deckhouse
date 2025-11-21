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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ModuleConfigResource = "moduleconfigs"
	ModuleConfigKind     = "ModuleConfig"

	ModuleConfigAnnotationAllowDisable = "modules.deckhouse.io/allow-disabling"

	// TODO: remove after 1.73+
	ModuleConfigFinalizerOld = "modules.deckhouse.io/module-config"
	ModuleConfigFinalizer    = "modules.deckhouse.io/module-registered"

	ModuleConfigMessageUnknownModule = "Ignored: unknown module name"
)

var (
	// ModuleConfigGVR GroupVersionResource
	ModuleConfigGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ModuleConfigResource,
	}
	ModuleConfigGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ModuleConfigKind,
	}
)

var _ runtime.Object = (*ModuleConfig)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ModuleConfig is a configuration for module or for global config values.
type ModuleConfig struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleConfigSpec `json:"spec"`

	Status ModuleConfigStatus `json:"status,omitempty"`
}

// SettingsValues empty interface in needed to handle DeepCopy generation. DeepCopy does not work with unnamed empty interfaces
type SettingsValues map[string]any

func (v *SettingsValues) DeepCopy() *SettingsValues {
	nmap := make(map[string]any, len(*v))

	for key, value := range *v {
		nmap[key] = deepCopyValue(value)
	}

	vv := SettingsValues(nmap)

	return &vv
}

func (v SettingsValues) DeepCopyInto(out *SettingsValues) {
	{
		v := &v
		clone := v.DeepCopy()
		*out = *clone
		return
	}
}

func deepCopyValue(val any) any {
	switch v := val.(type) {
	case map[string]any:
		newMap := make(map[string]any, len(v))
		for k, vv := range v {
			newMap[k] = deepCopyValue(vv)
		}
		return newMap
	case []any:
		newSlice := make([]any, len(v))
		for i, vv := range v {
			newSlice[i] = deepCopyValue(vv)
		}
		return newSlice
	default:
		return v
	}
}

type ModuleConfigSpec struct {
	Version      int            `json:"version,omitempty"`
	Settings     SettingsValues `json:"settings,omitempty"`
	Enabled      *bool          `json:"enabled,omitempty"`
	UpdatePolicy string         `json:"updatePolicy,omitempty"`
	Source       string         `json:"source,omitempty"`
	Maintenance  string         `json:"maintenance,omitempty"`
}

type ModuleConfigStatus struct {
	Version string `json:"version"`
	Message string `json:"message"`
}

func (m *ModuleConfig) IsEnabled() bool {
	if m.Spec.Enabled != nil {
		return *m.Spec.Enabled
	}
	return false
}

// +kubebuilder:object:root=true

// ModuleConfigList is a list of ModuleConfig resources
type ModuleConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModuleConfig `json:"items"`
}
