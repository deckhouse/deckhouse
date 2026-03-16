package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type Machine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineSpec   `json:"spec,omitempty"`
	Status MachineStatus `json:"status,omitempty"`
}

type MachineSpec struct {
	Class                MachineClassSpec      `json:"class,omitempty"`
	ProviderID           string                `json:"providerID,omitempty"`
	NodeTemplateSpec     MachineNodeTemplate   `json:"nodeTemplate,omitempty"`
	MachineConfiguration *MachineConfiguration `json:",inline,omitempty"`
}

type MachineClassSpec struct {
	APIGroup string `json:"apiGroup,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Name     string `json:"name,omitempty"`
}

type MachineNodeTemplate struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              corev1.NodeSpec `json:"spec,omitempty"`
}

type MachineStatus struct {
	Node          string                 `json:"node,omitempty"`
	Conditions    []corev1.NodeCondition `json:"conditions,omitempty"`
	LastOperation MachineLastOperation   `json:"lastOperation,omitempty"`
	CurrentStatus MachineCurrentStatus   `json:"currentStatus,omitempty"`
}

type MachineLastOperation struct {
	Description    string               `json:"description,omitempty"`
	LastUpdateTime metav1.Time          `json:"lastUpdateTime,omitempty"`
	State          MachineState         `json:"state,omitempty"`
	Type           MachineOperationType `json:"type,omitempty"`
}

type MachinePhase string

const (
	MachinePhasePending          MachinePhase = "Pending"
	MachinePhaseAvailable        MachinePhase = "Available"
	MachinePhaseRunning          MachinePhase = "Running"
	MachinePhaseTerminating      MachinePhase = "Terminating"
	MachinePhaseUnknown          MachinePhase = "Unknown"
	MachinePhaseFailed           MachinePhase = "Failed"
	MachinePhaseCrashLoopBackOff MachinePhase = "CrashLoopBackOff"
)

type MachineState string

const (
	MachineStateProcessing MachineState = "Processing"
	MachineStateFailed     MachineState = "Failed"
	MachineStateSuccessful MachineState = "Successful"
)

type MachineOperationType string

const (
	MachineOperationCreate      MachineOperationType = "Create"
	MachineOperationUpdate      MachineOperationType = "Update"
	MachineOperationHealthCheck MachineOperationType = "HealthCheck"
	MachineOperationDelete      MachineOperationType = "Delete"
)

type MachineCurrentStatus struct {
	Phase          MachinePhase `json:"phase,omitempty"`
	TimeoutActive  bool         `json:"timeoutActive,omitempty"`
	LastUpdateTime metav1.Time  `json:"lastUpdateTime,omitempty"`
}

type MachineConfiguration struct {
	MachineDrainTimeout    *metav1.Duration `json:"drainTimeout,omitempty"`
	MachineHealthTimeout   *metav1.Duration `json:"healthTimeout,omitempty"`
	MachineCreationTimeout *metav1.Duration `json:"creationTimeout,omitempty"`
	MaxEvictRetries        *int32           `json:"maxEvictRetries,omitempty"`
	NodeConditions         *string          `json:"nodeConditions,omitempty"`
}

// +kubebuilder:object:root=true
type MachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Machine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Machine{}, &MachineList{})
}
