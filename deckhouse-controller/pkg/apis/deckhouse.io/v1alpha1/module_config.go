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

var (
	// ModuleConfigGVR GroupVersionResource
	ModuleConfigGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: "moduleconfigs",
	}
	ModuleConfigGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    "ModuleConfig",
	}
)

var _ runtime.Object = (*ModuleConfig)(nil)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
type SettingsValues map[string]interface{}

func (v *SettingsValues) DeepCopy() *SettingsValues {
	nmap := make(map[string]interface{}, len(*v))

	for key, value := range *v {
		nmap[key] = value
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

type ModuleConfigSpec struct {
	Version  int            `json:"version,omitempty"`
	Settings SettingsValues `json:"settings,omitempty"`
	Enabled  *bool          `json:"enabled,omitempty"`
}

type ModuleConfigStatus struct {
	Version string `json:"version"`
	Message string `json:"message"`
}

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleConfigList is a list of ModuleConfig resources
type ModuleConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModuleConfig `json:"items"`
}

type moduleConfigKind struct{}

func (in *ModuleConfigStatus) GetObjectKind() schema.ObjectKind {
	return &moduleConfigKind{}
}

func (f *moduleConfigKind) SetGroupVersionKind(_ schema.GroupVersionKind) {}
func (f *moduleConfigKind) GroupVersionKind() schema.GroupVersionKind {
	return ModuleConfigGVK
}
