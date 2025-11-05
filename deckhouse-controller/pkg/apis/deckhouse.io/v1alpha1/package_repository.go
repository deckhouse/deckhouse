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
	Registry PackageRepositorySpecRegistry `json:"registry"`
}

type PackageRepositorySpecRegistry struct {
	Scheme    string `json:"scheme,omitempty"`
	Repo      string `json:"repo"`
	DockerCFG string `json:"dockerCfg"`
	CA        string `json:"ca"`
}

type PackageRepositoryStatus struct {
	SyncTime      metav1.Time                      `json:"syncTime,omitempty"`
	Packages      []PackageRepositoryStatusPackage `json:"packages,omitempty"`
	PackagesCount int                              `json:"packagesCount,omitempty"`
	Phase         string                           `json:"phase,omitempty"`
	Message       string                           `json:"message,omitempty"`
}

type PackageRepositoryStatusPackage struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// +kubebuilder:object:root=true

// PackageRepositoryList is a list of PackageRepository resources
type PackageRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PackageRepository `json:"items"`
}
