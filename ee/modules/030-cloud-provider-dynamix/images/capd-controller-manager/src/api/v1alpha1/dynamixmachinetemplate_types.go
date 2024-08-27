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

// DynamixMachineTemplateSpec defines the desired state of DynamixMachineTemplate
type DynamixMachineTemplateSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Template DynamixMachineTemplateSpecTemplate `json:"template"`
}

type DynamixMachineTemplateSpecTemplate struct {
	// +optional
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty"`

	Spec DynamixMachineSpecTemplate `json:"spec"`
}

type DynamixMachineSpecTemplate struct {
	// The VM image this instance will be created from.
	// +kubebuilder:validation:Required
	ImageName string `json:"template"`
	// CPU defines the VM CPU.
	// +optional
	CPU int32 `json:"cpu,omitempty"`
	// MemoryMB is the size of a VM's memory in MiBs.
	// +kubebuilder:default=8192
	// +optional
	Memory int32 `json:"memory,omitempty"`
	// RootDiskSize size of the bootable disk in GiB.
	// +kubebuilder:default=30
	RootDiskSize int64 `json:"rootDiskSizeGb"`
}

// DynamixMachineTemplateStatus defines the observed state of DynamixMachineTemplate
type DynamixMachineTemplateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DynamixMachineTemplate is the Schema for the dynamixmachinetemplates API
type DynamixMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DynamixMachineTemplateSpec   `json:"spec,omitempty"`
	Status DynamixMachineTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DynamixMachineTemplateList contains a list of DynamixMachineTemplate
type DynamixMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DynamixMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DynamixMachineTemplate{}, &DynamixMachineTemplateList{})
}
