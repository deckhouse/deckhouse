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

// ZvirtMachineTemplateSpec defines the desired state of ZvirtMachineTemplate
type ZvirtMachineTemplateSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Template ZvirtMachineTemplateSpecTemplate `json:"template"`
}

type ZvirtMachineTemplateSpecTemplate struct {
	// +optional
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty"`

	Spec ZvirtMachineSpecTemplate `json:"spec"`
}

type ZvirtMachineSpecTemplate struct {
	// The VM template this instance will be created from.
	// +kubebuilder:validation:Required
	TemplateName string `json:"template"`
	// VNICProfileID the id of the zVirt vNic profile for the VM.
	// +kubebuilder:validation:Required
	VNICProfileID string `json:"vnicProfileID"`
	// CPU defines the VM CPU.
	// +optional
	CPU CPU `json:"cpu,omitempty"`
	// MemoryMB is the size of a VM's memory in MiBs.
	// +kubebuilder:default=8192
	// +optional
	Memory int32 `json:"memory,omitempty"`
	// RootDiskSize size of the bootable disk in GiB.
	// +kubebuilder:default=20
	RootDiskSize int64 `json:"rootDiskSizeGb"`
	// NicName is a name that will be assigned to the vNIC attached to the VM.
	// +kubebuilder:default=nic1
	// +optional
	NicName string `json:"nicName"`
}

// ZvirtMachineTemplateStatus defines the observed state of ZvirtMachineTemplate
type ZvirtMachineTemplateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// ZvirtMachineTemplate is the Schema for the zvirtmachinetemplates API
type ZvirtMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ZvirtMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ZvirtMachineTemplateList contains a list of ZvirtMachineTemplate
type ZvirtMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZvirtMachineTemplate `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &ZvirtMachineTemplate{}, &ZvirtMachineTemplateList{})
}
