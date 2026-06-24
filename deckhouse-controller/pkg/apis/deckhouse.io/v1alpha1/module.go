/*
Copyright 2023 Flant JSC

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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ModuleResource = "modules"
	ModuleKind     = "Module"

	ModuleSourceEmbedded = "Embedded"

	ModuleAnnotationDescriptionRu = "ru.meta.deckhouse.io/description"
	ModuleAnnotationDescriptionEn = "en.meta.deckhouse.io/description"

	ModuleConditionEnabledByModuleConfig  = "EnabledByModuleConfig"
	ModuleConditionEnabledByModuleManager = "EnabledByModuleManager"
	ModuleConditionLastReleaseDeployed    = "LastReleaseDeployed"
	ModuleConditionIsReady                = "IsReady"
	ModuleConditionIsOverridden           = "IsOverridden"

	ModulePhaseUnavailable      = "Unavailable"
	ModulePhaseAvailable        = "Available"
	ModulePhaseDownloading      = "Downloading"
	ModulePhaseDownloadingError = "DownloadingError"
	ModulePhaseReconciling      = "Reconciling"
	ModulePhaseInstalling       = "Installing"
	ModulePhaseDownloaded       = "Downloaded"
	ModulePhaseConflict         = "Conflict"
	ModulePhaseReady            = "Ready"
	ModulePhaseError            = "Error"

	ModuleReasonBundle                    = "Bundle"
	ModuleReasonModuleConfig              = "ModuleConfig"
	ModuleReasonDynamicGlobalHookExtender = "DynamicGlobalHookExtender"
	ModuleReasonEnabledScriptExtender     = "EnabledScriptExtender"
	ModuleReasonDeckhouseVersionExtender  = "DeckhouseVersionExtender"
	ModuleReasonKubernetesVersionExtender = "KubernetesVersionExtender"
	ModuleReasonBootstrappedExtender      = "BootstrappedExtender"
	ModuleReasonModuleDependencyExtender  = "ModuleDependencyExtender"
	ModuleReasonEditionAvailableExtender  = "EditionAvailableExtender"
	ModuleReasonEditionEnabledExtender    = "EditionEnabledExtender"
	ModuleReasonNotInstalled              = "NotInstalled"
	ModuleReasonDisabled                  = "Disabled"
	ModuleReasonConflict                  = "Conflict"
	ModuleReasonDownloading               = "Downloading"
	ModuleReasonHookError                 = "HookError"
	ModuleReasonModuleError               = "ModuleError"
	ModuleReasonReconciling               = "Reconciling"
	ModuleReasonInstalling                = "Installing"
	ModuleReasonError                     = "Error"

	ModuleMessageBundle                    = "turned off by bundle"
	ModuleMessageModuleConfig              = "turned off by module config"
	ModuleMessageDynamicGlobalHookExtender = "turned off by global hook"
	ModuleMessageEnabledScriptExtender     = "turned off by enabled script"
	ModuleMessageDeckhouseVersionExtender  = "turned off by deckhouse version"
	ModuleMessageKubernetesVersionExtender = "turned off by kubernetes version"
	ModuleMessageBootstrappedExtender      = "turned off because the cluster not bootstrapped yet"
	ModuleMessageModuleDependencyExtender  = "turned off because of unmet module dependencies"
	ModuleMessageNotInstalled              = "not installed"
	ModuleMessageDisabled                  = "disabled"
	ModuleMessageConflict                  = "several available sources"
	ModuleMessageDownloading               = "downloading"
	ModuleMessageReconciling               = "reconciling"
	ModuleMessageInstalling                = "installing"
	ModuleMessageOnStartupHook             = "onStartup hooks done"

	DeckhouseRequirementFieldName        string = "deckhouse"
	KubernetesRequirementFieldName       string = "kubernetes"
	ModuleDependencyRequirementFieldName string = "modules"

	ExperimentalModuleStage = "Experimental"
	DeprecatedModuleStage   = "Deprecated"
)

var (
	// ModuleGVR GroupVersionResource
	ModuleGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ModuleResource,
	}
	// ModuleGVK GroupVersionKind
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
// +kubebuilder:printcolumn:name="Weight",type="integer",JSONPath=".properties.weight",priority=1,description="Module weight"
// +kubebuilder:printcolumn:name="Stage",type="string",JSONPath=".properties.stage",description="Module stage"
// +kubebuilder:printcolumn:name="Release channel",type="string",JSONPath=".properties.releaseChannel",priority=1,description="Release channel of the module."
// +kubebuilder:printcolumn:name="Source",type="string",JSONPath=".properties.source",description="Source of the module it provided by one."
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".properties.version",priority=1,description="Module version."
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Module phase."
// +kubebuilder:printcolumn:name="Enabled",type="string",JSONPath=".status.conditions[?(@.type=='EnabledByModuleManager')].status",description="Module`s enabled status."
// +kubebuilder:printcolumn:name="Disabled Message",type="string",JSONPath=".status.conditions[?(@.type=='EnabledByModuleManager')].message",priority=1,description="Module`s enabled information."
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='IsReady')].status",description="Module`s ready status."
// +kubebuilder:metadata:labels="heritage=deckhouse"
// +kubebuilder:metadata:labels="app.kubernetes.io/name=deckhouse"
// +kubebuilder:metadata:labels="app.kubernetes.io/part-of=deckhouse"
// +crd-enricher:deckhouse:documentation:crd={preserveUnknownFields: false, minimal: true, stripFormat: [int32]}

// Describes the module's status in the cluster. The `Module` object is created automatically after configuring the [ModuleSource](#modulesource) and successfully completing synchronization.
type Module struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Properties ModuleProperties `json:"properties,omitempty"`

	Status ModuleStatus `json:"status,omitempty"`
}

type ModuleRequirements struct {
	ModulePlatformRequirements `json:",inline" yaml:",inline"`
	// A list of other enabled modules required for the module.
	ParentModules map[string]string `json:"modules,omitempty" yaml:"modules,omitempty"`
}

type ModulePlatformRequirements struct {
	// Required Deckhouse version.
	Deckhouse string `json:"deckhouse,omitempty" yaml:"deckhouse,omitempty"`
	// Required Kubernetes version.
	Kubernetes string `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	// Required cluster installation status (for built-in DKP modules only).
	Bootstrapped string `json:"bootstrapped,omitempty" yaml:"bootstrapped,omitempty"`
}

type ModuleProperties struct {
	// Module _weight_ (priority).
	Weight uint32 `json:"weight,omitempty"`
	// Source the module was downloaded from (otherwise will be blank).
	Source string `json:"source,omitempty"`
	// Module release channel.
	ReleaseChannel string `json:"releaseChannel,omitempty"`
	// Current stage of the module lifecycle.
	Stage string `json:"stage,omitempty"`
	// Indicates whether the module critical or not.
	Critical bool `json:"critical,omitempty"`
	// Module namespace.
	Namespace string `json:"namespace,omitempty"`
	// Module subsystems.
	Subsystems []string `json:"subsystems,omitempty"`
	// Module version.
	Version string `json:"version,omitempty"`
	// Module update policy.
	UpdatePolicy string `json:"updatePolicy,omitempty"`
	// Indicates the group where only one module can be active at a time.
	ExclusiveGroup string `json:"exclusiveGroup,omitempty" yaml:"exclusiveGroup,omitempty"`
	// Available sources for downloading the module.
	AvailableSources []string `json:"availableSources,omitempty"`
	// Module dependencies, a set of requirements that must be met for Deckhouse Kubernetes Platform (DKP) to run the module.
	// +kubebuilder:pruning:PreserveUnknownFields
	Requirements *ModuleRequirements `json:"requirements,omitempty" yaml:"requirements,omitempty"`
	// Parameters of module disable protection.
	DisableOptions *ModuleDisableOptions `json:"disableOptions,omitempty" yaml:"disableOptions,omitempty"`
	// Module accessibility settings.
	Accessibility *ModuleAccessibility `json:"accessibility,omitempty" yaml:"accessibility,omitempty"`
}

type ModuleAccessibility struct {
	// Module operation settings in Deckhouse editions.
	Editions map[string]ModuleEdition `json:"editions,omitempty" yaml:"editions"`
}

type ModuleEdition struct {
	Available        bool     `json:"available,omitempty" yaml:"available"`
	EnabledInBundles []string `json:"enabledInBundles,omitempty" yaml:"enabledInBundles"`
}

type ModuleDisableOptions struct {
	Confirmation bool   `json:"confirmation,omitempty" yaml:"confirmation"`
	Message      string `json:"message,omitempty" yaml:"message"`
}

type ModuleStatus struct {
	// Module phase.
	// +kubebuilder:validation:Enum=Unavailable;Available;Downloading;DownloadingError;Reconciling;Installing;HooksDisabled;WaitSyncTasks;Downloaded;Conflict;Ready;Error
	Phase string `json:"phase,omitempty"`
	// Hooks status report.
	HooksState string `json:"hooksState,omitempty"`
	// +crd-enricher:raw:x-kubernetes-patch-strategy=merge
	// +crd-enricher:raw:x-kubernetes-patch-merge-key=type
	Conditions []ModuleCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type ModuleCondition struct {
	// Type is the type of the condition.
	Type string `json:"type,omitempty"`
	// Machine-readable, UpperCamelCase text indicating the reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	Status corev1.ConditionStatus `json:"status,omitempty"`
	// Timestamp of when the condition was last probed.
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

func (m *Module) IsEmbedded() bool {
	return m.Properties.Source == ModuleSourceEmbedded
}

// IsEnabledByBundle checks if the module enabled in the specific edition and bundle
func (m *Module) IsEnabledByBundle(editionName, bundleName string) bool {
	if m.Properties.Accessibility == nil {
		return false
	}

	access := m.Properties.Accessibility

	if len(access.Editions) == 0 {
		return false
	}

	// check edition‑specific bundles first
	if edition, ok := access.Editions[editionName]; ok && isEnabledInBundle(edition.EnabledInBundles, bundleName) {
		return true
	}

	// check the default settings
	defaultSettings, ok := access.Editions["_default"]
	if !ok {
		return false
	}

	// fallback to the default
	return isEnabledInBundle(defaultSettings.EnabledInBundles, bundleName)
}

func isEnabledInBundle(bundles []string, requested string) bool {
	for _, bundle := range bundles {
		if bundle == requested {
			return true
		}
	}

	return false
}

func (m *Module) IsCondition(condName string, status corev1.ConditionStatus) bool {
	for _, cond := range m.Status.Conditions {
		if cond.Type == condName {
			return cond.Status == status
		}
	}

	return false
}

// +kubebuilder:object:generate=false
type ConditionOption func(opts *ConditionSettings)

func WithTimer(fn func() time.Time) func(opts *ConditionSettings) {
	return func(opts *ConditionSettings) {
		opts.Timer = fn
	}
}

// +kubebuilder:object:generate=false
type ConditionSettings struct {
	Timer func() time.Time
}

func (m *Module) SetConditionTrue(condName string, opts ...ConditionOption) {
	settings := &ConditionSettings{
		Timer: time.Now,
	}

	for _, opt := range opts {
		opt(settings)
	}

	for idx, cond := range m.Status.Conditions {
		if cond.Type == condName {
			m.Status.Conditions[idx].LastProbeTime = metav1.Time{Time: settings.Timer()}
			if cond.Status != corev1.ConditionTrue {
				m.Status.Conditions[idx].LastTransitionTime = metav1.Time{Time: settings.Timer()}
				m.Status.Conditions[idx].Status = corev1.ConditionTrue
			}
			m.Status.Conditions[idx].Reason = ""
			m.Status.Conditions[idx].Message = ""

			return
		}
	}

	m.Status.Conditions = append(m.Status.Conditions, ModuleCondition{
		Type:               condName,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      metav1.Time{Time: settings.Timer()},
		LastTransitionTime: metav1.Time{Time: settings.Timer()},
	})
}

func (m *Module) SetConditionFalse(condName, reason, message string, opts ...ConditionOption) {
	settings := &ConditionSettings{
		Timer: time.Now,
	}

	for _, opt := range opts {
		opt(settings)
	}

	for idx, cond := range m.Status.Conditions {
		if cond.Type == condName {
			m.Status.Conditions[idx].LastProbeTime = metav1.Time{Time: settings.Timer()}
			if cond.Status != corev1.ConditionFalse {
				m.Status.Conditions[idx].LastTransitionTime = metav1.Time{Time: settings.Timer()}
				m.Status.Conditions[idx].Status = corev1.ConditionFalse
			}
			if cond.Reason != reason {
				m.Status.Conditions[idx].Reason = reason
			}
			if cond.Message != message {
				m.Status.Conditions[idx].Message = message
			}
			return
		}
	}

	m.Status.Conditions = append(m.Status.Conditions, ModuleCondition{
		Type:               condName,
		Status:             corev1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		LastProbeTime:      metav1.Time{Time: settings.Timer()},
		LastTransitionTime: metav1.Time{Time: settings.Timer()},
	})
}

func (m *Module) SetConditionUnknown(condName, reason, message string, opts ...ConditionOption) {
	settings := &ConditionSettings{
		Timer: time.Now,
	}

	for _, opt := range opts {
		opt(settings)
	}

	for idx, cond := range m.Status.Conditions {
		if cond.Type == condName {
			m.Status.Conditions[idx].LastProbeTime = metav1.Time{Time: settings.Timer()}
			if cond.Status != corev1.ConditionUnknown {
				m.Status.Conditions[idx].LastTransitionTime = metav1.Time{Time: settings.Timer()}
				m.Status.Conditions[idx].Status = corev1.ConditionUnknown
			}
			if cond.Reason != reason {
				m.Status.Conditions[idx].Reason = reason
			}
			if cond.Message != message {
				m.Status.Conditions[idx].Message = message
			}
			return
		}
	}

	m.Status.Conditions = append(m.Status.Conditions, ModuleCondition{
		Type:               condName,
		Status:             corev1.ConditionUnknown,
		Reason:             reason,
		Message:            message,
		LastProbeTime:      metav1.Time{Time: settings.Timer()},
		LastTransitionTime: metav1.Time{Time: settings.Timer()},
	})
}

func (m *Module) DisabledByModuleConfigMoreThan(timeout time.Duration) bool {
	for _, cond := range m.Status.Conditions {
		if cond.Type == ModuleConditionEnabledByModuleConfig && cond.Status == corev1.ConditionFalse {
			return time.Since(cond.LastTransitionTime.Time) >= timeout
		}
	}

	return false
}

func (m *Module) HasCondition(condName string) bool {
	for _, cond := range m.Status.Conditions {
		if cond.Type == condName {
			return true
		}
	}
	return false
}

func (m *Module) GetVersion() string {
	return m.Properties.Version
}

func (m *Module) IsExperimental() bool {
	return m.Properties.Stage == ExperimentalModuleStage
}

func (m *Module) IsDeprecated() bool {
	return m.Properties.Stage == DeprecatedModuleStage
}

// +kubebuilder:object:root=true

// ModuleList is a list of Module resources
type ModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Module `json:"items"`
}
