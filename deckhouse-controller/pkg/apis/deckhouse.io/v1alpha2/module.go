/*
Copyright 2025 Flant JSC

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

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

const (
	ModuleResource = "modules"
	ModuleKind     = "Module"

	// ModuleConditionTypeCompleted changes only by module controller
	ModuleConditionTypeCompleted                = "Completed"
	ModuleConditionReasonVersionNotFound        = "VersionNotFound"
	ModuleConditionReasonModulePackageNotFound  = "ModulePackageNotFound"
	ModuleConditionReasonVersionIsDraft         = "VersionIsDraft"
	ModuleConditionReasonVersionSpecIsCorrupted = "VersionSpecIsCorrupted"

	ModuleFinalizerStatisticRegistered = "module.deckhouse.io/statistic-registered"

	ModuleAnnotationRegistrySpecChanged = "packages.deckhouse.io/registry-spec-changed"
)

var (
	ModuleGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ModuleResource,
	}
	ModuleGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ModuleKind,
	}
)

var _ runtime.Object = (*Module)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name=Package,type=string,JSONPath=.spec.packageName
// +kubebuilder:printcolumn:name=Version,type=string,JSONPath=.spec.packageVersion
// +kubebuilder:printcolumn:name=Repository,type=string,JSONPath=.spec.packageRepositoryName,priority=1
// +kubebuilder:printcolumn:name=Installed,type=string,JSONPath=.status.conditions[?(@.type=='Installed')].status
// +kubebuilder:printcolumn:name=Ready,type=string,JSONPath=.status.conditions[?(@.type=='Ready')].status
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +kubebuilder:printcolumn:name=Age,type=date,JSONPath=.metadata.creationTimestamp

// Module represents a module instance managed via the package system.
type Module struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a Module.
	Spec ModuleSpec `json:"spec"`

	// Status of a Module.
	Status ModuleStatus `json:"status,omitempty"`
}

type ModuleSpec struct {
	// Name of the module package to install.
	PackageName string `json:"packageName"`

	// Name of the repository where the package is located.
	// If not specified, the default repository is used.
	// +optional
	PackageRepositoryName string `json:"packageRepositoryName,omitempty"`

	// Version of the module package to install.
	PackageVersion string `json:"packageVersion"`

	// Release channel for the module package.
	// +optional
	ReleaseChannel string `json:"releaseChannel,omitempty"`

	// Configuration settings for the module.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Settings *v1alpha1.MappedFields `json:"settings,omitempty"`
}

type ModuleStatus struct {
	// Information about the currently installed version.
	// +optional
	CurrentVersion *ModuleStatusVersion `json:"currentVersion,omitempty"`

	// Conditions represent the latest available observations of the module's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// InternalConditions represent internal conditions of the module.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	InternalConditions []metav1.Condition `json:"internalConditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ResourceConditions represent conditions related to module resources.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	ResourceConditions []metav1.Condition `json:"resourceConditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type ModuleStatusVersion struct {
	// Semantic version of the installed module.
	// +optional
	Version string `json:"version,omitempty"`

	// Release channel from which the version was installed.
	// +optional
	Channel string `json:"channel,omitempty"`
}

// +kubebuilder:object:root=true

// ModuleList is a list of Module resources
type ModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Module `json:"items"`
}
