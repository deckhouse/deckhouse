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
	PackageRepositoryResource = "packagerepositories"
	PackageRepositoryKind     = "PackageRepository"

	PackageRepositoryPhaseActive      = "Active"
	PackageRepositoryPhaseTerminating = "Terminating"

	PackageRepositoryFinalizerPackageVersionExists = "packages.deckhouse.io/package-version-exists"

	PackageRepositoryAnnotationRegistryChecksum = "packages.deckhouse.io/registry-spec-checksum"

	PackageRepositoryConditionLastOperationScanFinished = "LastOperationScanFinished"
)

var (
	PackageRepositoryGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: PackageRepositoryResource,
	}
	PackageRepositoryGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    PackageRepositoryKind,
	}
)

var _ runtime.Object = (*PackageRepository)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name=Phase,type=string,JSONPath=.status.phase
// +kubebuilder:printcolumn:name=Sync,type=date,JSONPath=.status.syncTime
// +kubebuilder:printcolumn:name=MSG,type=string,JSONPath=.status.conditions[?(@.type=='LastOperationScanFinished')].message
// +kubebuilder:printcolumn:name=Packages,type=integer,JSONPath=.status.packagesCount,priority=1

// PackageRepository is a source of packages for Deckhouse.
type PackageRepository struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a PackageRepository.
	Spec PackageRepositorySpec `json:"spec"`

	// Status of a PackageRepository.
	Status PackageRepositoryStatus `json:"status,omitempty"`
}

type PackageRepositorySpec struct {
	// Interval for registry scan.
	//
	// Defines the frequency of checking the container registry for new packages.
	// +optional
	ScanInterval *metav1.Duration `json:"scanInterval,omitempty"`
	// Configuration for the package registry.
	Registry PackageRepositorySpecRegistry `json:"registry"`
}

type PackageRepositorySpecRegistry struct {
	// Scheme to use for accessing the registry (e.g., https).
	// +optional
	Scheme string `json:"scheme,omitempty"`

	// Repository path in the registry.
	Repo string `json:"repo"`

	// Docker configuration for authentication.
	// +optional
	DockerCFG string `json:"dockerCfg,omitempty"`

	// Certificate authority data for TLS verification.
	// +optional
	CA string `json:"ca,omitempty"`
}

type PackageRepositoryStatus struct {
	// Last time the repository was synchronized.
	// +optional
	SyncTime metav1.Time `json:"syncTime,omitempty"`

	// List of packages available in this repository.
	// +optional
	Packages []PackageRepositoryStatusPackage `json:"packages,omitempty"`

	// Total number of packages in this repository.
	// +optional
	PackagesCount int `json:"packagesCount,omitempty"`

	// Current phase of the repository.
	// +optional
	Phase string `json:"phase,omitempty"`

	// Human-readable message about the repository status.
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations of the repository's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type PackageRepositoryStatusPackage struct {
	// Name of the package.
	Name string `json:"name"`

	// Type of the package.
	Type string `json:"type"`
}

// +kubebuilder:object:root=true

// PackageRepositoryList is a list of PackageRepository resources
type PackageRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PackageRepository `json:"items"`
}
