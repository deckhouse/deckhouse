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
	ApplicationResource = "applications"
	ApplicationKind     = "Application"

	ApplicationFinalizerStatisticRegistered = "application.deckhouse.io/statistic-registered"

	ApplicationAnnotationRegistrySpecChanged = "packages.deckhouse.io/registry-spec-changed"

	// ApplicationAnnotationEndpointDescription marks an Ingress in the application
	// chart as an application endpoint and holds its description; the hosts and
	// paths of that Ingress are reflected in status.urls.
	ApplicationAnnotationEndpointDescription = "packages.deckhouse.io/application-endpoint-description"
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

// +crd-enricher:deckhouse:documentation:examples={apiVersion: deckhouse.io/v1alpha1, kind: Application, metadata: {name: example}, spec: {maintenance: NoResourceReconciliation, packageName: console, packageRepositoryName: deckhouse, packageVersion: v1.0.0, releaseChannel: stable}}
// Application represents a namespace-scoped application instance.
type Application struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the application configuration.
	Spec ApplicationSpec `json:"spec"`

	// Application status.
	Status ApplicationStatus `json:"status,omitempty"`
}

type ApplicationSpec struct {
	// Name of the application package to install.
	// +crd-enricher:deckhouse:documentation:examples=console
	PackageName string `json:"packageName"`

	// Name of the PackageRepository where the package is located.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=deckhouse
	PackageRepositoryName string `json:"packageRepositoryName,omitempty"`

	// Version of the application package to install.
	// +crd-enricher:deckhouse:documentation:examples=v1.0.0
	PackageVersion string `json:"packageVersion"`

	// Release channel for the application package.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=stable
	ReleaseChannel string `json:"releaseChannel,omitempty"`

	// Update settings of the application: the update mode, update windows, and blocking the update on alerts.
	//
	// If the parameter is omitted, the application uses the global [`applications.update`](global.html#parameters-applications-update) settings. If those are not set either, it uses the Deckhouse update settings: the [`update`](/modules/deckhouse/configuration.html#parameters-update) parameter of the `deckhouse` module.
	//
	// Update notifications cannot be configured for a single application. The application always uses the global ones: [`applications.update.notification`](global.html#parameters-applications-update-notification) if the global `applications.update` settings are set, and the Deckhouse notification settings otherwise.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples={mode: Auto, windows: [{from: "8:00", to: "15:00", days: [Tue, Sat]}], blockOnAlerts: {enabled: true, severity: 2}}
	Update *ApplicationUpdate `json:"update,omitempty"`

	// Configuration settings for the application.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Settings *MappedFields `json:"settings,omitempty"`

	// Defines the application maintenance mode.
	//
	// - `NoResourceReconciliation`: A mode for developing or tweaking the application.
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
	// - `Auto`: Automatic update mode for all versions.
	//
	//   Minor version updates (for example, from `v1.2.*` to `v1.3.*`) and patch version updates (for example, from `v1.3.1` to `v1.3.2`) are applied taking into account the [update windows](#application-v1alpha1-spec-update-windows), if they are set.
	// - `Manual`: Manual update mode for all versions.
	//
	//   Confirmation is required for updating both minor and patch versions.
	// +kubebuilder:validation:Enum=AutoPatch;Auto;Manual
	// +kubebuilder:default=AutoPatch
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=[AutoPatch, Auto, Manual]
	Mode string `json:"mode,omitempty"`

	// List of update windows during the day.
	// +optional
	Windows []ApplicationUpdateWindow `json:"windows,omitempty"`

	// Settings for blocking the application update while there are alerts in the cluster.
	// +optional
	BlockOnAlerts *ApplicationUpdateBlockOnAlerts `json:"blockOnAlerts,omitempty"`
}

