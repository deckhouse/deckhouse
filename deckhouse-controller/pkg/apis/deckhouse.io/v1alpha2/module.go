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
// +kubebuilder:printcolumn:name=Version,type=string,JSONPath=.spec.packageVersion
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

	// Version of the module package to install
	// +crd-enricher:deckhouse:documentation:examples=v1.0.0.
	PackageVersion string `json:"packageVersion"`

	// Release channel for the module package.
	// +optional
	ReleaseChannel string `json:"releaseChannel,omitempty"`

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
