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
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// Resource and kind names of ApplicationPackageVersion.
	ApplicationPackageVersionResource = "applicationpackageversions"
	ApplicationPackageVersionKind     = "ApplicationPackageVersion"

	// Labels carrying the version's origin and lifecycle state.
	ApplicationPackageVersionLabelDraft           = "packages.deckhouse.io/draft"
	ApplicationPackageVersionLabelPackage         = "packages.deckhouse.io/package"
	ApplicationPackageVersionLabelRepository      = "packages.deckhouse.io/repository"
	ApplicationPackageVersionLabelExistInRegistry = "packages.deckhouse.io/exist-in-registry"

	// Condition type and the reasons reported when metadata loading fails.
	ApplicationPackageVersionConditionTypeMetadataLoaded         = "MetadataLoaded"
	ApplicationPackageVersionConditionReasonFetchErr             = "FetchingReleaseError"
	ApplicationPackageVersionConditionReasonGetPackageRepoErr    = "GetPackageRepositoryError"
	ApplicationPackageVersionConditionReasonGetRegistryClientErr = "GetRegistryClientError"
	ApplicationPackageVersionConditionReasonGetImageErr          = "GetImageError"

	// Finalizer blocking deletion while any application still uses the version.
	ApplicationPackageVersionFinalizer = "applicationpackageversion.deckhouse.io/used-by-application"
)

// Group-version identifiers of ApplicationPackageVersion.
var (
	ApplicationPackageVersionGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ApplicationPackageVersionResource,
	}
	ApplicationPackageVersionGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ApplicationPackageVersionKind,
	}
)

var _ runtime.Object = (*ApplicationPackageVersion)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=apv
// +kubebuilder:printcolumn:name=Package,type=string,JSONPath=.spec.packageName
// +kubebuilder:printcolumn:name=Repository,type=string,JSONPath=.spec.packageRepositoryName
// +kubebuilder:printcolumn:name="MetadataLoaded",type="string",JSONPath=".status.conditions[?(@.type=='MetadataLoaded')].status"
// +kubebuilder:printcolumn:name="UsedBy",type=integer,JSONPath=`.status.usedByCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="TransitionTime",type="date",priority=1,JSONPath=".status.conditions[?(@.type=='MetadataLoaded')].lastTransitionTime"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.conditions[?(@.type=='MetadataLoaded')].message"
// +crd-enricher:raw:properties.apiVersion.description="APIVersion defines the versioned schema of this representation of an object.\nServers should convert recognized schemas to the latest internal value, and\nmay reject unrecognized values.\n\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)"
// +crd-enricher:raw:properties.kind.description="Kind is a string value representing the REST resource this object represents.\nServers may infer this from the endpoint the client submits requests to.\nCannot be updated.\nIn CamelCase.\n\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds)"

// +crd-enricher:deckhouse:documentation:examples={apiVersion: deckhouse.io/v1alpha1, kind: ApplicationPackageVersion, metadata: {name: example}, spec: {packageName: console, packageRepositoryName: deckhouse, packageVersion: v1.0.0}}
// ApplicationPackageVersion represents a version of an application package.
type ApplicationPackageVersion struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the application package version parameters.
	Spec ApplicationPackageVersionSpec `json:"spec,omitempty"`

	// Application package version status.
	Status ApplicationPackageVersionStatus `json:"status,omitempty"`
}

// ApplicationPackageVersionSpec identifies the version. Every field is immutable because
// the object name is derived from the three of them.
type ApplicationPackageVersionSpec struct {
	// Name of the application package.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="packageName is immutable"
	// +crd-enricher:deckhouse:documentation:examples=console
	PackageName string `json:"packageName"`

	// Name of the package repository containing the package.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="packageRepositoryName is immutable"
	// +crd-enricher:deckhouse:documentation:examples=deckhouse
	PackageRepositoryName string `json:"packageRepositoryName"`

	// Version of the application package.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="packageVersion is immutable"
	// +crd-enricher:deckhouse:documentation:examples=v1.0.0
	PackageVersion string `json:"packageVersion"`
}

