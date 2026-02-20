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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ApplicationPackageVersionResource = "applicationpackageversions"
	ApplicationPackageVersionKind     = "ApplicationPackageVersion"

	ApplicationPackageVersionLabelDraft           = PackageVersionLabelDraft
	ApplicationPackageVersionLabelPackage         = PackageVersionLabelPackage
	ApplicationPackageVersionLabelRepository      = PackageVersionLabelRepository
	ApplicationPackageVersionLabelExistInRegistry = PackageVersionLabelExistInRegistry

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

	// Conditions represent the latest available observations of the package version's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []ApplicationPackageVersionCondition `json:"conditions,omitempty"`

	// Information about applications that are using this package version.
	// +optional
	UsedBy []ApplicationPackageVersionStatusInstance `json:"usedBy,omitempty"`

	// Number of applications using this package version.
	// +optional
	UsedByCount int `json:"usedByCount,omitempty"`
}

type ApplicationPackageVersionStatusInstance struct {
	// Namespace where the application is installed.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the application instance.
	// +optional
	Name string `json:"name,omitempty"`
}

type ApplicationPackageVersionCondition struct {
	// Type of the condition.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Type string `json:"type,omitempty"`

	// Machine-readable, UpperCamelCase text indicating the reason for the condition's last transition.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	// +optional
	Reason string `json:"reason,omitempty"`

	// Human-readable message indicating details about last transition.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	// +optional
	Message string `json:"message,omitempty"`

	// Status of the condition.
	// Can be True, False, Unknown.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Status corev1.ConditionStatus `json:"status,omitempty"`

	// Timestamp of when the condition was last probed.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`

	// Last time the condition transitioned from one status to another.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

type ApplicationPackageVersionStatusMetadata struct {
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

type PackageRequirements struct {
	// Required Deckhouse version.
	// +optional
	Deckhouse string `json:"deckhouse,omitempty"`

	// Required Kubernetes version.
	// +optional
	Kubernetes string `json:"kubernetes,omitempty"`

	// Required versions of other modules.
	// +optional
	Modules map[string]string `json:"modules,omitempty"`
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
