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
	corev1 "k8s.io/api/core/v1"
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

	ApplicationPackageVersionConditionTypeEnriched               = "MetadataLoaded"
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
// +kubebuilder:resource:scope=Cluster

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
	PackageName string `json:"packageName,omitempty"`
	Version     string `json:"version,omitempty"`
	Repository  string `json:"repository,omitempty"`
}

type ApplicationPackageVersionStatus struct {
	PackageName     string                                    `json:"packageName,omitempty"`
	PackageMetadata *ApplicationPackageVersionStatusMetadata  `json:"packageMetadata,omitempty"`
	Version         string                                    `json:"version,omitempty"`
	Conditions      []ApplicationPackageVersionCondition      `json:"conditions,omitempty"`
	UsedBy          []ApplicationPackageVersionStatusInstance `json:"usedBy,omitempty"`
	UsedByCount     int                                       `json:"usedByCount,omitempty"`
}

type ApplicationPackageVersionStatusInstance struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

type ApplicationPackageVersionCondition struct {
	// Type is the type of the condition.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Type string `json:"type,omitempty"`
	// Machine-readable, UpperCamelCase text indicating the reason for the condition's last transition.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Message string `json:"message,omitempty"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Status corev1.ConditionStatus `json:"status,omitempty"`
	// Timestamp of when the condition was last probed.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

type ApplicationPackageVersionStatusMetadata struct {
	Description   *PackageDescription               `json:"description,omitempty"`
	Category      string                            `json:"category,omitempty"`
	Stage         string                            `json:"stage,omitempty"`
	Requirements  *PackageRequirements              `json:"requirements,omitempty"`
	Licensing     *PackageLicensing                 `json:"licensing,omitempty"`
	Changelog     *PackageChangelog                 `json:"changelog,omitempty"`
	Compatibility *PackageVersionCompatibilityRules `json:"versionCompatibilityRules,omitempty"`
}

func (a *ApplicationPackageVersion) IsDraft() bool {
	val, ok := a.Labels[ApplicationPackageVersionLabelDraft]
	if ok && val == "true" {
		return true
	}

	return false
}

// +kubebuilder:object:root=true

// ApplicationPackageVersionList is a list of ApplicationPackageVersion resources
type ApplicationPackageVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ApplicationPackageVersion `json:"items"`
}

type PackageRequirements struct {
	Deckhouse  string            `json:"deckhouse,omitempty"`
	Kubernetes string            `json:"kubernetes,omitempty"`
	Modules    map[string]string `json:"modules,omitempty"`
}

type PackageDescription struct {
	Ru string `json:"ru,omitempty"`
	En string `json:"en,omitempty"`
}

type PackageLicensing struct {
	Editions map[string]PackageEdition `json:"editions,omitempty"`
}

type PackageEdition struct {
	Available bool `json:"available,omitempty"`
}

type PackageChangelog struct {
	Features []string `json:"features,omitempty"`
	Fixes    []string `json:"fixes,omitempty"`
}

type PackageVersionCompatibilityRules struct {
	Upgrade   *PackageVersionCompatibilityRule `json:"upgrade,omitempty"`
	Downgrade *PackageVersionCompatibilityRule `json:"downgrade,omitempty"`
}

type PackageVersionCompatibilityRule struct {
	From             string `json:"from,omitempty"`
	To               string `json:"to,omitempty"`
	AllowSkipPatches int    `json:"allowSkipPatches,omitempty"`
	AllowSkipMinor   int    `json:"allowSkipMinor,omitempty"`
	AllowSkipMajor   int    `json:"allowSkipMajor,omitempty"`
	MaxRollback      int    `json:"maxRollback,omitempty"`
}

// Returns a name following the format <repository>-<packageName>-<version>
func MakeApplicationPackageVersionName(repository, packageName, version string) string {
	return repository + "-" + packageName + "-" + version
}
