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
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name=Package,type=string,JSONPath=.spec.packageName
// +kubebuilder:printcolumn:name=Version,type=string,JSONPath=.spec.packageVersion
// +kubebuilder:printcolumn:name=Repository,type=string,JSONPath=.spec.packageRepositoryName,priority=1
// +kubebuilder:printcolumn:name=State,type=string,JSONPath=.status.summary.state
// +kubebuilder:printcolumn:name=Installed,type=string,JSONPath=.status.conditions[?(@.type=='Installed')].status,priority=1
// +kubebuilder:printcolumn:name=Ready,type=string,JSONPath=.status.conditions[?(@.type=='Ready')].status,priority=1
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.summary.message"
// +kubebuilder:printcolumn:name=Age,type=date,JSONPath=.metadata.creationTimestamp
// +crd-enricher:raw:properties.apiVersion.description="APIVersion defines the versioned schema of this representation of an object.\nServers should convert recognized schemas to the latest internal value, and\nmay reject unrecognized values.\n\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)."
// +crd-enricher:raw:properties.kind.description="Kind is a string value representing the REST resource this object represents.\nServers may infer this from the endpoint the client submits requests to.\nCannot be updated.\nIn CamelCase.\n\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds)."
// +crd-enricher:raw:properties.apiVersion.x-doc-skip=true
// +crd-enricher:raw:properties.kind.x-doc-skip=true
// +crd-enricher:raw:properties.metadata.x-doc-skip=true

// +crd-enricher:deckhouse:documentation:examples={apiVersion: deckhouse.io/v1alpha1, kind: Application, metadata: {name: example}, spec: {maintenance: NoResourceReconciliation, packageName: console, packageRepositoryName: deckhouse, packageVersion: v1.0.0, releaseChannel: stable}}
// Application defines an installed application instance in a namespace.
type Application struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Application configuration.
	Spec ApplicationSpec `json:"spec"`

	// Current state of the application.
	Status ApplicationStatus `json:"status,omitempty"`
}

type ApplicationSpec struct {
	// Name of the application package to install.
	// +crd-enricher:deckhouse:documentation:examples=console
	PackageName string `json:"packageName"`

	// Name of the PackageRepository object where the application package is located.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=deckhouse
	PackageRepositoryName string `json:"packageRepositoryName,omitempty"`

	// Version of the application package to install.
	// +crd-enricher:deckhouse:documentation:examples=v1.0.0
	PackageVersion string `json:"packageVersion"`

	// Application update settings: update mode, update windows, and blocking updates when alerts are present.
	//
	// If this parameter is not specified, the global [`applications.update`](global.html#parameters-applications-update) settings are used. If the global settings are also not specified, the Deckhouse Platform update settings are applied, as defined by the [`update`](/modules/deckhouse/configuration.html#parameters-update) parameter of the `deckhouse` module.
	//
	// Update notifications cannot be configured separately for an application. If `applications.update` is configured, the global [`applications.update.notification`](global.html#parameters-applications-update-notification) settings are used. Otherwise, the Deckhouse Platform notification settings apply.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples={mode: Auto, windows: [{from: "8:00", to: "15:00", days: [Tue, Sat]}], blockOnAlerts: {enabled: true, severity: 2}}
	Update *ApplicationUpdate `json:"update,omitempty"`

	// Configuration application settings.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Settings *MappedFields `json:"settings,omitempty"`

	// Application maintenance mode.
	//
	// Supported modes:
	//
	// - `NoResourceReconciliation`: A mode for developing or debugging the application.
	//
	//   In this mode:
	//
	//   - Configuration or hook changes are not reconciled, which prevents resources from being updated automatically.
	//   - Resource monitoring is disabled, which prevents deleted resources from being restored.
	//   - All the application's resources are labeled with `maintenance.deckhouse.io/no-resource-reconciliation`.
	//   - The [`ApplicationIsInMaintenanceMode`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#deckhouse-applicationisinmaintenancemode) alert is triggered.
	// +kubebuilder:validation:Enum=NoResourceReconciliation
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=NoResourceReconciliation
	Maintenance string `json:"maintenance,omitempty"`

	// Resource settings and replica counts for workloads deployed by the application.
	//
	// Specified values override the settings provided by the application package. Workloads and containers not listed in this parameter retain their settings from the package.
	//
	// Each workload is identified by a combination of kind (`kind`) and name (`name`). Only one entry is allowed for each combination.
	// +listType=map
	// +listMapKey=kind
	// +listMapKey=name
	// +optional
	ResourceRequests []ApplicationResourceRequest `json:"resourceRequests,omitempty"`
}

