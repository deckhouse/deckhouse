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
	PackageRepositoryOperationResource = "packagerepositoryoperations"
	PackageRepositoryOperationKind     = "PackageRepositoryOperation"

	PackageRepositoryOperationPhasePending    = "Pending"
	PackageRepositoryOperationPhaseDiscover   = "Discover"
	PackageRepositoryOperationPhaseProcessing = "Processing"
	PackageRepositoryOperationPhaseCompleted  = "Completed"

	// PackageRepositoryOperation condition types
	PackageRepositoryOperationConditionProcessed = "Processed"

	// PackageRepositoryOperation condition reasons
	PackageRepositoryOperationReasonPackageRepositoryNotFound    = "PackageRepositoryNotFound"
	PackageRepositoryOperationReasonRegistryClientCreationFailed = "RegistryClientCreationFailed"
	PackageRepositoryOperationReasonPackageListingFailed         = "PackageListingFailed"

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
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name=Count,type=integer,JSONPath=.status.packages.total
// +kubebuilder:printcolumn:name=Processed,type=string,JSONPath=.status.conditions[?(@.type=='Processed')].status

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
	Phase          string                                      `json:"phase,omitempty"`
	StartTime      *metav1.Time                                `json:"startTime,omitempty"`
	CompletionTime *metav1.Time                                `json:"completionTime,omitempty"`
	Packages       *PackageRepositoryOperationStatusPackages   `json:"packages,omitempty"`
	Conditions     []PackageRepositoryOperationStatusCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type PackageRepositoryOperationStatusPackages struct {
	Discovered       []PackageRepositoryOperationStatusDiscoveredPackage `json:"discovered,omitempty"`
	Failed           []PackageRepositoryOperationStatusFailedPackage     `json:"failed,omitempty"`
	Processed        []PackageRepositoryOperationStatusPackage           `json:"processed,omitempty"`
	ProcessedOverall int                                                 `json:"processedOverall,omitempty"`
	Total            int                                                 `json:"total,omitempty"`
}

type PackageRepositoryOperationStatusDiscoveredPackage struct {
	Name string `json:"name"`
}

type PackageRepositoryOperationStatusFailedPackage struct {
	Name   string                                               `json:"name"`
	Errors []PackageRepositoryOperationStatusFailedPackageError `json:"errors"`
}

type PackageRepositoryOperationStatusFailedPackageError struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

type PackageRepositoryOperationStatusPackage struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type PackageRepositoryOperationStatusCondition struct {
	Type               string                 `json:"type"`
	Status             corev1.ConditionStatus `json:"status"`
	Reason             string                 `json:"reason,omitempty"`
	Message            string                 `json:"message,omitempty"`
	LastProbeTime      metav1.Time            `json:"lastProbeTime,omitempty"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty"`
}

// +kubebuilder:object:root=true

// PackageRepositoryOperationList is a list of PackageRepositoryOperation resources
type PackageRepositoryOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PackageRepositoryOperation `json:"items"`
}
