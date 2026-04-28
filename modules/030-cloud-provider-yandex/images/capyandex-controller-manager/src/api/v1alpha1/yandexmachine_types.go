package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const MachineFinalizer = "yandexmachine.infrastructure.cluster.x-k8s.io"
const ProviderIDPrefix = "yandex://"

const (
	VMReadyCondition clusterv1.ConditionType = "VirtualMachineReady"
	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"
	WaitingForBootstrapScriptReason = "WaitingForBootstrapScript"
	VMCreatingReason = "VMCreating"
	VMDeletingReason = "VMDeleting"
	VMNotReadyReason = "VMNotReady"
	VMErrorReason = "VMError"
	VMInFailedStateReason = "VMInFailedState"
	VMInStoppedStateReason = "VMInStoppedState"
)

type YandexMachineResources struct {
	Cores int32 `json:"cores,omitempty"`
	CoreFraction int32 `json:"coreFraction,omitempty"`
	MemoryMiB int64 `json:"memoryMiB,omitempty"`
	GPUs int32 `json:"gpus,omitempty"`
}

type YandexBootDisk struct {
	Type string `json:"type,omitempty"`
	SizeGiB int32 `json:"sizeGiB,omitempty"`
	ImageID string `json:"imageID,omitempty"`
}

type YandexNetworkInterface struct {
	SubnetID string `json:"subnetID,omitempty"`
	AssignPublicIPAddress bool `json:"assignPublicIPAddress,omitempty"`
}

type YandexSchedulingPolicy struct {
	Preemptible bool `json:"preemptible,omitempty"`
}

type YandexMachineSpec struct {
	ProviderID string `json:"providerID,omitempty"`
	Region string `json:"region,omitempty"`
	Zone string `json:"zone,omitempty"`
	FolderID string `json:"folderID,omitempty"`
	PlatformID string `json:"platformID,omitempty"`
	Resources YandexMachineResources `json:"resources,omitempty"`
	BootDisk YandexBootDisk `json:"bootDisk,omitempty"`
	NetworkInterfaces []YandexNetworkInterface `json:"networkInterfaces,omitempty"`
	SchedulingPolicy *YandexSchedulingPolicy `json:"schedulingPolicy,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	NetworkType string `json:"networkType,omitempty"`
}

type MachineInitializationStatus struct {
	Provisioned *bool `json:"provisioned,omitempty"`
}

type YandexMachineStatus struct {
	Ready bool `json:"ready,omitempty"`
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`
	FailureReason *string `json:"failureReason,omitempty"`
	FailureMessage *string `json:"failureMessage,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	Initialization MachineInitializationStatus `json:"initialization,omitempty,omitzero"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type YandexMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   YandexMachineSpec   `json:"spec,omitempty"`
	Status YandexMachineStatus `json:"status,omitempty"`
}

func (r *YandexMachine) GetConditions() []metav1.Condition {
	return r.Status.Conditions
}

func (r *YandexMachine) SetConditions(conditions []metav1.Condition) {
	r.Status.Conditions = conditions
}

// +kubebuilder:object:root=true
type YandexMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []YandexMachine `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &YandexMachine{}, &YandexMachineList{})
}