// ApplicationUpdate is the update policy of a single application. It mirrors the global
// applications.update settings, except for the notification ones.
type ApplicationUpdate struct {
	// Update mode of the application on its [release channel](#application-v1alpha1-spec-releasechannel).
	//
	// - `AutoPatch`: Automatic update mode for patch versions.
	//
	//   To change a minor version (for example, from `v1.2.*` to `v1.3.*`), confirmation is required.
	//
	//   A patch version update (for example, from `v1.3.1` to `v1.3.2`) is applied taking into account the [update windows](#application-v1alpha1-spec-update-windows), if they are set.
	//
	// - `Auto`: Automatic update mode for all versions.
	//
	//   Minor version updates (for example, from `v1.2.*` to `v1.3.*`) and patch version updates (for example, from `v1.3.1` to `v1.3.2`) are applied taking into account the [update windows](#application-v1alpha1-spec-update-windows), if they are set.
	//
	// - `Manual`: Manual update mode for all versions.
	//
	//   Confirmation is required for updating both minor and patch versions.
	// +kubebuilder:validation:Enum=AutoPatch;Auto;Manual
	// +kubebuilder:default=AutoPatch
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=[AutoPatch, Auto, Manual]
	Mode string `json:"mode,omitempty"`

	// List of application update windows.
	// +optional
	Windows []ApplicationUpdateWindow `json:"windows,omitempty"`

	// Settings for blocking application updates while there are alerts in the cluster.
	// +optional
	BlockOnAlerts *ApplicationUpdateBlockOnAlerts `json:"blockOnAlerts,omitempty"`
}

// Time interval during the day when application updates are allowed.
type ApplicationUpdateWindow struct {
	// Start time of the update window (UTC timezone).
	//
	// Should be earlier than the end time of the update window.
	// +kubebuilder:validation:Pattern=`^(?:\d|[01]\d|2[0-3]):[0-5]\d$`
	// +crd-enricher:deckhouse:documentation:examples="13:00"
	From string `json:"from"`

	// End time of the update window (UTC timezone).
	//
	// Should be later than the start time of the update window.
	// +kubebuilder:validation:Pattern=`^(?:\d|[01]\d|2[0-3]):[0-5]\d$`
	// +crd-enricher:deckhouse:documentation:examples="18:30"
	To string `json:"to"`

	// Days of the week on which the update window is applied.
	//
	// If the parameter is omitted, the update window is applied every day.
	// +kubebuilder:validation:items:Enum=Mon;Tue;Wed;Thu;Fri;Sat;Sun
	// +optional
	Days []string `json:"days,omitempty"`
}

// ApplicationUpdateBlockOnAlerts holds the settings for blocking the application update on alerts.
type ApplicationUpdateBlockOnAlerts struct {
	// Indicates whether to block application updates when the cluster has alerts with a severity level equal to or higher than the value specified in the [`severity`](#application-v1alpha1-spec-update-blockonalerts-severity) parameter.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Alert severity level at which updates are blocked.
	//
	// Set as a value from `0` to `9`, where `0` is the highest severity and `9` is the lowest.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=9
	// +kubebuilder:default=4
	// +optional
	Severity *int32 `json:"severity,omitempty"`
}

// Resource settings and replica count for a single application workload.
// +kubebuilder:validation:XValidation:rule="has(self.replicas) ? self.kind in ['Deployment','StatefulSet','ReplicaSet'] : true",message="replicas can only be set for Deployment, StatefulSet and ReplicaSet"
type ApplicationResourceRequest struct {
	// Kind of workload for which resource settings are specified.
	// +kubebuilder:validation:Enum=Deployment;StatefulSet;DaemonSet;ReplicaSet;Pod;Job;CronJob
	// +crd-enricher:deckhouse:documentation:examples=Deployment
	Kind string `json:"kind"`

	// Workload name as specified in the application package.
	// +kubebuilder:validation:MinLength=1
	// +crd-enricher:deckhouse:documentation:examples=controller
	Name string `json:"name"`

	// Number of workload replicas.
	//
	// Applies only to workloads that support replica management: Deployment, StatefulSet, and ReplicaSet.
	// +kubebuilder:validation:Minimum=0
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=3
	Replicas *int32 `json:"replicas,omitempty"`

	// Compute resources of the workload's containers.
	//
	// If the specified container does not exist in the workload, the entry is ignored.
	// +listType=map
	// +listMapKey=name
	// +optional
	Containers []ApplicationContainerResourceRequest `json:"containers,omitempty"`
}

