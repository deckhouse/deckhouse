/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const MachineFinalizer = "dynamixmachine.infrastructure.cluster.x-k8s.io"

const (
	VMReadyCondition clusterv1.ConditionType = "VirtualMachineReady"
)

const (
	VMNotReadyReason      = "VMNotReady"
	VMErrorReason         = "VMError"
	VMInFailedStateReason = "VMInFailedState"

	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"
	WaitingForBootstrapScriptReason       = "WaitingForBootstrapScript"
)

// DynamixMachineSpec defines the desired state of DynamixMachine
type DynamixMachineSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ID is the UUID of the VM
	// +optional
	ID string `json:"id,omitempty"`
	// ProviderID is the UUID of the VM, prefixed with 'dynamix://' proto.
	// +optional
	ProviderID string `json:"providerID,omitempty"`
	// The VM image this instance will be created from.
	ImageName string `json:"imageName"`
	// CPU defines the VM CPU.
	// +kubebuilder:default=1
	CPU int32 `json:"cpu,omitempty"`
	// MemoryMB is the size of a VM's memory in MiBs.
	// +kubebuilder:default=8192
	Memory int32 `json:"memory,omitempty"`
	// RootDiskSize size of the bootable disk in GiB.
	// +kubebuilder:default=30
	RootDiskSize int64 `json:"rootDiskSizeGb"`
}

// DynamixMachineStatus defines the observed state of DynamixMachine
type DynamixMachineStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Ready indicates the VM has been provisioned and is ready.
	// +optional
	Ready bool `json:"ready"`

	// Addresses holds a list of the host names, external IP addresses, internal IP addresses, external DNS names, and/or internal DNS names for the VM.
	// +optional
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// FailureReason will contain an error type if something goes wrong during Machine lifecycle.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage will describe an error if something goes wrong during Machine lifecycle.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the DynamixMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DynamixMachine is the Schema for the dynamixmachines API
type DynamixMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DynamixMachineSpec   `json:"spec,omitempty"`
	Status DynamixMachineStatus `json:"status,omitempty"`
}

// GetConditions gets the DynamixInstance status conditions
func (r *DynamixMachine) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the DynamixInstance status conditions
func (r *DynamixMachine) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// DynamixMachineList contains a list of DynamixMachine
type DynamixMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DynamixMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DynamixMachine{}, &DynamixMachineList{})
}
