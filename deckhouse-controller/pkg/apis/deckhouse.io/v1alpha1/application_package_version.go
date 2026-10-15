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
// +crd-enricher:raw:properties.apiVersion.x-doc-skip=true
// +crd-enricher:raw:properties.kind.x-doc-skip=true
// +crd-enricher:raw:properties.metadata.x-doc-skip=true

// +crd-enricher:deckhouse:documentation:examples={apiVersion: deckhouse.io/v1alpha1, kind: ApplicationPackageVersion, metadata: {name: example}, spec: {packageName: console, packageRepositoryName: deckhouse, packageVersion: v1.0.0}}
// ApplicationPackageVersion defines a version of an application package.
type ApplicationPackageVersion struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Application package version parameters.
	Spec ApplicationPackageVersionSpec `json:"spec,omitempty"`

	// Current state of the application package version.
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
	// Value of `.metadata.generation` for which the current status was calculated.
	//
	// If the value is lower than the current `.metadata.generation`, the status does not yet reflect the latest changes to the instance.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Application package version metadata, including the description, requirements, disable options, and version compatibility rules.
	// +optional
	PackageMetadata *ApplicationPackageVersionStatusMetadata `json:"packageMetadata,omitempty"`

	// OpenAPI schemas for validating user-defined application settings and final values passed to package hooks and templates.
	// +optional
	// +crd-enricher:raw:properties.settingsSchema.description="OpenAPI v3 schema for validating user-defined application settings in `Application.spec.settings`.\n\nThe schema is stored as an opaque object because it has a recursive JSON Schema structure. The controller validates it when loading the package metadata."
	// +crd-enricher:raw:properties.valuesSchema.description="OpenAPI v3 schema for validating the final set of `values` passed to package hooks and templates.\n\nThe final set is formed by merging default values with user-defined application settings.\n\nThe schema is stored as an opaque object because it has a recursive JSON Schema structure. The controller validates it when loading the package metadata."
	PackageSchemas *PackageVersionStatusSchemas `json:"packageSchemas,omitempty"`

	// Conditions reflecting the current state of the application package version.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +crd-enricher:raw:items.description="Condition reflecting an aspect of the current state of the package version."
	// +crd-enricher:raw:items.properties.lastTransitionTime.description="Time when the condition status last changed."
	// +crd-enricher:raw:items.properties.message.description="Message with additional information about the condition.\n\nMay be empty."
	// +crd-enricher:raw:items.properties.observedGeneration.description="Value of `.metadata.generation` for which the condition was set.\n\nIndicates whether the condition corresponds to the current generation of the instance."
	// +crd-enricher:raw:items.properties.reason.description="Programmatic identifier indicating the reason for the last condition status change."
	// +crd-enricher:raw:items.properties.status.description="Condition status."
	// +crd-enricher:raw:items.properties.type.description="Condition type in CamelCase or in the format `foo.example.com/CamelCase`."
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// List of applications installed from this package version.
	// +optional
	UsedBy []ApplicationPackageVersionStatusInstance `json:"usedBy,omitempty"`

	// Number of applications installed from this package version.
	//
	// Equals the number of entries in `usedBy`.
	// +optional
	// +kubebuilder:validation:Minimum=0
	UsedByCount int32 `json:"usedByCount,omitempty"`
}

