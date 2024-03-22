/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const MachineFinalizer = "zvirtmachine.infrastructure.cluster.x-k8s.io"

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

// ZvirtMachineSpec defines the desired state of ZvirtMachine
type ZvirtMachineSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ID is the UUID of the VM
	// +optional
	ID string `json:"id,omitempty"`
	// ProviderID is the UUID of the VM, prefixed with 'zvirt://' proto.
	// +optional
	ProviderID string `json:"providerID,omitempty"`
	// The VM template this instance will be created from.
	TemplateName string `json:"template"`
	// VNICProfileID the id of the zVirt vNic profile for the VM.
	VNICProfileID string `json:"vnicProfileID"`
	// CPU defines the VM CPU.
	CPU CPU `json:"cpu"`
	// MemoryMB is the size of a VM's memory in MiBs.
	// +kubebuilder:default=8192
	Memory int32 `json:"memory,omitempty"`
	// RootDiskSize size of the bootable disk in GiB.
	// +kubebuilder:default=20
	RootDiskSize int64 `json:"rootDiskSizeGb"`
	// NicName is a name that will be assigned to the vNIC attached to the VM.
	// +kubebuilder:default=nic1
	NicName string `json:"nicName"`
}

// CPU defines the VM cpu, made of (Sockets * Cores * Threads).
// Most of the time you should only set Sockets to the number of cores you want VM to have and set Cores and Threads to 1.
type CPU struct {
	// Sockets is the number of sockets for a VM.
	// +kubebuilder:default=4
	Sockets int32 `json:"sockets"`

	// Cores is the number of cores per socket.
	// +kubebuilder:default=1
	Cores int32 `json:"cores"`

	// Threads is the number of thread per core.
	// +kubebuilder:default=1
	Threads int32 `json:"threads"`
}

// ZvirtMachineStatus defines the observed state of ZvirtMachine
type ZvirtMachineStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Ready indicates the VM has been provisioned and is ready.
	// +optional
	Ready bool `json:"ready"`

	// Addresses holds a list of the host names, external IP addresses, internal IP addresses, external DNS names, and/or internal DNS names for the VM.
	// +optional
	Addresses []VMAddress `json:"addresses,omitempty"`

	// FailureReason will contain an error type if something goes wrong during Machine lifecycle.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage will describe an error if something goes wrong during Machine lifecycle.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the ZvirtMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

type VMAddress struct {
	Type    clusterv1.MachineAddressType `json:"type"`
	Address string                       `json:"address"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ZvirtMachine is the Schema for the zvirtmachines API
type ZvirtMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZvirtMachineSpec   `json:"spec,omitempty"`
	Status ZvirtMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZvirtMachineList contains a list of ZvirtMachine
type ZvirtMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZvirtMachine `json:"items"`
}

// GetConditions gets the ZvirtInstance status conditions
func (r *ZvirtMachine) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the ZvirtInstance status conditions
func (r *ZvirtMachine) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

func init() {
	objectTypes = append(objectTypes, &ZvirtMachine{}, &ZvirtMachineList{})
}
