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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

const (
	ModuleResource = "modules"
	ModuleKind     = "Module"

	ModuleFinalizerStatisticRegistered = "module.deckhouse.io/statistic-registered"

	// ModuleAnnotationEmbedded marks a module that is embedded in the Deckhouse image.
	ModuleAnnotationEmbedded = "modules.deckhouse.io/embedded"

	// ModuleAnnotationCritical marks a module whose definition declares it critical.
	ModuleAnnotationCritical = "modules.deckhouse.io/critical"
)

var (
	ModuleGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ModuleResource,
	}
	ModuleGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ModuleKind,
	}
)

var _ runtime.Object = (*Module)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:unservedversion
// +kubebuilder:printcolumn:name=Version,type=string,JSONPath=.status.currentVersion.version
// +kubebuilder:printcolumn:name=Repository,type=string,JSONPath=.spec.packageRepositoryName,priority=1
// +kubebuilder:printcolumn:name=State,type=string,JSONPath=.status.summary.state
// +kubebuilder:printcolumn:name=Installed,type=string,JSONPath=.status.conditions[?(@.type=='Installed')].status,priority=1
// +kubebuilder:printcolumn:name=Ready,type=string,JSONPath=.status.conditions[?(@.type=='Ready')].status,priority=1
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.summary.message"
// +kubebuilder:printcolumn:name=Age,type=date,JSONPath=.metadata.creationTimestamp
// +crd-enricher:crd:preserveUnknownFields=false

// Module represents a module instance managed via the package system.
type Module struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a Module.
	Spec ModuleSpec `json:"spec"`

	// Status of a Module.
	Status ModuleStatus `json:"status,omitempty"`
}

type ModuleSpec struct {
	// Name of the repository where the package is located.
	// If not specified, the default repository is used.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=deckhouse
	PackageRepositoryName string `json:"packageRepositoryName,omitempty"`

	// Version of the module package to install.
	// +crd-enricher:deckhouse:documentation:examples=v1.0.0
	PackageVersion string `json:"packageVersion"`

	// Release channel for the module package.
	// +optional
	ReleaseChannel string `json:"releaseChannel,omitempty"`

	// Module update settings: update mode, update windows, and blocking updates when alerts are present.
	//
	// If this parameter is not specified, the Deckhouse Platform update settings are applied.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples={mode: Auto, windows: [{from: "8:00", to: "15:00", days: [Tue, Sat]}], blockOnAlerts: {enabled: true, severity: 2}}
	Update *ModuleUpdate `json:"update,omitempty"`

	// Enables or disables the module. Unset leaves the decision to the platform.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Defines the module maintenance mode.
	//
	// - `NoResourceReconciliation`: A mode for developing or tweaking the module.
	//
	//   In this mode:
	//
	//   - Configuration or hook changes are not reconciled, which prevents resources from being updated automatically.
	//   - Resource monitoring is disabled, which prevents deleted resources from being restored.
	//   - All the module's resources are labeled with `maintenance.deckhouse.io/no-resource-reconciliation`.
	//   - The `ModuleIsInMaintenanceMode` alert is triggered.
	// +kubebuilder:validation:Enum=NoResourceReconciliation
	// +optional
	Maintenance string `json:"maintenance,omitempty"`

	// Version of the settings schema. Distinct from packageVersion, which selects the module package.
	// +kubebuilder:validation:Type=number
	// +optional
	SettingsVersion int `json:"settingsVersion,omitempty"`

	// Configuration settings for the module.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Settings *v1alpha1.MappedFields `json:"settings,omitempty"`
}