// Resource settings for a single workload container.
type ApplicationContainerResourceRequest struct {
	// Name of the container inside the workload.
	// +kubebuilder:validation:MinLength=1
	// +crd-enricher:deckhouse:documentation:examples=controller
	Name string `json:"name"`

	// Container resource requests and limits.
	//
	// Uses the same format as the `resources` field of a Pod container. Supports `cpu`, `memory`, and `ephemeral-storage`.
	//
	// Specified values are merged with the settings from the application package separately for each resource. For example, if only `memory` is specified, the `cpu` value from the package is retained.
	//
	// Resource requests or limits defined in the package cannot be removed. Their values can only be overridden.
	// +optional
	// +crd-enricher:raw:properties.claims.description="List of resource claims used by the container.\n\nEach entry references a resource claim defined in `pod.spec.resourceClaims`. Requires the `DynamicResourceAllocation` feature gate to be enabled.\n\nThis field is immutable. It can only be set for containers."
	// +crd-enricher:raw:properties.claims.items.description="Resource claim used by the container."
	// +crd-enricher:raw:properties.claims.items.properties.name.description="Name of a ResourceClaim defined in `pod.spec.resourceClaims`.\n\nThe referenced claim makes its resources available to the container."
	// +crd-enricher:raw:properties.claims.items.properties.request.description="Name of a request defined in the referenced resource claim.\n\nIf specified, only the resources allocated for this request are made available to the container. If omitted, all resources allocated for the claim are made available."
	// +crd-enricher:raw:properties.limits.description="Maximum amount of compute resources available to the container."
	// +crd-enricher:raw:properties.requests.description="Minimum amount of compute resources required by the container.\n\nIf this parameter is not specified, the `limits` value is used if explicitly set. Otherwise, the value is implementation-defined.\n\nThe `requests` value cannot exceed `limits`."
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type ApplicationStatus struct {
	// Summary of the application state, including messages and recommendations for the user.
	//
	// The controller updates this information during each reconciliation. Each application has exactly one lifecycle state.
	//
	// Use this information to display the application state in the web interface. Do not recalculate the state from conditions.
	// +optional
	Summary *ApplicationStatusSummary `json:"summary,omitempty"`

	// Information about the currently installed application version.
	// +optional
	CurrentVersion *ApplicationStatusVersion `json:"currentVersion,omitempty"`

	// Application endpoint URLs obtained from Ingress resources in the application chart with the `packages.deckhouse.io/application-endpoint-description` annotation.
	// +optional
	URLs []ApplicationStatusURL `json:"urls,omitempty"`

	// Nelm report on application status.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Tracking runtime.RawExtension `json:"tracking"`

	// Application configuration used during the last successful apply.
	//
	// Includes user-defined settings and default values defined in the configuration schema.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	LastAppliedConfiguration runtime.RawExtension `json:"lastAppliedConfiguration"`

	// Conditions reflecting the current state of the application.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +crd-enricher:raw:items.description="Condition reflecting an aspect of the current state of the application."
	// +crd-enricher:raw:items.properties.lastTransitionTime.description="Time when the condition status last changed."
	// +crd-enricher:raw:items.properties.message.description="Message with additional information about the condition."
	// +crd-enricher:raw:items.properties.observedGeneration.description="Value of `.metadata.generation` for which the condition was set.\n\nIndicates whether the condition corresponds to the current generation of the instance."
	// +crd-enricher:raw:items.properties.reason.description="Programmatic identifier indicating the reason for the last condition status change."
	// +crd-enricher:raw:items.properties.status.description="Condition status."
	// +crd-enricher:raw:items.properties.type.description="Condition type in CamelCase or in the format `foo.example.com/CamelCase`."
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// ApplicationStatusSummary aggregates the high-level lifecycle state, message
// and resolution hint for the application. It is consumed by the UI as a single
// source of truth so that the frontend does not have to re-implement the state
// machine on top of conditions.
type ApplicationStatusSummary struct {
	// Current application lifecycle state.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=[Pending, Failed, Updating, Ready, Degraded, Suspended, Deleting]
	State string `json:"state,omitempty"`

	// Description of the current application state.
	// +optional
	Message string `json:"message,omitempty"`

	// Recommended action for the current application state.
	//
	// May be empty if no additional action is required.
	// +optional
	Tip string `json:"tip,omitempty"`
}

// Application endpoint defined by an Ingress resource in the application chart.
type ApplicationStatusURL struct {
	// Application endpoint URL.
	URL string `json:"url"`

	// Endpoint description from the `packages.deckhouse.io/application-endpoint-description` annotation.
	//
	// The description is empty if the annotation value is `"true"`.
	// +optional
	Description string `json:"description,omitempty"`
}

// ApplicationStatusVersion is the installed version; an object, so status stored by earlier releases still decodes.
type ApplicationStatusVersion struct {
	// Installed application version in SemVer format.
	// +optional
	Version string `json:"version,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationList is a list of Application resources
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Application `json:"items"`
}
