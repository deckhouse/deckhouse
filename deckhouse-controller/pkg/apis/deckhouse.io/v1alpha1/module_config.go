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
// +kubebuilder:pruning:XPreserveUnknownFields
type SettingsValues runtime.RawExtension // map[string]any

// MarshalJSON implements json.Marshaler
func (v SettingsValues) MarshalJSON() ([]byte, error) {
	if v.Raw != nil {
		return v.Raw, nil
	}
	return []byte("{}"), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (v *SettingsValues) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	v.Raw = make([]byte, len(data))
	copy(v.Raw, data)
	return nil
}

func (v *SettingsValues) DeepCopy() *SettingsValues {
	if v == nil {
		return nil
	}
	out := new(SettingsValues)
	v.DeepCopyInto(out)
	return out
}

func (v *SettingsValues) DeepCopyInto(out *SettingsValues) {
	if v.Raw != nil {
		out.Raw = make([]byte, len(v.Raw))
		copy(out.Raw, v.Raw)
	} else {
		out.Raw = nil
	}
	if v.Object != nil {
		out.Object = v.Object.DeepCopyObject()
	} else {
		out.Object = nil
	}
}

type ModuleConfigSpec struct {
	Version      int    `json:"version,omitempty"`
	Enabled      *bool  `json:"enabled,omitempty"`
	UpdatePolicy string `json:"updatePolicy,omitempty"`
	Source       string `json:"source,omitempty"`
	Maintenance  string `json:"maintenance,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Settings *SettingsValues `json:"settings,omitempty"`
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
