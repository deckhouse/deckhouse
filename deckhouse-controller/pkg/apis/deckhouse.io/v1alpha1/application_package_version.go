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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ApplicationPackageVersionResource = "applicationpackageversions"
	ApplicationPackageVersionKind     = "ApplicationPackageVersion"

	ApplicationPackageVersionLabelDraft           = "packages.deckhouse.io/draft"
	ApplicationPackageVersionLabelPackage         = "packages.deckhouse.io/package"
	ApplicationPackageVersionLabelRepository      = "packages.deckhouse.io/repository"
	ApplicationPackageVersionLabelExistInRegistry = "packages.deckhouse.io/exist-in-registry"

	ApplicationPackageVersionConditionTypeMetadataLoaded         = "MetadataLoaded"
	ApplicationPackageVersionConditionReasonFetchErr             = "FetchingReleaseError"
	ApplicationPackageVersionConditionReasonGetPackageRepoErr    = "GetPackageRepositoryError"
	ApplicationPackageVersionConditionReasonGetRegistryClientErr = "GetRegistryClientError"
	ApplicationPackageVersionConditionReasonGetImageErr          = "GetImageError"

	ApplicationPackageVersionFinalizer = "applicationpackageversion.deckhouse.io/used-by-application"
)

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
// +kubebuilder:printcolumn:name="TransitionTime",type="date",JSONPath=".status.conditions[?(@.type=='MetadataLoaded')].lastTransitionTime"
// +kubebuilder:printcolumn:name="MetadataLoaded",type="string",JSONPath=".status.conditions[?(@.type=='MetadataLoaded')].status"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='MetadataLoaded')].message"
// +kubebuilder:printcolumn:name="UsedBy",type=integer,JSONPath=`.status.usedByCount`

// ApplicationPackageVersion represents a version of an application package.
type ApplicationPackageVersion struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ApplicationPackageVersionSpec `json:"spec,omitempty"`

	// Status of an ApplicationPackageVersion.
	Status ApplicationPackageVersionStatus `json:"status,omitempty"`
}

type ApplicationPackageVersionSpec struct {
	// Name of the application package.
	// +optional
	// +kubebuilder:validation:Immutable
	PackageName string `json:"packageName,omitempty"`

	// The name of the repository containing the package.
	// +optional
	// +kubebuilder:validation:Immutable
	PackageRepositoryName string `json:"packageRepositoryName,omitempty"`

	// Version of the application package.
	// +optional
	// +kubebuilder:validation:Immutable
	PackageVersion string `json:"packageVersion,omitempty"`
}

type ApplicationPackageVersionStatus struct {
	// Metadata about the package such as description, requirements, etc.
	// +optional
	PackageMetadata *ApplicationPackageVersionStatusMetadata `json:"packageMetadata,omitempty"`

	// Schemas for validating settings and values passed to the package.
	// +optional
	PackageSchemas *ApplicationPackageVersionStatusSchemas `json:"packageSchemas,omitempty"`

	// Conditions represent the latest available observations of the package version's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Information about applications that are using this package version.
	// +optional
	UsedBy []ApplicationPackageVersionStatusInstance `json:"usedBy,omitempty"`

	// Number of applications using this package version.
	// +optional
	UsedByCount int `json:"usedByCount,omitempty"`
}

type ApplicationPackageVersionStatusSchemas struct {
	// SettingsSchema is the OpenAPI v3 schema used to validate the user-supplied
	// settings of the package. Stored as an opaque object because its contents
	// form a recursive JSON schema that cannot be expressed structurally in a
	// CRD; the controller validates this subtree in Go when loading package
	// metadata.
	// +optional
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	SettingsSchema *apiextensionsv1.CustomResourceValidation `json:"settingsSchema,omitempty"`

	// ValuesSchema is the OpenAPI v3 schema used to validate the effective
	// values (defaults merged with settings) passed to the package's hooks and
	// charts. Stored as an opaque object because its contents form a recursive
	// JSON schema that cannot be expressed structurally in a CRD; the
	// controller validates this subtree in Go when loading package metadata.
	// +optional
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	ValuesSchema *apiextensionsv1.CustomResourceValidation `json:"valuesSchema,omitempty"`
}

type ApplicationPackageVersionStatusInstance struct {
	// Namespace where the application is installed.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the application instance.
	// +optional
	Name string `json:"name,omitempty"`
}

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
func (a *ApplicationPackageVersion) IsDraft() bool {
	val, ok := a.Labels[ApplicationPackageVersionLabelDraft]
	if ok && val == "true" {
		return true
	}

	return false
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

// AddInstalledApp adds an application to the list of applications using this package version.
func (a *ApplicationPackageVersion) AddInstalledApp(namespace string, appName string) *ApplicationPackageVersion {
	appStatusInstalledApp := ApplicationPackageVersionStatusInstance{Namespace: namespace, Name: appName}

	a.Status.UsedBy = append(a.Status.UsedBy, appStatusInstalledApp)

	a.Status.UsedByCount++

	return a
}

// RemoveInstalledApp removes an application from the list of applications using this package version.
func (a *ApplicationPackageVersion) RemoveInstalledApp(namespace string, appName string) *ApplicationPackageVersion {
	prevLen := len(a.Status.UsedBy)
	a.Status.UsedBy = slices.DeleteFunc(a.Status.UsedBy, func(v ApplicationPackageVersionStatusInstance) bool {
		return v.Namespace == namespace && v.Name == appName
	})

	if len(a.Status.UsedBy) < prevLen && a.Status.UsedByCount > 0 {
		a.Status.UsedByCount--
	}

	return a
}

// +kubebuilder:object:root=true

// ApplicationPackageVersionList is a list of ApplicationPackageVersion resources
type ApplicationPackageVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ApplicationPackageVersion `json:"items"`
}

