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
	ApplicationConditionTypeProcessed                = "Processed"
	ApplicationConditionReasonVersionNotFound        = "VersionNotFound"
	ApplicationConditionReasonVersionIsDraft         = "VersionIsDraft"
	ApplicationConditionReasonVersionSpecIsCorrupted = "VersionSpecIsCorrupted"

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
// +kubebuilder:resource:scope=Namespaced

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
	PackageName       string          `json:"packageName"`
	PackageRepository string          `json:"packageRepository,omitempty"`
	Version           string          `json:"version"`
	ReleaseChannel    string          `json:"releaseChannel,omitempty"`
	Settings          *SettingsValues `json:"settings,omitempty"`
}

type ApplicationStatus struct {
	CurrentVersion     *ApplicationStatusVersion            `json:"currentVersion,omitempty"`
	Repository         string                               `json:"repository,omitempty"`
	Status             string                               `json:"status,omitempty"`
	Conditions         []ApplicationStatusCondition         `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	InternalConditions []ApplicationInternalStatusCondition `json:"internalConditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type ApplicationStatusVersion struct {
	Current string `json:"current,omitempty"`
	Channel string `json:"channel,omitempty"`
}

type ApplicationStatusCondition struct {
	Type               string                 `json:"type"`
	Status             corev1.ConditionStatus `json:"status"`
	Reason             string                 `json:"reason,omitempty"`
	Message            string                 `json:"message,omitempty"`
	LastProbeTime      metav1.Time            `json:"lastProbeTime,omitempty"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty"`
}

type ApplicationInternalStatusCondition struct {
	Type               string                 `json:"type"`
	Status             corev1.ConditionStatus `json:"status"`
	LastProbeTime      metav1.Time            `json:"lastProbeTime,omitempty"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationList is a list of Application resources
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Application `json:"items"`
}
