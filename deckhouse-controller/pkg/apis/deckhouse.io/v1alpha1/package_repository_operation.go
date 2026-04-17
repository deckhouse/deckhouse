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
	metautils "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	PackageRepositoryOperationResource = "packagerepositoryoperations"
	PackageRepositoryOperationKind     = "PackageRepositoryOperation"

	// PackageRepositoryOperation condition types
	PackageRepositoryOperationConditionCompleted = "Completed"

	// PackageRepositoryOperation condition reasons
	PackageRepositoryOperationReasonDiscover   = "Discover"
	PackageRepositoryOperationReasonProcessing = "Processing"
	PackageRepositoryOperationReasonCompleted  = "Completed"
	PackageRepositoryOperationReasonFailed     = "Failed"

	// PackagesRepositoryOperationLabelRepository is the label used to identify PackageRepositoryOperations
	// that belong to a specific PackageRepository
	PackagesRepositoryOperationLabelRepository = "packages.deckhouse.io/repository"

	PackagesRepositoryOperationLabelOperationType = "packages.deckhouse.io/operation-type"
	PackageRepositoryOperationTypeUpdate          = "Update"

	PackagesRepositoryOperationLabelOperationTrigger = "packages.deckhouse.io/operation-trigger"
	PackagesRepositoryTriggerManual                  = "manual"
	PackagesRepositoryTriggerAuto                    = "auto"
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
// +kubebuilder:resource:scope=Cluster,shortName=pro
// +kubebuilder:printcolumn:name=Count,type=integer,JSONPath=.status.packages.total
// +kubebuilder:printcolumn:name=Completed,type=string,JSONPath=.status.conditions[?(@.type=='Completed')].status
// +kubebuilder:printcolumn:name=MSG,type=string,JSONPath=.status.conditions[?(@.type=='Completed')].message
// +kubebuilder:printcolumn:name=CompletionTime,type=date,JSONPath=.status.completionTime

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
	// Name of the package repository to operate on.
	PackageRepositoryName string `json:"packageRepositoryName"`

	// Type of operation to perform.
	Type string `json:"type"`

	// Configuration for update operations.
	// +optional
	Update *PackageRepositoryOperationUpdate `json:"update,omitempty"`
}

type PackageRepositoryOperationUpdate struct {
	// Whether to perform a full scan of the repository.
	// +optional
	FullScan bool `json:"fullScan,omitempty"`

	// Timeout for the operation.
	// +optional
	Timeout string `json:"timeout,omitempty"`
}

type PackageRepositoryOperationStatus struct {
	// Time when the operation started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Time when the operation completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Information about packages processed during the operation.
	// +optional
	Packages *PackageRepositoryOperationStatusPackages `json:"packages,omitempty"`

	// Conditions represent the latest available observations of the application's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type PackageRepositoryOperationStatusPackages struct {
	// List of packages discovered during the operation.
	// +optional
	Discovered []PackageRepositoryOperationStatusDiscoveredPackage `json:"discovered,omitempty"`

	// List of packages that failed processing.
	// +optional
	Failed []PackageRepositoryOperationStatusFailedPackage `json:"failed,omitempty"`

	// List of packages successfully processed.
	// +optional
	Processed []PackageRepositoryOperationStatusPackage `json:"processed,omitempty"`

	// Total number of packages processed.
	// +optional
	ProcessedOverall int `json:"processedOverall,omitempty"`

	// Total number of packages found.
	// +optional
	Total int `json:"total,omitempty"`
}

type PackageRepositoryOperationStatusDiscoveredPackage struct {
	// Name of the discovered package.
	Name string `json:"name"`
}

type PackageRepositoryOperationStatusFailedPackage struct {
	// Name of the package that failed.
	Name string `json:"name"`

	// List of errors encountered while processing this package.
	Errors []PackageRepositoryOperationStatusFailedPackageError `json:"errors"`
}

type PackageRepositoryOperationStatusFailedPackageError struct {
	// Version of the package that failed.
	Version string `json:"version"`

	// Message of the error.
	Message string `json:"message"`
}

type PackageRepositoryOperationStatusPackage struct {
	// Name of the processed package.
	Name string `json:"name"`

	// Type of the package.
	// +optional
	Type string `json:"type,omitempty"`

	// Number of versions found during this operation.
	// +optional
	FoundVersions int `json:"foundVersions,omitempty"`
}

func (o *PackageRepositoryOperation) GetStateByCondition() string {
	cond := metautils.FindStatusCondition(o.Status.Conditions, PackageRepositoryOperationConditionCompleted)
	if cond == nil {
		return ""
	}

	return cond.Reason
}

func (o *PackageRepositoryOperation) IsCompleted() bool {
	cond := metautils.FindStatusCondition(o.Status.Conditions, PackageRepositoryOperationConditionCompleted)
	if cond == nil {
		return false
	}

	return cond.Status == metav1.ConditionTrue
}

// +kubebuilder:object:root=true

// PackageRepositoryOperationList is a list of PackageRepositoryOperation resources
type PackageRepositoryOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PackageRepositoryOperation `json:"items"`
}
