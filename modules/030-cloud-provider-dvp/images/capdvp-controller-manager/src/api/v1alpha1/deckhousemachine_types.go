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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1b2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const MachineFinalizer = "deckhousemachine.infrastructure.cluster.x-k8s.io"

const (
	VMReadyCondition clusterv1b2.ConditionType = "VirtualMachineReady"
)

const (
	VMNotReadyReason       = "VMNotReady"
	VMErrorReason          = "VMError"
	VMInFailedStateReason  = "VMInFailedState"
	VMInStoppedStateReason = "VMInStoppedState"

	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"
	WaitingForBootstrapScriptReason       = "WaitingForBootstrapScript"
)

// CPU defines the VM CPU, made of variable number of cores, each getting the Fraction amount of processing time on a physical core.
type CPU struct {
	// Cores is the number of cores per socket.
	// +kubebuilder:default=4
	// +kubebuilder:validation:Minimum=1
	Cores int `json:"cores"`

	// Fraction is a guaranteed share of CPU time that will be allocated to the VM.
	// Expressed as percentage.
	// +kubebuilder:default="100%"
	// +kubebuilder:validation:Pattern=`^100%$|^[1-9][0-9]?%$`
	Fraction string `json:"cpuFraction"`
}

type DiskImageRef struct {
	// Name of the attached resource.
	Name string `json:"name"`
	// Kind is a block device type.
	// +kubebuilder:default="VirtualImage"
	// +kubebuilder:validation:Enum:={"ClusterVirtualImage", "VirtualImage", "VirtualDiskSnapshot"}
	Kind string `json:"kind"`
}

type VMAddress struct {
	// Type of address.
	Type clusterv1b2.MachineAddressType `json:"type"`
	// Address in IPv4 or IPv6 notation.
	Address string `json:"address"`
}

type AdditionalDisks struct {
	// Size of the disk.
	Size         resource.Quantity `json:"size"`
	StorageClass string            `json:"storageClass"`
}

// DeckhouseMachineSpec defines the desired state of DeckhouseMachine.
type DeckhouseMachineSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ProviderID is the UUID of the VM, prefixed with 'dvp://' proto.
	// +optional
	ProviderID string `json:"providerID,omitempty"`

	// VMClassName defines the name of the VirtualMachineClass resource describing requirements
	// for a virtual CPU, memory, and the resource allocation policy of this machine.
	VMClassName string `json:"vmClassName"`

	// CPU holds number of cores and processing time allocated to them.
	CPU CPU `json:"cpu"`

	// Memory is this machine's RAM amount.
	// +kubebuilder:default="8Gi"
	Memory resource.Quantity `json:"memory"`

	// RootDiskSize holds the size of the bootable disk.
	RootDiskSize resource.Quantity `json:"rootDiskSize"`

	// RootDiskStorageClass holds the name of the StorageClass to use for bootable disk.
	RootDiskStorageClass string `json:"rootDiskStorageClass"`

	// AdditionalDisks holds the list of additional disks to attach to the VM.
	AdditionalDisks []AdditionalDisks `json:"additionalDisks,omitempty"`

	// BootDiskImageRef holds the image to boot this VM from.
	BootDiskImageRef DiskImageRef `json:"bootDiskImageRef"`

	// Bootloader holds the type of underlying firmware this VM runs on. Must be kept in sync with DVP bootloader enum.
	// +kubebuilder:default=EFI
	// +kubebuilder:validation:Enum:={"BIOS", "EFI", "EFIWithSecureBoot"}
	Bootloader string `json:"bootloader,omitempty"`
}

// DeckhouseMachineStatus defines the observed state of DeckhouseMachine.
type DeckhouseMachineStatus struct {
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
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// initialization provides observations of the Machine initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Machine provisioning.
	// +optional
	Initialization MachineInitializationStatus `json:"initialization,omitempty,omitzero"`
}

// MachineInitializationStatus provides observations of the Machine initialization process.
// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Machine provisioning.
// +kubebuilder:validation:MinProperties=1
type MachineInitializationStatus struct {
	// infrastructureProvisioned is true when the infrastructure provider reports that Machine's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate provisioning.
	// The value of this field is never updated after provisioning is completed.
	// +optional
	InfrastructureProvisioned *bool `json:"infrastructureProvisioned,omitempty"`

	// bootstrapDataSecretCreated is true when the bootstrap provider reports that the Machine's boostrap secret is created.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate provisioning.
	// The value of this field is never updated after provisioning is completed.
	// +optional
	BootstrapDataSecretCreated *bool `json:"bootstrapDataSecretCreated,omitempty"`

	Provisioned *bool `json:"provisioned,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DeckhouseMachine is the Schema for the deckhousemachines API.
type DeckhouseMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeckhouseMachineSpec   `json:"spec,omitempty"`
	Status DeckhouseMachineStatus `json:"status,omitempty"`
}

// GetConditions gets the DeckhouseMachine status conditions
func (r *DeckhouseMachine) GetConditions() []metav1.Condition {
	return r.Status.Conditions
}

// SetConditions sets the DeckhouseMachine status conditions
func (r *DeckhouseMachine) SetConditions(conditions []metav1.Condition) {
	r.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// DeckhouseMachineList contains a list of DeckhouseMachine.
type DeckhouseMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeckhouseMachine `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &DeckhouseMachine{}, &DeckhouseMachineList{})
}