// PackageRequirements describes the platform and module dependencies of a package,
// surfaced as part of the package version status.
type PackageRequirements struct {
	// Required Deckhouse version.
	// +optional
	Deckhouse *VersionConstraint `json:"deckhouse,omitempty"`

	// Required Kubernetes version.
	// +optional
	Kubernetes *VersionConstraint `json:"kubernetes,omitempty"`

	// Required modules, partitioned into mandatory, conditional, and anyOf
	// dependency buckets.
	// +optional
	Modules *PackageModulesRequirements `json:"modules,omitempty"`
}

// VersionConstraint wraps a single semver constraint expression (e.g. ">= 1.26").
type VersionConstraint struct {
	// Semver constraint expression.
	// +optional
	Constraint string `json:"constraint,omitempty"`
}

// PackageModulesRequirements groups module dependencies by how they affect startup.
type PackageModulesRequirements struct {
	// Mandatory dependencies — must be present (and satisfy the constraint, if any)
	// for the package to start.
	// +optional
	Mandatory []PackageModuleDependency `json:"mandatory,omitempty"`

	// Conditional dependencies — not required to be present, but if installed must
	// satisfy the constraint for the package to function correctly. Replaces the
	// legacy "!optional" suffix from the v1 requirements format.
	// +optional
	Conditional []PackageModuleDependency `json:"conditional,omitempty"`

	// AnyOf groups of alternative dependencies — at least one member of each group
	// must be installed (and satisfy its constraint, if any) for the package to
	// start. Groups are checker-only and add no edges to the dependency graph.
	// +optional
	AnyOf []PackageModuleGroup `json:"anyOf,omitempty"`

	// NoneOf groups of forbidden dependencies — no member of any group may be
	// installed for the package to start. A member with no constraint is forbidden
	// at any version; a member with a constraint is forbidden only at versions
	// matching that constraint. Groups are checker-only and add no edges to the
	// dependency graph.
	// +optional
	NoneOf []PackageModuleGroup `json:"noneOf,omitempty"`
}

// PackageModuleDependency is a single named module dependency with a semver
// constraint. The constraint is required for entries in
// PackageModulesRequirements.Conditional ("if installed, no version requirement"
// is a no-op and rejected at parse time); for entries in
// PackageModulesRequirements.Mandatory the constraint is optional and an empty
// value means "any version".
type PackageModuleDependency struct {
	// Module name.
	Name string `json:"name"`

	// Semver constraint expression.
	// +optional
	Constraint string `json:"constraint,omitempty"`
}

// PackageModuleGroup is a named group of module dependencies. Group semantics
// depend on the containing bucket: members of an anyOf group are alternatives
// (at least one must be installed), members of a noneOf group are forbidden
// (none may be installed). The Name is required and surfaces in scheduler
// diagnostics; the Description is optional human-facing documentation.
type PackageModuleGroup struct {
	// Stable identifier used by the scheduler in diagnostics.
	Name string `json:"name"`

	// Human-readable description of the group's purpose.
	// +optional
	Description string `json:"description,omitempty"`

	// Module dependencies in this group. The bucket containing the group
	// (anyOf / noneOf) defines whether members are alternatives or forbidden.
	Modules []PackageModuleDependency `json:"modules"`
}

type PackageDescription struct {
	// Russian description of the package.
	// +optional
	Ru string `json:"ru,omitempty"`

	// English description of the package.
	// +optional
	En string `json:"en,omitempty"`
}

type PackageLicensing struct {
	// Licensing information for different package editions.
	// +optional
	Editions map[string]PackageEdition `json:"editions,omitempty"`
}

type PackageEdition struct {
	// Whether this edition is available for use.
	// +optional
	Available bool `json:"available,omitempty"`
}

type PackageChangelog struct {
	// List of new features in this version.
	// +optional
	Features []string `json:"features,omitempty"`

	// List of bug fixes in this version.
	// +optional
	Fixes []string `json:"fixes,omitempty"`
}

type PackageVersionCompatibilityRules struct {
	// Compatibility rules for upgrading to this version.
	// +optional
	Upgrade *PackageVersionCompatibilityRule `json:"upgrade,omitempty"`

	// Compatibility rules for downgrading from this version.
	// +optional
	Downgrade *PackageVersionCompatibilityRule `json:"downgrade,omitempty"`
}

type PackageVersionCompatibilityRule struct {
	// Starting version range for compatibility.
	// +optional
	From string `json:"from,omitempty"`

	// Ending version range for compatibility.
	// +optional
	To string `json:"to,omitempty"`

	// How many patch versions can be skipped.
	// +optional
	AllowSkipPatches int `json:"allowSkipPatches,omitempty"`

	// How many minor versions can be skipped.
	// +optional
	AllowSkipMinor int `json:"allowSkipMinor,omitempty"`

	// How many major versions can be skipped.
	// +optional
	AllowSkipMajor int `json:"allowSkipMajor,omitempty"`

	// Maximum number of versions that can be rolled back.
	// +optional
	MaxRollback int `json:"maxRollback,omitempty"`
}

// MakeApplicationPackageVersionName returns a name following the format <repository>-<packageName>-<version>
func MakeApplicationPackageVersionName(repository, packageName, version string) string {
	return repository + "-" + packageName + "-" + version
}
