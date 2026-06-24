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

	PackageRepositoryConditionLastScanSucceeded = "LastScanSucceeded"
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
// +kubebuilder:printcolumn:name=Scan,type=date,JSONPath=.status.lastScanTime
// +kubebuilder:printcolumn:name=MSG,type=string,JSONPath=.status.conditions[?(@.type=='LastScanSucceeded')].message
// +kubebuilder:printcolumn:name=Packages,type=integer,JSONPath=.status.packagesCount,priority=1

// PackageRepository is a source of packages for Deckhouse Kubernetes Platform.
type PackageRepository struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the package repository configuration.
	Spec PackageRepositorySpec `json:"spec"`

	// Package repository status.
	Status PackageRepositoryStatus `json:"status,omitempty"`
}

type PackageRepositorySpec struct {
	// Interval for container registry scan.
	//
	// Defines the frequency of checking the container registry for new packages.
	// +optional
	// +kubebuilder:validation:Pattern=`^(\d+h)?(\d+m)?(\d+s)?$`
	// +crd-enricher:deckhouse:documentation:default=6h
	// +crd-enricher:deckhouse:documentation:examples=5m
	// +crd-enricher:deckhouse:documentation:examples=1h
	// +crd-enricher:deckhouse:documentation:examples=6h30m
	ScanInterval *metav1.Duration `json:"scanInterval,omitempty"`
	// Configuration for accessing the container registry with packages.
	Registry PackageRepositorySpecRegistry `json:"registry"`
}

type PackageRepositorySpecRegistry struct {
	// Protocol for accessing the repository (for example, `https`).
	// +optional
	Scheme string `json:"scheme,omitempty"`

	// Address of the package repository in the container registry.
	Repo string `json:"repo"`

	// Container registry access token in Base64 (`~/.docker/config.json` format).
	// Leave this field empty if anonymous access to the container registry is used.
	// +optional
	DockerCFG string `json:"dockerCfg,omitempty"`

	// Root CA certificate (PEM format) used to verify the container registry certificate over HTTPS
	// (if the container registry uses self-signed SSL certificates).
	// +optional
	CA string `json:"ca,omitempty"`

	// Username for authenticating to the container registry.
	// +optional
	Login string `json:"login,omitempty"`

	// Password for authenticating to the container registry.
	// +optional
	Password string `json:"password,omitempty"`
}

type PackageRepositoryStatus struct {
	// Time of the most recent scan of any outcome.
	// +optional
	LastScanTime *metav1.Time `json:"lastScanTime,omitempty"`

	// Time of the most recent scan that found at least one new version.
	// Scans that found nothing new do not advance this timestamp.
	// +optional
	LastChangeTime *metav1.Time `json:"lastChangeTime,omitempty"`

	// Number of new versions found by the most recent scan.
	// Set to zero when the last scan found nothing new.
	// +optional
	LastNewVersions int `json:"lastNewVersions,omitempty"`

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

	// Conditions reflecting the latest observations of the repository state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Indicates whether the container registry supports pagination when listing tags.
	// +optional
	PartialScanAvailable bool `json:"partialScanAvailable,omitempty"`
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