// ModuleUpdate is the update policy of a single module.
type ModuleUpdate struct {
	// Update mode of the module on its release channel.
	//
	// - `AutoPatch`: Automatic update mode for patch versions.
	//
	//   To change a minor version (for example, from `v1.2.*` to `v1.3.*`), confirmation is required.
	//
	//   A patch version update (for example, from `v1.3.1` to `v1.3.2`) is applied taking into account the update windows, if they are set.
	//
	// - `Auto`: Automatic update mode for all versions.
	//
	//   Minor version updates (for example, from `v1.2.*` to `v1.3.*`) and patch version updates (for example, from `v1.3.1` to `v1.3.2`) are applied taking into account the update windows, if they are set.
	//
	// - `Manual`: Manual update mode for all versions.
	//
	//   Confirmation is required for updating both minor and patch versions.
	// +kubebuilder:validation:Enum=AutoPatch;Auto;Manual
	// +kubebuilder:default=AutoPatch
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=[AutoPatch, Auto, Manual]
	Mode string `json:"mode,omitempty"`

	// List of module update windows.
	// +optional
	Windows []ModuleUpdateWindow `json:"windows,omitempty"`

	// Settings for blocking module updates while there are alerts in the cluster.
	// +optional
	BlockOnAlerts *ModuleUpdateBlockOnAlerts `json:"blockOnAlerts,omitempty"`
}

// Time interval during the day when module updates are allowed.
type ModuleUpdateWindow struct {
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

// ModuleUpdateBlockOnAlerts holds the settings for blocking the module update on alerts.
type ModuleUpdateBlockOnAlerts struct {
	// Indicates whether to block module updates when the cluster has alerts with a severity level equal to or higher than the value specified in the `severity` parameter.
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

type ModuleStatus struct {
	// Reports whether the module is enabled: `spec.enabled` resolved against the
	// edition bundle, the enabled-script and the module dependencies. It can differ
	// from `spec.enabled` — an explicitly enabled module stays disabled while a
	// module it requires is disabled. Absent until the first decision.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Summary aggregates the high-level user-facing state, message and
	// resolution hint for the module. The controller always populates it
	// on reconcile — every module maps to exactly one lifecycle state — so
	// it is the single source of truth for the UI; clients should not re-derive
	// these values from the conditions. The pointer leaves it absent only
	// before the first status computation.
	// +optional
	Summary *ModuleStatusSummary `json:"summary,omitempty"`

	// Information about the currently installed version.
	// +optional
	CurrentVersion *ModuleStatusVersion `json:"currentVersion,omitempty"`

	// Nelm tracking.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Tracking runtime.RawExtension `json:"tracking"`

	// LastAppliedConfiguration is the effective settings (user configuration merged
	// with config-schema defaults) that drove the most recent successful apply.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	LastAppliedConfiguration runtime.RawExtension `json:"lastAppliedConfiguration"`

	// Conditions reflecting the latest observations of the module state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// ModuleStatusSummary aggregates the high-level lifecycle state, message
// and resolution hint for the module. It is consumed by the UI as a single
// source of truth so that the frontend does not have to re-implement the state
// machine on top of conditions.
type ModuleStatusSummary struct {
	// State is the high-level lifecycle state observed for the module.
	// Always one of: Pending, Failed, Updating, Configuring, Ready, Degraded, Suspended, Deleting.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=[Pending, Failed, Updating, Configuring, Ready, Degraded, Suspended, Deleting]
	State string `json:"state,omitempty"`

	// Message is a human-readable description of the current state.
	// +optional
	Message string `json:"message,omitempty"`

	// Tip is a human-readable instruction on how to resolve the current
	// state. Empty when no action is required.
	// +optional
	Tip string `json:"tip,omitempty"`
}

type ModuleStatusVersion struct {
	// Semantic version of the installed module.
	// +optional
	Version string `json:"version,omitempty"`

	// Release channel from which the version was installed.
	// +optional
	Channel string `json:"channel,omitempty"`
}

// +kubebuilder:object:root=true

// ModuleList is a list of Module resources
type ModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Module `json:"items"`
}

// IsEmbedded returns true if the module is embedded in the Deckhouse image.
func (m *Module) IsEmbedded() bool {
	return m.Annotations[ModuleAnnotationEmbedded] == "true"
}
