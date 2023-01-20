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

// VirtualMachineSpec defines the desired state of VirtualMachine
type VirtualMachineSpec struct {
	// Running state indicates the requested running state of the VirtualMachineInstance
	// mutually exclusive with Running
	Running *bool `json:"running,omitempty"`
	// IPAddressClaimName defines the name for associated VirtualMahcineIPAddressClaim resource, if not specified defaults to {vm name}
	IPAddressClaimName *string         `json:"ipAddressClaimName,omitempty"`
	Resources          v1.ResourceList `json:"resources,omitempty"`
	// UserName defines the user name that should be automatically created inside the VM. This option requires cloud-init in the virtual machine.
	UserName *string `json:"userName,omitempty"`
	// SSHPublicKey defines the SSH public key that should be automatically added to user inside the VM. This option requires cloud-init in the virtual machine.
	SSHPublicKey *string   `json:"sshPublicKey,omitempty"`
	BootDisk     *BootDisk `json:"bootDisk,omitempty"`
	// CloudInit represents a cloud-init nocloud user data source.
	// More info: https://cloudinit.readthedocs.io/en/latest/reference/datasources/nocloud.html
	CloudInit *virtv1.CloudInitNoCloudSource `json:"cloudInit,omitempty"`
	// DiskAttachments represents a lits of additional disks that should be attached to the virtual machine
	DiskAttachments *[]DiskSource `json:"diskAttachments,omitempty"`
}

// VirtualMachineStatus defines the observed state of VirtualMachine
type VirtualMachineStatus struct {
	// Phase is a human readable, high-level representation of the status of the virtual machine
	Phase virtv1.VirtualMachinePrintableStatus `json:"phase,omitempty"`
	// IP address of Virtual Machine
	IPAddress string `json:"ipAddress,omitempty"`
}

// BootDisk defines the boot disk for virtual machine
type BootDisk struct {
	// Name for virtual machine boot disk, if not specified defaults to {vm name}-boot
	Name string `json:"name,omitempty"`
	// Source represents the source of a boot disk, if specified the new disk will be created
	Source *corev1.TypedLocalObjectReference `json:"source,omitempty"`
	// StorageClassName represents the storage class for newly created disk
	StorageClassName string `json:"storageClassName,omitempty"`
	// Size represents the size for newly created disk
	Size resource.Quantity `json:"size"`
	// AutoDelete enables automatic removal of associated boot disk after removing the virtual machine
	AutoDelete bool `json:"autoDelete,omitempty"`
	// Bus indicates the type of disk device to emulate.
	// supported values: virtio, sata, scsi, usb.
	Bus string `json:"bus,omitempty"`
}

// ImageSourceScope represents the source of the image.
// +enum
type ImageSourceScope string

const (
	// ImageSourceScopePublic indicates that disk should be
	// created from public image. This is the default mode.
	ImageSourceScopePublic ImageSourceScope = "public"

	// ImageSourceScopePrivate indicates that disk should be
	// created from private image from the same namespace.
	ImageSourceScopePrivate ImageSourceScope = "private"
)

// Represents the source of existing disk
type DiskSource struct {
	// Name represents the name of the Disk in the same namespace
	Name string `json:"name"`
	// Hotpluggable indicates whether the volume can be hotplugged and hotunplugged.
	// +optional
	Hotpluggable bool `json:"hotpluggable,omitempty"`
	// Bus indicates the type of disk device to emulate.
	// supported values: virtio, sata, scsi, usb.
	Bus virtv1.DiskBus `json:"bus,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:JSONPath=".status.ipAddress",name=IPAddress,type=string
//+kubebuilder:printcolumn:JSONPath=".status.phase",name=Status,type=string
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:resource:shortName={"vm","vms"}

// VirtualMachine represents the resource that defines virtual machine
type VirtualMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineSpec   `json:"spec,omitempty"`
	Status VirtualMachineStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VirtualMachineList contains a list of VirtualMachine
type VirtualMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualMachine{}, &VirtualMachineList{})
}
