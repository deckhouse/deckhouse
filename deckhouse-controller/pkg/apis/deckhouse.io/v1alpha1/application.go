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
	ApplicationResource = "applications"
	ApplicationKind     = "Application"

	ApplicationConditionRequirementsMet        = "RequirementsMet"
	ApplicationConditionStartupHooksSuccessful = "StartupHooksSuccessful"
	ApplicationConditionManifestsDeployed      = "ManifestsDeployed"
	ApplicationConditionReplicasAvailable      = "ReplicasAvailable"

	// ApplicationConditionTypeProcessed changes only by application controller
	ApplicationConditionTypeProcessed                    = "Processed"
	ApplicationConditionReasonVersionNotFound            = "VersionNotFound"
	ApplicationConditionReasonApplicationPackageNotFound = "ApplicationPackageNotFound"
	ApplicationConditionReasonVersionIsDraft             = "VersionIsDraft"
	ApplicationConditionReasonVersionSpecIsCorrupted     = "VersionSpecIsCorrupted"

	// Application condition types
	ApplicationConditionInstalled            = "Installed"
	ApplicationConditionUpdateInstalled      = "UpdateInstalled"
	ApplicationConditionConfigurationApplied = "ConfigurationApplied"
	ApplicationConditionPartiallyDegraded    = "PartiallyDegraded"
	ApplicationConditionManaged              = "Managed"
	ApplicationConditionReady                = "Ready"

	// Application condition reasons
	ApplicationConditionInstalledReasonDownloading                              = "Downloading"
	ApplicationConditionInstalledReasonInstallationInProgress                   = "InstallationInProgress"
	ApplicationConditionInstalledReasonDownloadWasFailed                        = "DownloadWasFailed"
	ApplicationConditionInstalledReasonRequirementsNotMet                       = "RequirementsNotMet"
	ApplicationConditionInstalledReasonManifestsDeploymentFailed                = "ManifestsDeploymentFailed"
	ApplicationConditionInstalledReasonLicenseCheckFailed                       = "LicenseCheckFailed"
	ApplicationConditionInstalledReasonUpdateWasFailed                          = "UpdateWasFailed"
	ApplicationConditionUpdateInstalledReasonDownloading                        = "Downloading"
	ApplicationConditionUpdateInstalledReasonUpdateInProgress                   = "UpdateInProgress"
	ApplicationConditionUpdateInstalledReasonUpdateFailed                       = "UpdateFailed"
	ApplicationConditionUpdateInstalledReasonRequirementsNotMet                 = "RequirementsNotMet"
	ApplicationConditionConfigurationAppliedReasonConfigurationValidationFailed = "ConfigurationValidationFailed"
	ApplicationConditionPartiallyDegradedReasonScalingInProgress                = "ScalingInProgress"
	ApplicationConditionManagedReasonOperationFailed                            = "OperationFailed"
	ApplicationConditionReadyReasonNotReady                                     = "NotReady"

	ApplicationFinalizerStatisticRegistered = "application.deckhouse.io/statistic-registered"
)

var (
	ApplicationGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ApplicationResource,
	}
	ApplicationGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ApplicationKind,
	}
)

var _ runtime.Object = (*Application)(nil)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=app
// +kubebuilder:printcolumn:name=Package,type=string,JSONPath=.spec.packageName
// +kubebuilder:printcolumn:name=Age,type=date,JSONPath=.metadata.creationTimestamp
// +kubebuilder:printcolumn:name=Version,type=string,JSONPath=.spec.version
// +kubebuilder:printcolumn:name=Registry,type=string,JSONPath=.spec.packageRepository,priority=1
// +kubebuilder:printcolumn:name=Ready,type=string,JSONPath=.status.conditions[?(@.type=='Ready')].status
// +kubebuilder:printcolumn:name=Installed,type=string,JSONPath=.status.conditions[?(@.type=='Installed')].status
// +kubebuilder:printcolumn:name="Processed",type="string",JSONPath=".status.resourceConditions[?(@.type=='Processed')].status"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.resourceConditions[?(@.type=='Processed')].message"

// Application represents a namespace-scoped application instance.
type Application struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of an Application.
	Spec ApplicationSpec `json:"spec"`

	// Status of an Application.
	Status ApplicationStatus `json:"status,omitempty"`
}

type ApplicationSpec struct {
	// Name of the application package to install.
	PackageName string `json:"packageName"`

	// Name of the repository where the package is located.
	// If not specified, the default repository is used.
	// +optional
	PackageRepositoryName string `json:"packageRepositoryName,omitempty"`

	// Version of the application package to install.
	PackageVersion string `json:"packageVersion"`

	// Release channel for the application package.
	// +optional
	ReleaseChannel string `json:"releaseChannel,omitempty"`

	// Configuration settings for the application.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Settings *MappedFields `json:"settings,omitempty"`
}

type ApplicationStatus struct {
	// Information about the currently installed version.
	// +optional
	CurrentVersion *ApplicationStatusVersion `json:"currentVersion,omitempty"`

	// Conditions represent the latest available observations of the application's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []ApplicationStatusCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// InternalConditions represent internal conditions of the application.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	InternalConditions []ApplicationStatusInternalCondition `json:"internalConditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ResourceConditions represent conditions related to application resources.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	ResourceConditions []ApplicationStatusResourceCondition `json:"resourceConditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type ApplicationStatusVersion struct {
	// Semantic version of the installed application.
	// +optional
	Version string `json:"version,omitempty"`

	// Release channel from which the version was installed.
	// +optional
	Channel string `json:"channel,omitempty"`
}

type ApplicationStatusCondition struct {
	// Type of application condition.
	Type string `json:"type"`

	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`

	// Programmatic identifier indicating the reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Human readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty"`

	// Last time the condition was probed.
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`

	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

type ApplicationStatusInternalCondition struct {
	// Type of internal application condition.
	Type string `json:"type"`

	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`

	// Programmatic identifier indicating the reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Human readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty"`

	// Last time the condition was probed.
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`

	// The last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

type ApplicationStatusResourceCondition struct {
	// Type of resource condition.
	Type string `json:"type"`

	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`

	// Programmatic identifier indicating the reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Human readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty"`

	// Last time the condition was probed.
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`

	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationList is a list of Application resources
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Application `json:"items"`
}
