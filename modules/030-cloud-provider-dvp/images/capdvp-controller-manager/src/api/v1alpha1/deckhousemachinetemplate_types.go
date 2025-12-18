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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DeckhouseMachineTemplateSpec defines the desired state of DeckhouseMachineTemplate.
type DeckhouseMachineTemplateSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Template DeckhouseMachineTemplateSpecTemplate `json:"template"`
}

type DeckhouseMachineTemplateSpecTemplate struct {
	// +optional
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty"`

	Spec DeckhouseMachineSpecTemplate `json:"spec"`
}

type DeckhouseMachineSpecTemplate struct {
	// VMClassName defines the name of the VirtualMachineClass resource describing requirements
	// for a virtual CPU, memory, and the resource allocation policy of this machine.
	VMClassName string `json:"vmClassName"`

	// CPU holds number of cores and processing time allocated to them.
	CPU CPU `json:"cpu"`

	// Memory is this machine's RAM amount in mebibytes (MiB).
	// +kubebuilder:default="8Gi"
	Memory resource.Quantity `json:"memory"`

	// AdditionalDisks holds the list of additional (non-boot) disks to attach to the VM.
	AdditionalDisks []AdditionalDisks `json:"additionalDisks,omitempty"`

	// RootDiskSize holds the size of the bootable disk.
	RootDiskSize resource.Quantity `json:"rootDiskSize"`

	// RootDiskStorageClass holds the name of the StorageClass to use for bootable disk.
	RootDiskStorageClass string `json:"rootDiskStorageClass"`

	// BootDiskImageRef holds the image to boot this VM from.
	BootDiskImageRef DiskImageRef `json:"bootDiskImageRef"`

	// Bootloader holds the type of underlying firmware this VM runs on. Must be kept in sync with DVP bootloader enum.
	// +kubebuilder:default=EFI
	// +kubebuilder:validation:Enum:={"BIOS", "EFI", "EFIWithSecureBoot"}
	Bootloader string `json:"bootloader,omitempty"`
}

// DeckhouseMachineTemplateStatus defines the observed state of DeckhouseMachineTemplate.
type DeckhouseMachineTemplateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DeckhouseMachineTemplate is the Schema for the deckhousemachinetemplates API.
type DeckhouseMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec DeckhouseMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// DeckhouseMachineTemplateList contains a list of DeckhouseMachineTemplate.
type DeckhouseMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeckhouseMachineTemplate `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &DeckhouseMachineTemplate{}, &DeckhouseMachineTemplateList{})
}
