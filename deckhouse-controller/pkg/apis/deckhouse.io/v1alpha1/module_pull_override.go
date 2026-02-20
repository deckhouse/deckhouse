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

	"github.com/deckhouse/deckhouse/go_lib/libapi"
)

const (
	ModulePullOverrideAnnotationDeployedOn = "modules.deckhouse.io/deployed-on"
	ModulePullOverrideFinalizer            = "modules.deckhouse.io/mpo-finalizer"

	ModulePullOverrideMessageReady          = "Ready"
	ModulePullOverrideMessageModuleEmbedded = "The module is embedded"
	ModulePullOverrideMessageModuleDisabled = "The module disabled"
	ModulePullOverrideMessageModuleNotFound = "The module not found"
	ModulePullOverrideMessageSourceNotFound = "The source not found"
	ModulePullOverrideMessageNoSource       = "The module does not have an active source"

	ModulePullOverrideAnnotationRenew = "renew"
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
// +kubebuilder:resource:scope=Cluster,shortName=mpo
// +kubebuilder:printcolumn:name="Updated",type="date",JSONPath=".status.updatedAt",format="date-time",description="When the module was last updated."
// +kubebuilder:printcolumn:name="msg",type="string",JSONPath=".status.message",description="Detailed description."

// ModulePullOverride defines the configuration.
type ModulePullOverride struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a ModulePullOverride.
	// +kubebuilder:validation:Required
	Spec ModulePullOverrideSpec `json:"spec"`

	// Status of a ModulePullOverride.
	// +optional
	Status ModulePullOverrideStatus `json:"status,omitempty"`
}

// ModulePullOverrideSpec defines the desired state of ModulePullOverride
type ModulePullOverrideSpec struct {
	// Reference to the ModuleSource with the module.
	// +optional
	Source string `json:"source,omitempty"`

	// Module container image tag, which will be pulled.
	// +kubebuilder:validation:Required
	ImageTag string `json:"imageTag"`

	// Scan interval for checking the image digest. If the digest changes, the module is updated.
	// +kubebuilder:default="15s"
	// +optional
	ScanInterval libapi.Duration `json:"scanInterval,omitempty"`

	// Indicates whether the module release should be rollback after deleting mpo.
	// +kubebuilder:default=false
	// +optional
	Rollback bool `json:"rollback,omitempty"`
}

// ModulePullOverrideStatus defines the observed state of ModulePullOverride
type ModulePullOverrideStatus struct {
	// When the module was last updated.
	// +optional
	UpdatedAt metav1.Time `json:"updatedAt,omitempty"`

	// Details of the resource status.
	// +optional
	Message string `json:"message,omitempty"`

	// Digest of the module image.
	// +optional
	ImageDigest string `json:"imageDigest,omitempty"`

	// Module weight.
	// +optional
	Weight uint16 `json:"weight,omitempty"`
}

// GetModuleSource returns the module source of the related module
func (mo *ModulePullOverride) GetModuleSource() string {
	return mo.Spec.Source
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
func (mo *ModulePullOverride) GetWeight() int {
	return mo.Status.Weight
}

// +kubebuilder:object:root=true

// ModulePullOverrideList is a list of ModulePullOverride resources
type ModulePullOverrideList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModulePullOverride `json:"items"`
}
