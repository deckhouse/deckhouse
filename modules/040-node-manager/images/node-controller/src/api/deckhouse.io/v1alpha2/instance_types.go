/*
Copyright 2026 Flant JSC

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
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status

// Instance represents machine/node/bashible lifecycle in one place.
type Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Desired references for this Instance.
	Spec InstanceSpec `json:"spec,omitempty"`

	// Most recently observed status of the instance.
	Status InstanceStatus `json:"status,omitempty"`
}

// InstanceSpec holds references to related resources.
type InstanceSpec struct {
	NodeRef        NodeRef         `json:"nodeRef,omitempty"`
	MachineRef     *MachineRef     `json:"machineRef,omitempty"`
	ClassReference *ClassReference `json:"classReference,omitempty"`
}

// InstanceStatus is the observed state of Instance.
type InstanceStatus struct {
	// High-level lifecycle phase.
	Phase InstancePhase `json:"phase,omitempty"`

	// Aggregated machine status for UX.
	MachineStatus string `json:"machineStatus,omitempty"`

	// Aggregated bashible status for UX.
	BashibleStatus BashibleStatus `json:"bashibleStatus,omitempty"`

	// Human-readable details for current state.
	Message string `json:"message,omitempty"`

	// Raw status observations.
	Conditions []InstanceCondition `json:"conditions,omitempty"`
}

// InstancePhase is a high-level lifecycle phase.
type InstancePhase string

const (
	InstancePhasePending      InstancePhase = "Pending"
	InstancePhaseProvisioning InstancePhase = "Provisioning"
	InstancePhaseProvisioned  InstancePhase = "Provisioned"
	InstancePhaseRunning      InstancePhase = "Running"
	InstancePhaseTerminating  InstancePhase = "Terminating"
	InstancePhaseUnknown      InstancePhase = "Unknown"
)

// BashibleStatus describes bashible state for UX.
type BashibleStatus string

const (
	BashibleStatusError           BashibleStatus = "Error"
	BashibleStatusUnknown         BashibleStatus = "Unknown"
	BashibleStatusReady           BashibleStatus = "Ready"
	BashibleStatusWaitingApproval BashibleStatus = "WaitingApproval"
)

const (
	InstanceConditionTypeMachineReady              = "MachineReady"
	InstanceConditionTypeBashibleReady             = "BashibleReady"
	InstanceConditionTypeWaitingApproval           = "WaitingApproval"
	InstanceConditionTypeWaitingDisruptionApproval = "WaitingDisruptionApproval"
)

// InstanceCondition describes one raw condition entry.
type InstanceCondition struct {
	Type               string                 `json:"type"`
	Status             metav1.ConditionStatus `json:"status"`
	Reason             string                 `json:"reason,omitempty"`
	Severity           string                 `json:"severity,omitempty"`
	Message            string                 `json:"message,omitempty"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty"`
	ObservedGeneration int64                  `json:"observedGeneration,omitempty"`
}

// MachineRef is reference to a machine resource.
type MachineRef struct {
	Kind       string `json:"kind,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
	Name       string `json:"name,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
}

// NodeRef is reference to node object.
type NodeRef struct {
	Name string `json:"name,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceList contains a list of Instance.
type InstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Instance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Instance{}, &InstanceList{})
}
