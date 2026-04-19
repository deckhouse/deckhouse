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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/libapi"
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

var _ runtime.Object = (*ModulePullOverride)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

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
	ImageTag     string          `json:"imageTag"`
	ScanInterval libapi.Duration `json:"scanInterval"`
	Rollback     bool            `json:"rollback"`
}

type ModulePullOverrideStatus struct {
	UpdatedAt   metav1.Time `json:"updatedAt"`
	Message     string      `json:"message"`
	ImageDigest string      `json:"imageDigest"`
	Weight      uint32      `json:"weight"`
}

// GetModuleName returns the module's name of the module pull override
func (mo *ModulePullOverride) GetModuleName() string {
	return mo.Name
}

// GetReleaseVersion returns the version of the related module
func (mo *ModulePullOverride) GetReleaseVersion() string {
	return mo.Spec.ImageTag
}

// GetWeight returns the weight of the module
func (mo *ModulePullOverride) GetWeight() uint32 {
	return mo.Status.Weight
}

// +kubebuilder:object:root=true

// ModulePullOverrideList is a list of ModulePullOverride resources
type ModulePullOverrideList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModulePullOverride `json:"items"`
}
