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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The desired state of `VirtualMachineIPAddressClaim`.
type VirtualMachineIPAddressClaimSpec struct {
	// Represents that claim should not be removed with virtual machine after the first use.
	//+kubebuilder:default:=true
	//+kubebuilder:validation:Required
	Static *bool `json:"static,omitempty"`
	// The issued `VirtualMachineIPAddressLease`, managed automatically.
	LeaseName string `json:"leaseName,omitempty"`
	// The requested IP address. If omittedthe next available IP address will be assigned.
	Address string `json:"address,omitempty"`
}

// The observed state of `VirtualMachineIPAddressClaim`.
type VirtualMachineIPAddressClaimStatus struct {
	// Represents the current state of IP address claim.
	Phase string `json:"phase,omitempty"`
	// Represents the virtual machine that currently uses this IP address.
	VMName string `json:"vmName,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:JSONPath=".spec.address",name=Address,type=string
//+kubebuilder:printcolumn:JSONPath=".spec.static",name=Static,type=string
//+kubebuilder:printcolumn:JSONPath=".status.phase",name=Status,type=string
//+kubebuilder:printcolumn:JSONPath=".status.vmName",name=VM,type=string
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:resource:shortName={"vmip","vmips"}

// The resource that defines IP address claim for virtual machine.
type VirtualMachineIPAddressClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineIPAddressClaimSpec   `json:"spec,omitempty"`
	Status VirtualMachineIPAddressClaimStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// Contains a list of `VirtualMachineIPAddressClaim`.
type VirtualMachineIPAddressClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachineIPAddressClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualMachineIPAddressClaim{}, &VirtualMachineIPAddressClaimList{})
}
