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

type PackageRepositoryOperationType string

const (
	PackageRepositoryOperationResource = "packagerepositoryoperations"
	PackageRepositoryOperationKind     = "PackageRepositoryOperation"

	// PackageRepositoryOperation condition types
	PackageRepositoryOperationConditionCompleted = "Completed"

	// PackageRepositoryOperation condition reasons
	PackageRepositoryOperationReasonDiscover      = "Discover"
	PackageRepositoryOperationReasonProcessing    = "Processing"
	PackageRepositoryOperationReasonScanSucceeded = "ScanSucceeded"
	PackageRepositoryOperationReasonScanFailed    = "ScanFailed"

	// PackageRepositoryOperation types, mirrored onto the operation by PackageLabelOperationType
	PackageRepositoryOperationTypeUpdate PackageRepositoryOperationType = "Update"
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
// +crd-enricher:raw:properties.apiVersion.description="APIVersion defines the versioned schema of this representation of an object.\nServers should convert recognized schemas to the latest internal value, and\nmay reject unrecognized values.\n\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)"
// +crd-enricher:raw:properties.kind.description="Kind is a string value representing the REST resource this object represents.\nServers may infer this from the endpoint the client submits requests to.\nCannot be updated.\nIn CamelCase.\n\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds)"
// +crd-enricher:raw:properties.apiVersion.x-doc-skip=true
// +crd-enricher:raw:properties.kind.x-doc-skip=true
// +crd-enricher:raw:properties.metadata.x-doc-skip=true

// +crd-enricher:deckhouse:documentation:examples={apiVersion: deckhouse.io/v1alpha1, kind: PackageRepositoryOperation, metadata: {name: example}, spec: {packageRepositoryName: deckhouse, type: Update, update: {fullScan: false, timeout: 5m}}}
// PackageRepositoryOperation represents an operation for scanning or updating a package repository.
type PackageRepositoryOperation struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Package repository operation parameters.
	Spec PackageRepositoryOperationSpec `json:"spec"`

	// Current state of the package repository operation.
	Status PackageRepositoryOperationStatus `json:"status,omitempty"`
}

type PackageRepositoryOperationSpec struct {
	// Name of the package repository for which the operation is performed.
	// +crd-enricher:deckhouse:documentation:examples=deckhouse
	PackageRepositoryName string `json:"packageRepositoryName"`

	// Operation type.
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Update
	Type PackageRepositoryOperationType `json:"type"`

	// Update operation parameters.
	// +optional
	Update *PackageRepositoryOperationUpdate `json:"update,omitempty"`
}

type PackageRepositoryOperationUpdate struct {
	// Indicates whether to perform a full scan of the package repository.
	// +optional
	FullScan bool `json:"fullScan,omitempty"`

	// Operation timeout.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=5m
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

	// Conditions reflecting the current state of the operation.
	//
	// The operation phase is determined by the `Completed` condition: while its status is `False`, the operation is in one of the intermediate phases (`Discover` or `Processing`). A `True` status indicates that the operation has completed with the `ScanSucceeded` or `ScanFailed` reason.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +crd-enricher:raw:items.properties.reason.x-doc-examples=[Discover, Processing, Succeeded, Failed]
	// +crd-enricher:raw:items.description="Condition reflecting an aspect of the current state of the operation."
	// +crd-enricher:raw:items.properties.lastTransitionTime.description="Time when the condition status last changed."
	// +crd-enricher:raw:items.properties.message.description="Message with additional information about the condition."
	// +crd-enricher:raw:items.properties.observedGeneration.description="Value of `.metadata.generation` for which the condition was set.\n\nIndicates whether the condition corresponds to the current generation of the instance."
	// +crd-enricher:raw:items.properties.reason.description="Programmatic identifier indicating the reason for the last condition status change.\n\nFor the `Completed` condition, possible values are `Discover`, `Processing`, `ScanSucceeded`, and `ScanFailed`."
	// +crd-enricher:raw:items.properties.status.description="Condition status."
	// +crd-enricher:raw:items.properties.type.description="Condition type in CamelCase or in the format `foo.example.com/CamelCase`.\n\nThis resource uses the `Completed` condition, which indicates whether the operation has completed."
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type PackageRepositoryOperationStatusPackages struct {
	// Packages discovered during the operation and awaiting processing.
	//
	// The list is empty after the operation completes.
	// +optional
	Discovered []PackageRepositoryOperationStatusDiscoveredPackage `json:"discovered,omitempty"`

	// Packages that could not be processed.
	// +optional
	Failed []PackageRepositoryOperationStatusFailedPackage `json:"failed,omitempty"`

	// Successfully processed packages.
	// +optional
	Processed []PackageRepositoryOperationStatusPackage `json:"processed,omitempty"`

	// Total number of processed packages.
	// +optional
	ProcessedOverall int `json:"processedOverall,omitempty"`

	// Total number of discovered packages.
	// +optional
	Total int `json:"total,omitempty"`

	// Total number of new package versions discovered during the operation.
	//
	// A version is considered new if the corresponding ApplicationPackageVersion or ModulePackageVersion did not exist in the cluster.
	//
	// An ApplicationPackageVersion is also considered new if it was previously marked as missing from the container registry, but its image was discovered during the current operation.
	// +optional
	NewVersionsOverall int `json:"newVersionsOverall,omitempty"`
}

type PackageRepositoryOperationStatusDiscoveredPackage struct {
	// Name of the discovered package.
	Name string `json:"name"`
}

type PackageRepositoryOperationStatusFailedPackage struct {
	// Name of the package that could not be processed.
	Name string `json:"name"`

	// Errors that occurred while processing the package.
	Errors []PackageRepositoryOperationStatusFailedPackageError `json:"errors"`
}

type PackageRepositoryOperationStatusFailedPackageError struct {
	// Package version for which the error occurred.
	Version string `json:"version"`

	// Error message.
	Message string `json:"message"`
}

type PackageRepositoryOperationStatusPackage struct {
	// Name of the processed package.
	Name string `json:"name"`

	// Package type.
	// +optional
	Type string `json:"type,omitempty"`

	// Number of package versions discovered during the operation.
	// +optional
	FoundVersions int `json:"foundVersions,omitempty"`

	// Number of new package versions discovered during the operation.
	//
	// A version is considered new if the corresponding ApplicationPackageVersion or ModulePackageVersion did not exist in the cluster.
	//
	// An ApplicationPackageVersion is also considered new if it was previously marked as missing from the container registry, but its image was discovered during the current operation.
	// +optional
	NewVersions int `json:"newVersions,omitempty"`
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
