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
	PackageRepositoryOperationResource = "packagerepositoryoperations"
	PackageRepositoryOperationKind     = "PackageRepositoryOperation"

	PackageRepositoryOperationTypeScan = "Update"

	PackageRepositoryOperationPhasePending    = "Pending"
	PackageRepositoryOperationPhaseProcessing = "Processing"
	PackageRepositoryOperationPhaseCompleted  = "Completed"
	PackageRepositoryOperationPhaseFailed     = "Failed"
)

var (
	PackageRepositoryOperationGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: PackageRepositoryOperationResource,
	}
	PackageRepositoryOperationGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    PackageRepositoryOperationKind,
	}
)

var _ runtime.Object = (*PackageRepositoryOperation)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// PackageRepositoryOperation represents an operation to scan/update a package repository.
type PackageRepositoryOperation struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a PackageRepositoryOperation.
	Spec PackageRepositoryOperationSpec `json:"spec"`

	// Status of a PackageRepositoryOperation.
	Status PackageRepositoryOperationStatus `json:"status,omitempty"`
}

type PackageRepositoryOperationSpec struct {
	PackageRepository string                            `json:"packageRepository"`
	Type              string                            `json:"type"`
	Update            *PackageRepositoryOperationUpdate `json:"update,omitempty"`
}

type PackageRepositoryOperationUpdate struct {
	FullScan bool   `json:"fullScan,omitempty"`
	Timeout  string `json:"timeout,omitempty"`
}

type PackageRepositoryOperationStatus struct {
	Phase             string                                         `json:"phase,omitempty"`
	Message           string                                         `json:"message,omitempty"`
	StartTime         *metav1.Time                                   `json:"startTime,omitempty"`
	CompletionTime    *metav1.Time                                   `json:"completionTime,omitempty"`
	Packages          *PackageRepositoryOperationStatusPackages      `json:"packages,omitempty"`
	PackagesToProcess []PackageRepositoryOperationStatusPackageQueue `json:"packagesToProcess,omitempty"`
}

type PackageRepositoryOperationStatusPackages struct {
	Discovered int `json:"discovered,omitempty"`
	Processed  int `json:"processed,omitempty"`
	Total      int `json:"total,omitempty"`
}

type PackageRepositoryOperationStatusPackageQueue struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// +kubebuilder:object:root=true

// PackageRepositoryOperationList is a list of PackageRepositoryOperation resources
type PackageRepositoryOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PackageRepositoryOperation `json:"items"`
}
