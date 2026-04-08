package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
type Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status InstanceStatus `json:"status,omitempty"`
}

type InstanceStatus struct {
	NodeRef         InstanceNodeRef         `json:"nodeRef,omitempty"`
	MachineRef      InstanceMachineRef      `json:"machineRef,omitempty"`
	CurrentStatus   InstanceCurrentStatus   `json:"currentStatus,omitempty"`
	LastOperation   InstanceLastOperation   `json:"lastOperation,omitempty"`
	BootstrapStatus InstanceBootstrapStatus `json:"bootstrapStatus,omitempty"`
	ClassReference  InstanceClassReference  `json:"classReference,omitempty"`
}

type InstanceClassReference struct {
	Kind string `json:"kind,omitempty"`
	Name string `json:"name,omitempty"`
}

type InstanceBootstrapStatus struct {
	LogsEndpoint string `json:"logsEndpoint,omitempty"`
	Description  string `json:"description,omitempty"`
}

type InstanceState string

const (
	InstanceStateProcessing InstanceState = "Processing"
	InstanceStateFailed     InstanceState = "Failed"
	InstanceStateSuccessful InstanceState = "Successful"
)

type InstanceOperationType string

const (
	InstanceOperationCreate      InstanceOperationType = "Create"
	InstanceOperationUpdate      InstanceOperationType = "Update"
	InstanceOperationHealthCheck InstanceOperationType = "HealthCheck"
	InstanceOperationDelete      InstanceOperationType = "Delete"
)

type InstanceLastOperation struct {
	Description    string                `json:"description,omitempty"`
	LastUpdateTime metav1.Time           `json:"lastUpdateTime,omitempty"`
	State          InstanceState         `json:"state,omitempty"`
	Type           InstanceOperationType `json:"type,omitempty"`
}

type InstancePhase string

const (
	InstancePhasePending          InstancePhase = "Pending"
	InstancePhaseAvailable        InstancePhase = "Available"
	InstancePhaseRunning          InstancePhase = "Running"
	InstancePhaseTerminating      InstancePhase = "Terminating"
	InstancePhaseUnknown          InstancePhase = "Unknown"
	InstancePhaseFailed           InstancePhase = "Failed"
	InstancePhaseCrashLoopBackOff InstancePhase = "CrashLoopBackOff"
)

type InstanceCurrentStatus struct {
	Phase          InstancePhase `json:"phase,omitempty"`
	LastUpdateTime metav1.Time   `json:"lastUpdateTime,omitempty"`
}

type InstanceMachineRef struct {
	Kind       string `json:"kind,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
	Name       string `json:"name,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
}

type InstanceNodeRef struct {
	Name string `json:"name,omitempty"`
}

// +kubebuilder:object:root=true
type InstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Instance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Instance{}, &InstanceList{})
}
