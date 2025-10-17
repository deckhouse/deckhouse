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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ClusterApplicationPackageVersionResource = "clusterapplicationpackageversions"
	ClusterApplicationPackageVersionKind     = "ClusterApplicationPackageVersion"

	ClusterApplicationPackageVersionLabelDraft      = "draft"
	ClusterApplicationPackageVersionLabelPackage    = "package"
	ClusterApplicationPackageVersionLabelRepository = "repository"
)

var (
	ClusterApplicationPackageVersionGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ClusterApplicationPackageVersionResource,
	}
	ClusterApplicationPackageVersionGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ClusterApplicationPackageVersionKind,
	}
)

var _ runtime.Object = (*ClusterApplicationPackageVersion)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ClusterApplicationPackageVersion represents a version of a cluster application package.
type ClusterApplicationPackageVersion struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Status of a ClusterApplicationPackageVersion.
	Status ClusterApplicationPackageVersionStatus `json:"status,omitempty"`
}

type ClusterApplicationPackageVersionStatus struct {
	PackageName string                                          `json:"packageName,omitempty"`
	Version     string                                          `json:"version,omitempty"`
	Metadata    *ClusterApplicationPackageVersionStatusMetadata `json:"metadata,omitempty"`
}

type ClusterApplicationPackageVersionStatusMetadata struct {
	Description   map[string]string                 `json:"description,omitempty"`
	Category      string                            `json:"category,omitempty"`
	Stage         string                            `json:"stage,omitempty"`
	Requirements  *PackageRequirements              `json:"requirements,omitempty"`
	Licensing     *PackageLicensing                 `json:"licensing,omitempty"`
	Changelog     *PackageChangelog                 `json:"changelog,omitempty"`
	Compatibility *PackageVersionCompatibilityRules `json:"versionCompatibilityRules,omitempty"`
}

type PackageRequirements struct {
	Deckhouse  string            `json:"deckhouse,omitempty"`
	Kubernetes string            `json:"kubernetes,omitempty"`
	Modules    map[string]string `json:"modules,omitempty"`
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

// +kubebuilder:object:root=true

// ClusterApplicationPackageVersionList is a list of ClusterApplicationPackageVersion resources
type ClusterApplicationPackageVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterApplicationPackageVersion `json:"items"`
}