// ApplicationUpdateWindow is a time range of a day in which the application may be updated.
type ApplicationUpdateWindow struct {
	// Start time of the update window (UTC timezone).
	//
	// Should be less than the end time of the update window.
	// +kubebuilder:validation:Pattern=`^(?:\d|[01]\d|2[0-3]):[0-5]\d$`
	// +crd-enricher:deckhouse:documentation:examples="13:00"
	From string `json:"from"`

	// End time of the update window (UTC timezone).
	//
	// Should be more than the start time of the update window.
	// +kubebuilder:validation:Pattern=`^(?:\d|[01]\d|2[0-3]):[0-5]\d$`
	// +crd-enricher:deckhouse:documentation:examples="18:30"
	To string `json:"to"`

	// The days of the week on which the update window is applied.
	//
	// If the parameter is omitted, the update window is applied every day.
	// +kubebuilder:validation:items:Enum=Mon;Tue;Wed;Thu;Fri;Sat;Sun
	// +optional
	Days []string `json:"days,omitempty"`
}

// ApplicationUpdateBlockOnAlerts holds the settings for blocking the application update on alerts.
type ApplicationUpdateBlockOnAlerts struct {
	// If enabled, the application update is blocked while there are alerts in the cluster with a severity level equal to or higher than the one set in the [`severity`](#application-v1alpha1-spec-update-blockonalerts-severity) parameter.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// The severity level of the alerts that block the update.
	//
	// The severity level is a number from 0 to 9, where 0 is the highest severity and 9 is the lowest severity.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=9
	// +kubebuilder:default=4
	// +optional
	Severity *int32 `json:"severity,omitempty"`
}

type ApplicationStatus struct {
	// Summary aggregates the high-level user-facing state, message and
	// resolution hint for the application. The controller always populates it
	// on reconcile — every application maps to exactly one lifecycle state — so
	// it is the single source of truth for the UI; clients should not re-derive
	// these values from the conditions. The pointer leaves it absent only
	// before the first status computation.
	// +optional
	Summary *ApplicationStatusSummary `json:"summary,omitempty"`

	// Information about the currently installed version.
	// +optional
	CurrentVersion *ApplicationStatusVersion `json:"currentVersion,omitempty"`

	// URLs of application endpoints, collected from Ingress resources of the
	// application chart annotated with `packages.deckhouse.io/application-endpoint-description`.
	// +optional
	URLs []ApplicationStatusURL `json:"urls,omitempty"`

	// Nelm tracking.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Tracking runtime.RawExtension `json:"tracking"`

	// LastAppliedConfiguration is the effective settings (user configuration merged
	// with config-schema defaults) that drove the most recent successful apply.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	LastAppliedConfiguration runtime.RawExtension `json:"lastAppliedConfiguration"`

	// Conditions reflecting the latest observations of the application state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// ApplicationStatusSummary aggregates the high-level lifecycle state, message
// and resolution hint for the application. It is consumed by the UI as a single
// source of truth so that the frontend does not have to re-implement the state
// machine on top of conditions.
type ApplicationStatusSummary struct {
	// State is the high-level lifecycle state observed for the application.
	// Always one of: Pending, Failed, Updating, Ready, Degraded, Suspended, Deleting.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=[Pending, Failed, Updating, Ready, Degraded, Suspended, Deleting]
	State string `json:"state,omitempty"`

	// Message is a human-readable description of the current state.
	// +optional
	Message string `json:"message,omitempty"`

	// Tip is a human-readable instruction on how to resolve the current
	// state. Empty when no action is required.
	// +optional
	Tip string `json:"tip,omitempty"`
}

// ApplicationStatusURL is a single application endpoint built from an Ingress
// of the application chart.
type ApplicationStatusURL struct {
	// URL of the application endpoint.
	URL string `json:"url"`

	// Description of the endpoint, taken from the value of the
	// `packages.deckhouse.io/application-endpoint-description` annotation.
	//
	// Empty when the annotation value is "true".
	// +optional
	Description string `json:"description,omitempty"`
}

type ApplicationStatusVersion struct {
	// Semantic version of the installed application.
	// +optional
	Version string `json:"version,omitempty"`

	// Release channel from which the version was installed.
	// +optional
	Channel string `json:"channel,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationList is a list of Application resources
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Application `json:"items"`
}