// ApplicationPackageVersionStatus reports loaded package metadata, schemas and which
// applications use the version.
type ApplicationPackageVersionStatus struct {
	// Generation of the spec this status was computed from; a lower value means the status
	// has not caught up with the latest spec yet.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Metadata about the package such as description, requirements, etc.
	// +optional
	PackageMetadata *ApplicationPackageVersionStatusMetadata `json:"packageMetadata,omitempty"`

	// Schemas for validating settings and values passed to the package.
	// +optional
	PackageSchemas *PackageVersionStatusSchemas `json:"packageSchemas,omitempty"`

	// Conditions reflecting the latest observations of the package version state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Information about applications that are using this package version.
	// +optional
	UsedBy []ApplicationPackageVersionStatusInstance `json:"usedBy,omitempty"`

	// Number of applications using this package version; kept equal to the length of usedBy.
	// +optional
	// +kubebuilder:validation:Minimum=0
	UsedByCount int32 `json:"usedByCount,omitempty"`
}

// ApplicationPackageVersionStatusInstance identifies one application using the package version.
type ApplicationPackageVersionStatusInstance struct {
	// Namespace where the application is installed.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the application instance.
	// +optional
	Name string `json:"name,omitempty"`
}

// ApplicationPackageVersionStatusMetadata is the package metadata loaded from the registry.
type ApplicationPackageVersionStatusMetadata struct {
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

// IsDraft reports whether this package version is marked as a draft. An unparsable label
// value counts as false, so a hand-edited label cannot flip behaviour by accident.
func (a *ApplicationPackageVersion) IsDraft() bool {
	val, err := strconv.ParseBool(a.Labels[ApplicationPackageVersionLabelDraft])

	return err == nil && val
}

// IsAppInstalled checks if a specific application is installed using this package version.
func (a *ApplicationPackageVersion) IsAppInstalled(namespace string, appName string) bool {
	if len(a.Status.UsedBy) == 0 {
		return false
	}

	for _, v := range a.Status.UsedBy {
		if v.Namespace == namespace && v.Name == appName {
			return true
		}
	}

	return false
}

// AddInstalledApp records an application as using this package version, and reports whether
// that changed the status. Idempotent, so a repeated attach cannot inflate the count.
func (a *ApplicationPackageVersion) AddInstalledApp(namespace string, appName string) bool {
	if a.IsAppInstalled(namespace, appName) {
		return false
	}

	a.Status.UsedBy = append(a.Status.UsedBy, ApplicationPackageVersionStatusInstance{
		Namespace: namespace,
		Name:      appName,
	})
	a.Status.UsedByCount = int32(len(a.Status.UsedBy))

	return true
}

// RemoveInstalledApp drops an application from the list of applications using this package
// version, and reports whether that changed the status.
func (a *ApplicationPackageVersion) RemoveInstalledApp(namespace string, appName string) bool {
	prevLen := len(a.Status.UsedBy)
	a.Status.UsedBy = slices.DeleteFunc(a.Status.UsedBy, func(v ApplicationPackageVersionStatusInstance) bool {
		return v.Namespace == namespace && v.Name == appName
	})

	if len(a.Status.UsedBy) == prevLen {
		return false
	}

	a.Status.UsedByCount = int32(len(a.Status.UsedBy))

	return true
}

// +kubebuilder:object:root=true

// ApplicationPackageVersionList is a list of ApplicationPackageVersion resources
type ApplicationPackageVersionList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ApplicationPackageVersion `json:"items"`
}

// MakeApplicationPackageVersionName returns a name following the format <repository>-<packageName>-<version>
func MakeApplicationPackageVersionName(repository, packageName, version string) string {
	return repository + "-" + packageName + "-" + version
}
