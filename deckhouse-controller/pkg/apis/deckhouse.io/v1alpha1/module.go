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

	ModuleConditionEnabledByModuleConfig  = "EnabledByModuleConfig"
	ModuleConditionEnabledByModuleManager = "EnabledByModuleManager"
	ModuleConditionIsReady                = "IsReady"

	ModulePhaseAvailable     = "Available"
	ModulePhaseDownloading   = "Downloading"
	ModulePhaseReconciling   = "Reconciling"
	ModulePhaseInstalling    = "Installing"
	ModulePhaseHooksDisabled = "HooksDisabled"
	ModulePhaseWaitSyncTasks = "WaitSyncTasks"
	ModulePhaseDownloaded    = "Downloaded"
	ModulePhaseConflict      = "Conflict"
	ModulePhaseReady         = "Ready"
	ModulePhaseError         = "Error"

	ModuleReasonBundle                      = "Bundle"
	ModuleReasonModuleConfig                = "ModuleConfig"
	ModuleReasonDynamicGlobalHookExtender   = "DynamicGlobalHookExtender"
	ModuleReasonEnabledScriptExtender       = "EnabledScriptExtender"
	ModuleReasonDeckhouseVersionExtender    = "DeckhouseVersionExtender"
	ModuleReasonKubernetesVersionExtender   = "KubernetesVersionExtender"
	ModuleReasonClusterBootstrappedExtender = "ClusterBootstrappedExtender"
	ModuleReasonNotInstalled                = "NotInstalled"
	ModuleReasonDisabled                    = "Disabled"
	ModuleReasonInit                        = "Init"
	ModuleReasonConflict                    = "Conflict"
	ModuleReasonDownloading                 = "Downloading"
	ModuleReasonHookError                   = "HookError"
	ModuleReasonModuleError                 = "ModuleError"
	ModuleReasonReconciling                 = "Reconciling"
	ModuleReasonInstalling                  = "Installing"
	ModuleReasonHooksDisabled               = "HooksDisabled"
	ModuleReasonWaitSyncTasks               = "WaitSyncTasks"
	ModuleReasonError                       = "Error"

	ModuleMessageBundle                      = "turned off by bundle"
	ModuleMessageModuleConfig                = "turned off by module config"
	ModuleMessageDynamicGlobalHookExtender   = "turned off by global hook"
	ModuleMessageEnabledScriptExtender       = "turned off by enabled script"
	ModuleMessageDeckhouseVersionExtender    = "turned off by deckhouse version"
	ModuleMessageKubernetesVersionExtender   = "turned off by kubernetes version"
	ModuleMessageClusterBootstrappedExtender = "turned off because the cluster not bootstrapped yet"
	ModuleMessageNotInstalled                = "not installed"
	ModuleMessageDisabled                    = "disabled"
	ModuleMessageInit                        = "init"
	ModuleMessageConflict                    = "several available sources"
	ModuleMessageDownloading                 = "downloading"
	ModuleMessageReconciling                 = "reconciling"
	ModuleMessageInstalling                  = "installing"
	ModuleMessageWaitSyncTasks               = "run sync tasks"
	ModuleMessageOnStartupHook               = "completed OnStartup hooks"
	ModuleMessageHooksDisabled               = "hooks disabled"
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

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleList is a list of Module resources
type ModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Module `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Module is a deckhouse module representation.
type Module struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Properties ModuleProperties `json:"properties,omitempty"`

	Status ModuleStatus `json:"status,omitempty"`
}

type ModuleProperties struct {
	Weight           uint32            `json:"weight,omitempty"`
	Source           string            `json:"source,omitempty"`
	ReleaseChannel   string            `json:"releaseChannel,omitempty"`
	Stage            string            `json:"stage,omitempty"`
	Description      string            `json:"description,omitempty"`
	Version          string            `json:"version,omitempty"`
	UpdatePolicy     string            `json:"updatePolicy,omitempty"`
	AvailableSources []string          `json:"availableSources,omitempty"`
	Requirements     map[string]string `json:"requirements,omitempty"`
}

type ModuleStatus struct {
	Phase      string            `json:"phase,omitempty"`
	HooksState string            `json:"hooksState,omitempty"`
	Conditions []ModuleCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type ModuleCondition struct {
	// Type is the type of the condition.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Type string `json:"type,omitempty"`
	// Machine-readable, UpperCamelCase text indicating the reason for the condition's last transition.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Message string `json:"message,omitempty"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Status corev1.ConditionStatus `json:"status,omitempty"`
	// Timestamp of when the condition was last probed.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

func (m *Module) IsEmbedded() bool {
	return m.Properties.Source == ModuleSourceEmbedded
}

func (m *Module) ConditionStatus(condName string) bool {
	for _, cond := range m.Status.Conditions {
		if cond.Type == condName {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func (m *Module) SetConditionTrue(condName string) {
	for idx, cond := range m.Status.Conditions {
		if cond.Type == condName {
			m.Status.Conditions[idx].LastProbeTime = metav1.Now()
			if cond.Status == corev1.ConditionFalse {
				m.Status.Conditions[idx].LastTransitionTime = metav1.Now()
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
		LastProbeTime:      metav1.Now(),
		LastTransitionTime: metav1.Now(),
	})
}

func (m *Module) SetConditionFalse(condName, reason, message string) {
	for idx, cond := range m.Status.Conditions {
		if cond.Type == condName {
			m.Status.Conditions[idx].LastProbeTime = metav1.Now()
			if cond.Status == corev1.ConditionTrue {
				m.Status.Conditions[idx].LastTransitionTime = metav1.Now()
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
		LastProbeTime:      metav1.Now(),
		LastTransitionTime: metav1.Now(),
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
