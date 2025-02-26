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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1b1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// CPU defines the VM CPU, made of variable number of cores, each getting the Fraction amount of processing time on a physical core.
type CPU struct {
	// Cores is the number of cores per socket.
	// +kubebuilder:default=4
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1024
	Cores int `json:"cores"`

	// Fraction is a guaranteed share of CPU time that will be allocated to the VM.
	// Expressed as percentage (0-100]
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=100
	Fraction int `json:"cpuFraction"`
}

type VMAddress struct {
	Type    clusterv1b1.MachineAddressType `json:"type"`
	Address string                         `json:"address"`
}

// DeckhouseMachineSpec defines the desired state of DeckhouseMachine.
type DeckhouseMachineSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Name is the hostname and unique identifier of this VM within DVP.
	Name string `json:"name"`

	// ProviderID is the UUID of the VM, prefixed with 'dvp://' proto.
	// +optional
	ProviderID string `json:"providerID,omitempty"`

	// VMClassName defines the name of the VirtualMachineClass resource describing requirements
	// for a virtual CPU, memory, and the resource allocation policy of this machine.
	VMClassName string `json:"vmClassName"`

	// CPU holds number of cores and processing time allocated to them.
	CPU CPU `json:"cpu"`

	// Memory is this machine's RAM amount in megabytes.
	// +kubebuilder:validation:Minimum=8
	// +kubebuilder:default=8192
	Memory int `json:"memory"`

	// RootDiskSize size of the bootable disk in GiB.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=30
	RootDiskSize int64 `json:"rootDiskSize"`
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
	Conditions clusterv1b1.Conditions `json:"conditions,omitempty"`
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

// +kubebuilder:object:root=true

// DeckhouseMachineList contains a list of DeckhouseMachine.
type DeckhouseMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeckhouseMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeckhouseMachine{}, &DeckhouseMachineList{})
}
