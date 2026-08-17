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
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// Resource and kind names of ModulePackageVersion.
	ModulePackageVersionResource = "modulepackageversions"
	ModulePackageVersionKind     = "ModulePackageVersion"

	// Labels carrying the version's origin and lifecycle state.
	ModulePackageVersionLabelLegacy          = "packages.deckhouse.io/legacy"
	ModulePackageVersionLabelDraft           = "packages.deckhouse.io/draft"
	ModulePackageVersionLabelPackage         = "packages.deckhouse.io/package"
	ModulePackageVersionLabelRepository      = "packages.deckhouse.io/repository"
	ModulePackageVersionLabelExistInRegistry = "packages.deckhouse.io/exist-in-registry"

	// Condition type and the reasons reported when metadata loading fails.
	ModulePackageVersionConditionTypeMetadataLoaded         = "MetadataLoaded"
	ModulePackageVersionConditionReasonFetchErr             = "FetchingReleaseError"
	ModulePackageVersionConditionReasonGetPackageRepoErr    = "GetPackageRepositoryError"
	ModulePackageVersionConditionReasonGetRegistryClientErr = "GetRegistryClientError"
	ModulePackageVersionConditionReasonGetImageErr          = "GetImageError"

	// Finalizer blocking deletion while any module still uses the version.
	ModulePackageVersionFinalizer = "modulepackageversion.deckhouse.io/used-by-module"
)

// Group-version identifiers of ModulePackageVersion.
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
// +kubebuilder:printcolumn:name="MetadataLoaded",type="string",JSONPath=".status.conditions[?(@.type=='MetadataLoaded')].status"
// +kubebuilder:printcolumn:name="Used",type=boolean,JSONPath=`.status.used`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="TransitionTime",type="date",priority=1,JSONPath=".status.conditions[?(@.type=='MetadataLoaded')].lastTransitionTime"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.conditions[?(@.type=='MetadataLoaded')].message"
// +crd-enricher:raw:properties.apiVersion.description="APIVersion defines the versioned schema of this representation of an object.\nServers should convert recognized schemas to the latest internal value, and\nmay reject unrecognized values.\n\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)"
// +crd-enricher:raw:properties.kind.description="Kind is a string value representing the REST resource this object represents.\nServers may infer this from the endpoint the client submits requests to.\nCannot be updated.\nIn CamelCase.\n\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds)"

// +crd-enricher:deckhouse:documentation:examples={apiVersion: deckhouse.io/v1alpha1, kind: ModulePackageVersion, metadata: {name: example}, spec: {packageName: sds-node-configurator, packageRepositoryName: deckhouse, packageVersion: v1.0.0}}
// ModulePackageVersion represents a version of a module package.
type ModulePackageVersion struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the module package version parameters.
	Spec ModulePackageVersionSpec `json:"spec,omitempty"`

	// Status of a ModulePackageVersion.
	Status ModulePackageVersionStatus `json:"status,omitempty"`
}

// ModulePackageVersionSpec identifies the version. Every field is immutable because the
// object name is derived from the three of them.
type ModulePackageVersionSpec struct {
	// Name of the module package.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="packageName is immutable"
	// +crd-enricher:deckhouse:documentation:examples=sds-node-configurator
	PackageName string `json:"packageName"`

	// The name of the repository containing the package.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="packageRepositoryName is immutable"
	// +crd-enricher:deckhouse:documentation:examples=deckhouse
	PackageRepositoryName string `json:"packageRepositoryName"`

	// Version of the module package.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="packageVersion is immutable"
	// +crd-enricher:deckhouse:documentation:examples=v1.0.0
	PackageVersion string `json:"packageVersion"`
}

// ModulePackageVersionStatus reports loaded package metadata and whether the version is in use.
type ModulePackageVersionStatus struct {
	// Generation of the spec this status was computed from; a lower value means the status
	// has not caught up with the latest spec yet.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Metadata about the package such as description, requirements, etc.
	// +optional
	PackageMetadata *ModulePackageVersionStatusMetadata `json:"packageMetadata,omitempty"`

	// Schemas for validating settings and values passed to the package.
	// +optional
	PackageSchemas *PackageVersionStatusSchemas `json:"packageSchemas,omitempty"`

	// Conditions represent the latest available observations of the package version's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Whether a module uses this version. A single flag suffices because the object name
	// embeds the module name, so at most one module can ever use it. Blocks deletion.
	// Serialized even when false, so the Used print column reads false rather than blank.
	// +optional
	Used bool `json:"used"`
}

// ModulePackageVersionStatusMetadata is the package metadata loaded from the registry.
type ModulePackageVersionStatusMetadata struct {
	// Localized descriptions of the package.
	// +optional
	Description *PackageDescription `json:"description,omitempty"`

	// Parameters of package disable protection.
	// +optional
	DisableOptions *PackageDisableOptions `json:"disableOptions,omitempty"`

	// The category this package belongs to.
	// +optional
	Category string `json:"category,omitempty"`

	// The development stage of the package (e.g., Experimental, Preview, General Availability, Deprecated).
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=[Experimental, Preview, General Availability, Deprecated]
	Stage string `json:"stage,omitempty"`

	// The weight of the package.
	// +optional
	Weight int32 `json:"weight,omitempty"`

	// Critical indicates whether the package is critical for the cluster.
	// +optional
	Critical bool `json:"critical,omitempty"`

	// Indicates the group where only one module can be active at a time.
	// +crd-enricher:deckhouse:documentation:examples=cni
	// +optional
	ExclusiveGroup string `json:"exclusiveGroup,omitempty" yaml:"exclusiveGroup,omitempty"`

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

// IsDraft reports whether this package version is marked as a draft.
func (m *ModulePackageVersion) IsDraft() bool {
	return m.hasTrueLabel(ModulePackageVersionLabelDraft)
}

// IsLegacy reports whether this package version was produced from a legacy ModuleRelease
// rather than discovered as a package in a repository.
func (m *ModulePackageVersion) IsLegacy() bool {
	return m.hasTrueLabel(ModulePackageVersionLabelLegacy)
}

// hasTrueLabel reports whether the named label holds a truthy value. An unparsable value
// counts as false, so a hand-edited label cannot flip behaviour by accident.
func (m *ModulePackageVersion) hasTrueLabel(label string) bool {
	val, err := strconv.ParseBool(m.Labels[label])

	return err == nil && val
}

// +kubebuilder:object:root=true

// ModulePackageVersionList is a list of ModulePackageVersion resources
type ModulePackageVersionList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ModulePackageVersion `json:"items"`
}

// MakeModulePackageVersionName returns a name following the format <repository>-<packageName>-<version>
func MakeModulePackageVersionName(repository, packageName, version string) string {
	return repository + "-" + packageName + "-" + version
}
