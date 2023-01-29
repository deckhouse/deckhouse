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
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	virtv1 "kubevirt.io/api/core/v1"
)

// The desired state of `VirtualMachine`.
type VirtualMachineSpec struct {
	// The requested running state of the `VirtualMachineInstance` mutually exclusive with Running.
	Running *bool `json:"running,omitempty"`
	// The name for associated `VirtualMahcineIPAddressClaim` resource.
	// If not specified, defaults to `{vm name}`.
	IPAddressClaimName *string `json:"ipAddressClaimName,omitempty"`
	// A set of (resource name, quantity) pairs.
	Resources v1.ResourceList `json:"resources,omitempty"`
	// The username that should be automatically created inside the VM.
	// This option requires `cloud-init` in the virtual machine.
	UserName *string `json:"userName,omitempty"`
	// The SSH public key that should be automatically added to user inside the VM.
	// This option requires `cloud-init` in the virtual machine.
	SSHPublicKey *string   `json:"sshPublicKey,omitempty"`
	BootDisk     *BootDisk `json:"bootDisk,omitempty"`
	// A cloud-init nocloud user data source. [More info...](https://cloudinit.readthedocs.io/en/latest/reference/datasources/nocloud.html)
	CloudInit *virtv1.CloudInitNoCloudSource `json:"cloudInit,omitempty"`
	// Represents a lits of additional disks that should be attached to the virtual machine.
	DiskAttachments *[]DiskSource `json:"diskAttachments,omitempty"`
	// A selector which must be true for the vm to fit on a node.
	// Selector which must match a node's labels for the vmi to be scheduled on that node.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// If toleration is specified, obey all the toleration rules.
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
}

// The observed state of `VirtualMachine`.
type VirtualMachineStatus struct {
	// Phase is a human readable, high-level representation of the status of the virtual machine.
	Phase virtv1.VirtualMachinePrintableStatus `json:"phase,omitempty"`
	// IP address of Virtual Machine.
	IPAddress string `json:"ipAddress,omitempty"`
}

// The boot disk for virtual machine.
type BootDisk struct {
	// Name for virtual machine boot disk.
	// If not specified defaults to `{vm name}-boot`.
	Name string `json:"name,omitempty"`
	// The source of a boot disk.
	// If specified, the new disk will be created.
	Source *corev1.TypedLocalObjectReference `json:"source,omitempty"`
	// The storage class for newly created disk.
	StorageClassName string `json:"storageClassName,omitempty"`
	// The size for newly created disk.
	Size resource.Quantity `json:"size"`
	// Enables automatic removal of associated boot disk after removing the virtual machine.
	AutoDelete bool `json:"autoDelete,omitempty"`
	// The type of disk device to emulate.
	//
	// Supported values: `virtio`, `sata`, `scsi`, `usb`.
	Bus string `json:"bus,omitempty"`
}

// The source of existing disk.
type DiskSource struct {
	// The name of the Disk in the same Namespace.
	Name string `json:"name"`
	// Indicates whether the volume can be hotplugged and hotunplugged.
	// +optional
	Hotpluggable bool `json:"hotpluggable,omitempty"`
	// The type of disk device to emulate.
	//
	// Supported values: `virtio`, `sata`, `scsi`, `usb`.
	Bus virtv1.DiskBus `json:"bus,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:JSONPath=".status.ipAddress",name=IPAddress,type=string
//+kubebuilder:printcolumn:JSONPath=".status.phase",name=Status,type=string
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:resource:shortName={"vm","vms"}

// Defines virtual machine.
type VirtualMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineSpec   `json:"spec,omitempty"`
	Status VirtualMachineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// Contains a list of `VirtualMachine`.
type VirtualMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualMachine{}, &VirtualMachineList{})
}
