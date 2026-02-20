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

package v1alpha1

import (
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ModulePackageVersionResource = "modulepackageversions"
	ModulePackageVersionKind     = "ModulePackageVersion"

	ModulePackageVersionLabelDraft           = PackageVersionLabelDraft
	ModulePackageVersionLabelPackage         = PackageVersionLabelPackage
	ModulePackageVersionLabelRepository      = PackageVersionLabelRepository
	ModulePackageVersionLabelExistInRegistry = PackageVersionLabelExistInRegistry

	ModulePackageVersionConditionTypeMetadataLoaded         = "MetadataLoaded"
	ModulePackageVersionConditionReasonFetchErr             = "FetchingReleaseError"
	ModulePackageVersionConditionReasonGetPackageRepoErr    = "GetPackageRepositoryError"
	ModulePackageVersionConditionReasonGetRegistryClientErr = "GetRegistryClientError"
	ModulePackageVersionConditionReasonGetImageErr          = "GetImageError"

	ModulePackageVersionFinalizer = "modulepackageversion.deckhouse.io/used-by-module"
)

var (
	ModulePackageVersionGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ModulePackageVersionResource,
	}
	ModulePackageVersionGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ModulePackageVersionKind,
	}
)

var _ runtime.Object = (*ModulePackageVersion)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=mpv
// +kubebuilder:printcolumn:name=Package,type=string,JSONPath=.spec.packageName
// +kubebuilder:printcolumn:name=Repository,type=string,JSONPath=.spec.packageRepositoryName
// +kubebuilder:printcolumn:name="TransitionTime",type="date",JSONPath=".status.conditions[?(@.type=='MetadataLoaded')].lastTransitionTime"
// +kubebuilder:printcolumn:name="MetadataLoaded",type="string",JSONPath=".status.conditions[?(@.type=='MetadataLoaded')].status"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='MetadataLoaded')].message"
// +kubebuilder:printcolumn:name="UsedBy",type=integer,JSONPath=`.status.usedByCount`

// ModulePackageVersion represents a version of a module package.
type ModulePackageVersion struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModulePackageVersionSpec `json:"spec,omitempty"`

	// Status of a ModulePackageVersion.
	Status ModulePackageVersionStatus `json:"status,omitempty"`
}

type ModulePackageVersionSpec struct {
	// Name of the module package.
	// +optional
	// +kubebuilder:validation:Immutable
	PackageName string `json:"packageName,omitempty"`

	// The name of the repository containing the package.
	// +optional
	// +kubebuilder:validation:Immutable
	PackageRepositoryName string `json:"packageRepositoryName,omitempty"`

	// Version of the module package.
	// +optional
	// +kubebuilder:validation:Immutable
	PackageVersion string `json:"packageVersion,omitempty"`
}

type ModulePackageVersionStatus struct {
	// Metadata about the package such as description, requirements, etc.
	// +optional
	PackageMetadata *ModulePackageVersionStatusMetadata `json:"packageMetadata,omitempty"`

	// Conditions represent the latest available observations of the package version's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Information about modules that are using this package version.
	// +optional
	UsedBy []ModulePackageVersionStatusInstance `json:"usedBy,omitempty"`

	// Number of modules using this package version.
	// +optional
	UsedByCount int `json:"usedByCount,omitempty"`
}

type ModulePackageVersionStatusInstance struct {
	// Namespace where the module is installed.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the module instance.
	// +optional
	Name string `json:"name,omitempty"`
}

type ModulePackageVersionStatusMetadata struct {
	// Localized descriptions of the package.
	// +optional
	Description *PackageDescription `json:"description,omitempty"`

	// The category this package belongs to.
	// +optional
	Category string `json:"category,omitempty"`

	// The development stage of the package (e.g., alpha, beta, stable).
	// +optional
	Stage string `json:"stage,omitempty"`

	// The system requirements for this package.
	// +optional
	Requirements *PackageRequirements `json:"requirements,omitempty"`

	// Licensing information for different editions.
	// +optional
	Licensing *PackageLicensing `json:"licensing,omitempty"`

	// Information about changes in this version.
	// +optional
	Changelog *PackageChangelog `json:"changelog,omitempty"`

	// Version compatibility rules for upgrades and downgrades.
	// +optional
	Compatibility *PackageVersionCompatibilityRules `json:"versionCompatibilityRules,omitempty"`
}

// IsDraft checks if this package version is marked as a draft.
func (m *ModulePackageVersion) IsDraft() bool {
	val, ok := m.Labels[ModulePackageVersionLabelDraft]
	if ok && val == "true" {
		return true
	}

	return false
}

// IsModuleInstalled checks if a specific module is installed using this package version.
func (m *ModulePackageVersion) IsModuleInstalled(namespace string, moduleName string) bool {
	if len(m.Status.UsedBy) == 0 {
		return false
	}

	for _, v := range m.Status.UsedBy {
		if v.Namespace == namespace && v.Name == moduleName {
			return true
		}
	}

	return false
}

// AddInstalledModule adds a module to the list of modules using this package version.
func (m *ModulePackageVersion) AddInstalledModule(namespace string, moduleName string) *ModulePackageVersion {
	moduleStatusInstance := ModulePackageVersionStatusInstance{Namespace: namespace, Name: moduleName}

	m.Status.UsedBy = append(m.Status.UsedBy, moduleStatusInstance)

	m.Status.UsedByCount++

	return m
}

// RemoveInstalledModule removes a module from the list of modules using this package version.
func (m *ModulePackageVersion) RemoveInstalledModule(namespace string, moduleName string) *ModulePackageVersion {
	prevLen := len(m.Status.UsedBy)
	m.Status.UsedBy = slices.DeleteFunc(m.Status.UsedBy, func(v ModulePackageVersionStatusInstance) bool {
		return v.Namespace == namespace && v.Name == moduleName
	})

	if len(m.Status.UsedBy) < prevLen && m.Status.UsedByCount > 0 {
		m.Status.UsedByCount--
	}

	return m
}

// +kubebuilder:object:root=true

// ModulePackageVersionList is a list of ModulePackageVersion resources
type ModulePackageVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModulePackageVersion `json:"items"`
}

// MakeModulePackageVersionName returns a name following the format <repository>-<packageName>-<version>
func MakeModulePackageVersionName(repository, packageName, version string) string {
	return repository + "-" + packageName + "-" + version
}