// Application installed from the package version.
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
	// Localized descriptions of the application package.
	// +optional
	// +crd-enricher:raw:properties.en.description="English description of the application package."
	// +crd-enricher:raw:properties.ru.description="Russian description of the application package."
	Description *PackageDescription `json:"description,omitempty"`

	// Parameters of package disable protection.
	// +optional
	// +crd-enricher:raw:properties.confirmation.description="Indicates whether confirmation is required to disable the application package."
	// +crd-enricher:raw:properties.messages.description="Localized messages displayed when confirming package disabling."
	// +crd-enricher:raw:properties.messages.properties.en.description="Disable confirmation message in English."
	// +crd-enricher:raw:properties.messages.properties.ru.description="Disable confirmation message in Russian."
	DisableOptions *PackageDisableOptions `json:"disableOptions,omitempty"`

	// Application package category.
	// +optional
	Category string `json:"category,omitempty"`

	// Life cycle stage of the application package.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=[Experimental, Preview, General Availability, Deprecated]
	Stage string `json:"stage,omitempty"`

	// System requirements for installing and running the application.
	// +optional
	// +crd-enricher:raw:properties.deckhouse.properties.constraint.description="Deckhouse Platform version constraint in SemVer format."
	// +crd-enricher:raw:properties.kubernetes.properties.constraint.description="Kubernetes version constraint in SemVer format."
	// +crd-enricher:raw:properties.modules.description="Deckhouse Platform module requirements: required and optional dependencies, groups of alternative modules, and groups of incompatible modules."
	// +crd-enricher:raw:properties.modules.properties.mandatory.description="Modules required for the application to run.\n\nEach specified module must be enabled. If a version constraint is specified, the module version must satisfy it."
	// +crd-enricher:raw:properties.modules.properties.mandatory.items.description="Required module with an optional version constraint in SemVer format.\n\nIf no constraint is specified, any version is allowed."
	// +crd-enricher:raw:properties.modules.properties.mandatory.items.properties.constraint.description="Module version constraint in SemVer format."
	// +crd-enricher:raw:properties.modules.properties.conditional.description="Optional modules with version constraints.\n\nThe module does not have to be enabled. If it is enabled, its version must satisfy the specified constraint.\n\nReplaces the deprecated `!optional` suffix from the v1 requirements format."
	// +crd-enricher:raw:properties.modules.properties.conditional.items.description="Optional module with a version constraint in SemVer format.\n\nIf no constraint is specified, any version is allowed."
	// +crd-enricher:raw:properties.modules.properties.conditional.items.properties.constraint.description="Module version constraint in SemVer format."
	// +crd-enricher:raw:properties.modules.properties.anyOf.description="Groups of alternative modules.\n\nAt least one module in each group must be enabled. If a version constraint is specified for a module, its version must satisfy that constraint.\n\nThese groups do not add edges to the dependency graph."
	// +crd-enricher:raw:properties.modules.properties.anyOf.items.description="Group of alternative modules, at least one of which must be enabled."
	// +crd-enricher:raw:properties.modules.properties.anyOf.items.properties.modules.description="Alternative modules in the group."
	// +crd-enricher:raw:properties.modules.properties.anyOf.items.properties.modules.items.description="Module with an optional version constraint in SemVer format.\n\nIf no constraint is specified, any version is allowed."
	// +crd-enricher:raw:properties.modules.properties.anyOf.items.properties.modules.items.properties.constraint.description="Module version constraint in SemVer format."
	// +crd-enricher:raw:properties.modules.properties.noneOf.description="Groups of incompatible modules.\n\nFor the application to run, none of the modules in these groups may be enabled with an incompatible version.\n\nIf no version constraint is specified, all versions of the module are considered incompatible. If a constraint is specified, only versions satisfying that constraint are considered incompatible.\n\nThese groups do not add edges to the dependency graph."
	// +crd-enricher:raw:properties.modules.properties.noneOf.items.description="Group of incompatible modules.\n\nNone of the listed modules may be enabled with an incompatible version."
	// +crd-enricher:raw:properties.modules.properties.noneOf.items.properties.modules.description="Incompatible modules in the group."
	// +crd-enricher:raw:properties.modules.properties.noneOf.items.properties.modules.items.description="Module with an optional version constraint in SemVer format.\n\nIf no constraint is specified, all versions of the module are considered incompatible."
	// +crd-enricher:raw:properties.modules.properties.noneOf.items.properties.modules.items.properties.constraint.description="Module version constraint in SemVer format defining the range of incompatible versions."
	Requirements *PackageRequirements `json:"requirements,omitempty"`

	// Application package licensing information for different editions.
	// +optional
	// +crd-enricher:raw:properties.editions.description="Licensing settings for different application package editions."
	// +crd-enricher:unset:properties.editions.additionalProperties.description
	// +crd-enricher:raw:properties.editions.additionalProperties.properties.available.description="Indicates whether the edition is available for use."
	// +crd-enricher:raw:properties.editions.additionalProperties.properties.enabledInBundles.description="Bundles in which the package is enabled by default in this edition.\n\nNot used for application packages."
	Licensing *PackageLicensing `json:"licensing,omitempty"`

	// Information about changes in this package version.
	// +optional
	// +crd-enricher:raw:properties.features.description="List of new features in this package version."
	// +crd-enricher:raw:properties.fixes.description="List of bugs fixed in this package version."
	Changelog *PackageChangelog `json:"changelog,omitempty"`

	// Version compatibility rules for upgrades and downgrades.
	// +kubebuilder:default={}
	// +optional
	// +crd-enricher:raw:properties.upgrade.description="Compatibility rules for upgrading to this application version."
	// +crd-enricher:raw:properties.upgrade.properties.step.description="Compatibility rule step."
	// +crd-enricher:raw:properties.upgrade.properties.step.properties.major.description="Number of major versions that can be skipped during an upgrade.\n\nThe `-1` value removes all limits, `0` forbids the change.\n\nDefaults to `0`."
	// +crd-enricher:raw:properties.upgrade.properties.step.properties.minor.description="Number of minor versions that can be skipped during an upgrade.\n\nThe `-1` value removes all limits, `0` forbids the change.\n\nDefaults to `1`."
	// +crd-enricher:raw:properties.upgrade.properties.step.properties.patches.description="Number of patch versions that can be skipped during an upgrade.\n\nThe `-1` value removes all limits, `0` forbids the change.\n\nDefaults to `-1`."
	// +crd-enricher:raw:properties.downgrade.description="Compatibility rules for downgrading from this application version."
	// +crd-enricher:raw:properties.downgrade.properties.step.description="Compatibility rule step."
	// +crd-enricher:raw:properties.downgrade.properties.step.properties.major.description="Number of major versions that can be skipped during a downgrade.\n\nThe `-1` value removes all limits, `0` forbids the change.\n\nDefaults to `0`."
	// +crd-enricher:raw:properties.downgrade.properties.step.properties.minor.description="Number of minor versions that can be skipped during a downgrade.\n\nThe `-1` value removes all limits, `0` forbids the change.\n\nDefaults to `0`."
	// +crd-enricher:raw:properties.downgrade.properties.step.properties.patches.description="Number of patch versions that can be skipped during a downgrade.\n\nThe `-1` value removes all limits, `0` forbids the change.\n\nDefaults to `-1`."
	Compatibility *PackageVersionCompatibilityRules `json:"versionCompatibilityRules,omitempty"`
}

// IsDraft reports whether this package version is marked as a draft. An unparsable label
// value counts as false, so a hand-edited label cannot flip behaviour by accident.
func (a *ApplicationPackageVersion) IsDraft() bool {
	val, err := strconv.ParseBool(a.Labels[PackageLabelDraft])

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
