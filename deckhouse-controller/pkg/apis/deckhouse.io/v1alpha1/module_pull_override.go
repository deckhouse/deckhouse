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
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	ModulePullOverrideGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: "modulepulloverrides",
	}
	ModulePullOverrideGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    "ModulePullOverride",
	}
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModulePullOverride object
type ModulePullOverride struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of an ModulePullOverride.
	Spec ModulePullOverrideSpec `json:"spec"`

	// Status of an ModulePullOverride.
	Status ModulePullOverrideStatus `json:"status,omitempty"`
}

type ModulePullOverrideSpec struct {
	Source       string   `json:"source"`
	ImageTag     string   `json:"imageTag"`
	ScanInterval Duration `json:"scanInterval"`
}

type ModulePullOverrideStatus struct {
	UpdatedAt   metav1.Time `json:"updatedAt"`
	Message     string      `json:"message"`
	ImageDigest string      `json:"imageDigest"`
}

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModulePullOverrideList is a list of ModulePullOverride resources
type ModulePullOverrideList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModulePullOverride `json:"items"`
}

type ModulePullOverrideKind struct{}

func (in *ModulePullOverrideStatus) GetObjectKind() schema.ObjectKind {
	return &ModulePullOverrideKind{}
}

func (f *ModulePullOverrideKind) SetGroupVersionKind(_ schema.GroupVersionKind) {}
func (f *ModulePullOverrideKind) GroupVersionKind() schema.GroupVersionKind {
	return ModulePullOverrideGVK
}

// GetModuleSource returns module source for module pull override
func (mo *ModulePullOverride) GetModuleSource() string {
	return mo.Spec.Source
}

// GetModuleName returns the module's name of the module pull override
func (mo *ModulePullOverride) GetModuleName() string {
	return mo.Name
}

// GetReleaseVersion returns the version of the module pull override ("dev")
func (mo *ModulePullOverride) GetReleaseVersion() string {
	return mo.Spec.ImageTag
}
