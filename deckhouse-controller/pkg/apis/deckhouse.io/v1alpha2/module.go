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

package v1alpha2

import (
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

const (
	ModuleResource = "modules"
	ModuleKind     = "Module"

	ModuleFinalizerStatisticRegistered = "module.deckhouse.io/statistic-registered"

	ModuleAnnotationRegistrySpecChanged = "packages.deckhouse.io/registry-spec-changed"

	// ModuleAnnotationDev marks a module restored from a development pull override.
	ModuleAnnotationDev = "modules.deckhouse.io/dev"

	// ModuleAnnotationHash holds the digest a dev module was last handed to the runtime on.
	ModuleAnnotationHash = "modules.deckhouse.io/hash"

	// ModuleAnnotationEmbedded marks a module that is embedded in the Deckhouse image.
	ModuleAnnotationEmbedded = "modules.deckhouse.io/embedded"
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
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name=Version,type=string,JSONPath=.spec.packageVersion
// +kubebuilder:printcolumn:name=Repository,type=string,JSONPath=.spec.packageRepositoryName,priority=1
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Module phase."
// +kubebuilder:printcolumn:name="Enabled",type="string",JSONPath=".status.conditions[?(@.type=='EnabledByModuleManager')].status",description="Module`s enabled status."
// +kubebuilder:printcolumn:name="Disabled Message",type="string",JSONPath=".status.conditions[?(@.type=='EnabledByModuleManager')].message",priority=1,description="Module`s enabled information."
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='IsReady')].status",description="Module`s ready status."
// +kubebuilder:metadata:labels="heritage=deckhouse"
// +kubebuilder:metadata:labels="app.kubernetes.io/name=deckhouse"
// +kubebuilder:metadata:labels="app.kubernetes.io/part-of=deckhouse"
// +crd-enricher:crd:preserveUnknownFields=false

// Module represents a module instance managed via the package system.
type Module struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a Module. A module a source offers and nothing
	// installed carries no package version, so the spec is optional.
	// +optional
	Spec ModuleSpec `json:"spec,omitempty"`

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
	// Empty while a module source offers the module and nothing installed it.
	// +crd-enricher:deckhouse:documentation:examples=v1.0.0
	// +optional
	PackageVersion string `json:"packageVersion,omitempty"`

	// Release channel for the module package.
	// +crd-enricher:deckhouse:documentation:examples=alpha
	// +optional
	ReleaseChannel string `json:"releaseChannel,omitempty"`

	// Update policy for the module package.
	// +crd-enricher:deckhouse:documentation:examples=test-alpha
	// +optional
	UpdatePolicy string `json:"updatePolicy,omitempty"`

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
	//   - All the module's resources are labeled with `maintenance: NoResourceReconciliation`.
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

type ModuleStatus struct {
	// Module phase.
	// +kubebuilder:validation:Enum=Unavailable;Available;Downloading;DownloadingError;Reconciling;Installing;HooksDisabled;WaitSyncTasks;Downloaded;Conflict;Ready;Error
	// +crd-enricher:deckhouse:documentation:examples=[Unavailable, Available, Downloading, DownloadingError, Reconciling, Installing, HooksDisabled, WaitSyncTasks, Downloaded, Conflict, Ready, Error]
	Phase string `json:"phase,omitempty"`

	// Hooks status report.
	// +optional
	HooksState string `json:"hooksState,omitempty"`

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

	// Conditions reflecting the latest observations of the application state.
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
	// Always one of: Pending, Failed, Updating, Ready, Degraded, Suspended.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=[Pending, Failed, Updating, Ready, Degraded, Suspended]
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

// IsDev returns true if the module is in dev mode.
func (m *Module) IsDev() bool {
	return m.Annotations[ModuleAnnotationDev] == "true"
}

// IsEmbedded returns true if the module is embedded in the Deckhouse image.
func (m *Module) IsEmbedded() bool {
	return m.Annotations[ModuleAnnotationEmbedded] == "true"
}

// GetVersion returns the version of the module package.
func (m *Module) GetVersion() string {
	return m.Spec.PackageVersion
}

// IsInstalled reports whether a package backs the module. A module a source offers and
// nothing installed carries no package version.
func (m *Module) IsInstalled() bool {
	return m.Spec.PackageVersion != ""
}

// HasCatalogPhase reports whether the phase is one a module nothing installed passes
// through: offered, in conflict between sources, or fetching its first release.
func (m *Module) HasCatalogPhase() bool {
	switch m.Status.Phase {
	case v1alpha1.ModulePhaseAvailable,
		v1alpha1.ModulePhaseConflict,
		v1alpha1.ModulePhaseDownloading,
		v1alpha1.ModulePhaseDownloadingError:
		return true
	}

	return false
}

// SetNotInstalledStatus marks the module as offered by a source and not installed.
func (m *Module) SetNotInstalledStatus() {
	m.Status.Phase = v1alpha1.ModulePhaseAvailable
	m.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleManager, v1alpha1.ModuleReasonDisabled, "")
	m.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonNotInstalled, v1alpha1.ModuleMessageNotInstalled)
}

// SetConflictStatus marks a module nothing installed as offered by several sources with
// none of them picked.
func (m *Module) SetConflictStatus() {
	m.Status.Phase = v1alpha1.ModulePhaseConflict
	m.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleManager, v1alpha1.ModuleReasonDisabled, "")
	m.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonConflict, v1alpha1.ModuleMessageConflict)
}

// IsCondition reports whether the named condition is present with the given status.
func (m *Module) IsCondition(condType string, status metav1.ConditionStatus) bool {
	cond := meta.FindStatusCondition(m.Status.Conditions, condType)

	return cond != nil && cond.Status == status
}

// HasCondition reports whether the named condition is present.
func (m *Module) HasCondition(condType string) bool {
	return meta.FindStatusCondition(m.Status.Conditions, condType) != nil
}

// DisabledByModuleConfigMoreThan reports whether the module config has kept the module
// disabled for at least the timeout.
func (m *Module) DisabledByModuleConfigMoreThan(timeout time.Duration) bool {
	cond := meta.FindStatusCondition(m.Status.Conditions, v1alpha1.ModuleConditionEnabledByModuleConfig)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		return false
	}

	return time.Since(cond.LastTransitionTime.Time) >= timeout
}

// +kubebuilder:object:generate=false
type ConditionOption func(opts *ConditionSettings)

// WithTimer overrides the clock the condition transition time is taken from.
func WithTimer(fn func() time.Time) ConditionOption {
	return func(opts *ConditionSettings) {
		opts.Timer = fn
	}
}

// +kubebuilder:object:generate=false
type ConditionSettings struct {
	Timer func() time.Time
}

// SetConditionTrue sets the condition to True. The reason must not be empty: the schema
// requires one on every condition.
func (m *Module) SetConditionTrue(condType, reason string, opts ...ConditionOption) {
	m.setCondition(condType, metav1.ConditionTrue, reason, "", opts)
}

// SetConditionFalse sets the condition to False with the reason and message.
func (m *Module) SetConditionFalse(condType, reason, message string, opts ...ConditionOption) {
	m.setCondition(condType, metav1.ConditionFalse, reason, message, opts)
}

// SetConditionUnknown sets the condition to Unknown with the reason and message.
func (m *Module) SetConditionUnknown(condType, reason, message string, opts ...ConditionOption) {
	m.setCondition(condType, metav1.ConditionUnknown, reason, message, opts)
}

// setCondition converges the condition. The transition time moves only when the status
// changes, so a repeated write of the same state leaves the object untouched.
func (m *Module) setCondition(condType string, status metav1.ConditionStatus, reason, message string, opts []ConditionOption) {
	settings := &ConditionSettings{Timer: time.Now}
	for _, opt := range opts {
		opt(settings)
	}

	meta.SetStatusCondition(&m.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Time{Time: settings.Timer()},
	})
}
