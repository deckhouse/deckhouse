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

// The desired state of `VirtualMachineIPAddressLease`.
type VirtualMachineIPAddressLeaseSpec struct {
	// The link to existing `VirtualMachineIPAddressClaim`.
	ClaimRef *VirtualMachineIPAddressLeaseClaimRef `json:"claimRef,omitempty"`
}

type VirtualMachineIPAddressLeaseClaimRef struct {
	// The Namespace of the referenced `VirtualMachineIPAddressClaim`.
	Namespace string `json:"namespace"`
	// The name of the referenced `VirtualMachineIPAddressClaim`.
	Name string `json:"name"`
}

// The observed state of `VirtualMachineIPAddressLease`.
type VirtualMachineIPAddressLeaseStatus struct {
	// Represents the current state of issued IP address lease.
	Phase string `json:"phase,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:JSONPath=".spec.claimRef",name=Claim,type=string
//+kubebuilder:printcolumn:JSONPath=".status.phase",name=Status,type=string
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:resource:scope=Cluster,shortName={"vmipl","vmipls"}

// The resource that defines fact of issued lease for `VirtualMachineIPAddressClaim`.
type VirtualMachineIPAddressLease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineIPAddressLeaseSpec   `json:"spec,omitempty"`
	Status VirtualMachineIPAddressLeaseStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// Contains a list of `VirtualMachineIPAddressLease`.
type VirtualMachineIPAddressLeaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachineIPAddressLease `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualMachineIPAddressLease{}, &VirtualMachineIPAddressLeaseList{})
}
