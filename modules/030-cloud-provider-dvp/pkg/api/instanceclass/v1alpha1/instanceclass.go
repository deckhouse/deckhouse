// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package v1alpha1 contains the DVPInstanceClass CRD root type.
//
// +groupName=deckhouse.io
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DVPInstanceClass is the CRD root type for DVP instance classes.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:storageversion
type DVPInstanceClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec InstanceClassSpec `json:"spec"`
}

type InstanceClassSpec struct {
	VirtualMachine InstanceClassVirtualMachine `json:"virtualMachine"`
	RootDisk       InstanceClassRootDisk       `json:"rootDisk"`
	// +optional
	// Parameters for additional virtual machine disks.
	AdditionalDisks []InstanceClassDisk `json:"additionalDisks,omitempty"`
	// Specifies settings for the etcd data disk.
	EtcdDisk InstanceClassDisk `json:"etcdDisk,omitempty"`
}

// Virtual machine settings for the created node.
//
// > The `runPolicy: AlwaysOnUnlessStoppedManually` policy is used for virtual machines of nodes.
// > This allows the virtual machine to be stopped manually (for example, for maintenance) without triggering an automatic restart.
type InstanceClassVirtualMachine struct {
	CPU    InstanceClassVirtualMachineCPU    `json:"cpu"`
	Memory InstanceClassVirtualMachineMemory `json:"memory"`
	// The name of the VirtualMachineClass.
	//
	// Intended for centralized configuration of preferred virtual machine parameters. It allows you to specify CPU instruction sets, resource configuration policies for CPU and memory, and define the ratio between these resources.
	VirtualMachineClassName string `json:"virtualMachineClassName"`
	// Defines a bootloader for the virtual machine.
	//
	// * `BIOS`: Use BIOS.
	// * `EFI`: Use Unified Extensible Firmware (EFI/UEFI).
	// * `EFIWithSecureBoot`: Use UEFI/EFI with the Secure Boot support.
	// +kubebuilder:validation:Enum=BIOS;EFI;EFIWithSecureBoot
	// +kubebuilder:default="EFI"
	Bootloader string `json:"bootloader,omitempty"`
	// Virtual machine run policy.
	//
	// * `AlwaysOn`: The virtual machine should always be running.
	// * `AlwaysOff`: The virtual machine should always be stopped.
	// * `Manual`: The virtual machine state is controlled manually.
	// * `AlwaysOnUnlessStoppedManually`: The virtual machine can be stopped manually (for example, for maintenance), but it will automatically start after a host reboot.
	//
	// +kubebuilder:validation:Enum=AlwaysOn;AlwaysOff;Manual;AlwaysOnUnlessStoppedManually
	// +kubebuilder:default="AlwaysOnUnlessStoppedManually"
	// +deckhouse:XDocExample:value="AlwaysOnUnlessStoppedManually"
	RunPolicy string `json:"runPolicy,omitempty"`
	// Live migration policy for the virtual machine.
	//
	// * `Manual`: Migration is controlled manually.
	// * `Never`: Migration is disabled.
	// * `AlwaysSafe`: Always use safe migration (may fail if VM has a high rate of memory changes).
	// * `PreferSafe`: Prefer safe migration, fallback to forced if needed.
	// * `AlwaysForced`: Always use forced migration with VM slowdown.
	// * `PreferForced`: Prefer forced migration (recommended for master nodes due to high memory activity).
	// +kubebuilder:validation:Enum=Manual;Never;AlwaysSafe;PreferSafe;AlwaysForced;PreferForced
	// +kubebuilder:default="PreferForced"
	// +deckhouse:XDocExample:value="PreferForced"
	LiveMigrationPolicy string `json:"liveMigrationPolicy,omitempty"`
	// Additional labels for a virtual machine resource.
	// +deckhouse:XDocExample:value="```yaml\ncluster-owner: user\n```"
	// +optional
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"`
	// Additional annotations for a virtual machine resource.
	// +deckhouse:XDocExample:value="```yaml\ncluster-owner: user\n```"
	// +optional
	AdditionalAnnotations map[string]string `json:"additionalAnnotations,omitempty"`
	// Allows a virtual machine to be assigned to specified DVP nodes.
	// [The same](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes/) as in the `spec.nodeSelector` parameter for Kubernetes Pods.
	// +optional
	NodeSelector corev1.NodeSelector `json:"nodeSelector,omitempty"`
	// [The same](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/) as in the `spec.priorityClassName` parameter for Kubernetes Pods.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// Allows setting tolerations for virtual machines for a DVP node.
	// [The same](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) as in the `spec.tolerations` parameter in Kubernetes Pods.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// CPU settings for the virtual machine.
type InstanceClassVirtualMachineCPU struct {
	// Number of CPU cores for the virtual machine.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Format=int32
	// +deckhouse:XDocExample:value="4"
	Cores int `json:"cores"`
	// Guaranteed share of CPU fraction that will be allocated to the virtual machine.
	// +kubebuilder:default="100%"
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Pattern=`^100%$|^[1-9][0-9]?%$`
	// +deckhouse:XDocExample:value="100%"
	// +optional
	CoreFraction string `json:"coreFraction,omitempty"`
}

// Specifies the memory settings for the virtual machine.
type InstanceClassVirtualMachineMemory struct {
	// Amount of memory resources allowed for the virtual machine.
	//
	// +kubebuilder:validation:Pattern=`^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$`
	// +deckhouse:XDocExample:value="4Gi"
	Size string `json:"size"`
}

// Image parameters that will be used to create the virtual machine's root disk.
type InstanceClassImage struct {
	// The kind of the image source.
	// +kubebuilder:validation:Enum=ClusterVirtualImage;VirtualImage;VirtualDisk
	Kind string `json:"kind"`
	// The name of the image that will be used to create the root disk.
	//
	// > The installation requires Linux OS images with cloud-init pre-installed.
	Name string `json:"name"`
}

// Specifies settings for the root disk of the virtual machine.
type InstanceClassRootDisk struct {
	// Root disk size.
	// +kubebuilder:validation:Pattern=`^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$`
	// +deckhouse:XDocExample:value="10Gi"
	Size string `json:"size"`
	// The name of the existing StorageClass will be used to create the virtual machine's root disk.
	//
	// If the value is not specified, the StorageClass will be used according to the [global storageClass parameter](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-storageclass) setting.
	// +optional
	StorageClass string             `json:"storageClass,omitempty"`
	Image        InstanceClassImage `json:"image"`
}

type InstanceClassDisk struct {
	// Size of the disk.
	// +kubebuilder:validation:Pattern=`^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$`
	// +deckhouse:XDocExample:value="10Gi"
	Size string `json:"size"`
	// Name of the existing StorageClass that will be used to create the disk.
	StorageClass string `json:"storageClass"`
}
