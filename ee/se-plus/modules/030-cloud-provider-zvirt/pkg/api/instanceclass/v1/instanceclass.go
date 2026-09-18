/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// Package v1 contains the ZvirtInstanceClass CRD root type.
//
// +groupName=deckhouse.io
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

const (
	ZvirtInstanceClassGroupName = "deckhouse.io"
	ZvirtInstanceClassVersion   = "v1"
	ZvirtInstanceClassKind      = "ZvirtInstanceClass"
)

var (
	_ cpapi.InstanceClassObject = (*ZvirtInstanceClass)(nil)

	GroupVersionKind = schema.GroupVersionKind{Group: ZvirtInstanceClassGroupName, Version: ZvirtInstanceClassVersion, Kind: ZvirtInstanceClassKind}
)

// Parameters of a group of zVirt VirtualMachines used by `machine-controller-manager` (the [node-manager](https://deckhouse.io/modules/node-manager/) module).
//
// The `CloudInstanceClass` resource of the `node-manager` module refers to this resource.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=cloudinstanceclasses
// +kubebuilder:printcolumn:name="Node Groups",type=string,JSONPath=.status.nodeGroupConsumers,description="NodeGroups which use this instance class."
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=.metadata.creationTimestamp
// +kubebuilder:metadata:labels="heritage=deckhouse"
// +kubebuilder:metadata:labels="module=cloud-provider-zvirt"
// +kubebuilder:storageversion
// +deckhouse:ru:description:value="Параметры группы виртуальных машин zVirt, которые будет использовать `machine-controller-manager` (модуль [node-manager](https://deckhouse.ru/modules/node-manager/))."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="На этот ресурс ссылается ресурс `CloudInstanceClass` модуля `node-manager`."
type ZvirtInstanceClass struct {
	// +deckhouse:XDocSkip
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceClassSpec   `json:"spec"`
	Status InstanceClassStatus `json:"status,omitempty"`
}

// InstanceClassSpec defines the desired state of the ZvirtInstanceClass.
// +deckhouse:ru:description:value="Определяет параметры виртуальной машины zVirt."
type InstanceClassSpec struct {
	// Number of vCPUs to allocate to the VM.
	// +deckhouse:ru:description:value="Количество vCPU, выделяемых виртуальным машинам."
	// +kubebuilder:validation:Minimum=1
	// +deckhouse:XDocExamples:value=2
	NumCPUs int `json:"numCPUs"`

	// Memory in MiB to allocate to the VM.
	// +deckhouse:ru:description:value="Память в MiB для выделения виртуальным машинам."
	// +kubebuilder:validation:Minimum=1
	// +deckhouse:XDocExamples:value=8192
	Memory int `json:"memory"`

	// Root disk size in GiB to use in zVirt VirtualMachines.
	//
	// The disk will be automatically resized if its size in the template differs from specified.
	// +deckhouse:ru:description:value="Размер корневого диска в GiB для использования в виртуальных машинах."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Размер диска будет изменён автоматически, если он отличается от размера диска в шаблоне."
	// +deckhouse:XDocExamples:value=50
	// +deckhouse:XDocDefault:value=50
	// +optional
	RootDiskSizeGb int `json:"rootDiskSizeGb,omitempty"`

	// Etcd disk size in GiB.
	//
	// Set only for an InstanceClass intended for the master NodeGroup.
	// +deckhouse:ru:description:value="Размер диска для etcd в GiB."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Устанавливается только для InstanceClass, предназначенного для группы master-узлов."
	// +deckhouse:XDocExamples:value=10
	// +deckhouse:XDocDefault:value=10
	// +optional
	EtcdDiskSizeGb *int `json:"etcdDiskSizeGb,omitempty"`

	// Template name to be cloned.
	// +deckhouse:ru:description:value="Имя шаблона, который будет использован при заказе виртуальных машин."
	// +deckhouse:XDocExamples:value="debian-bookworm"
	Template string `json:"template"`

	// Virtual NIC profile ID on the basis of which the virtual NIC will be created.
	// +deckhouse:ru:description:value="ID профиля vNIC."
	// +kubebuilder:validation:Pattern=`^[\da-fA-F]{8}\-[\da-fA-F]{4}\-[\da-fA-F]{4}\-[\da-fA-F]{4}\-[\da-fA-F]{12}$`
	// +deckhouse:XDocExamples:value="49bb4594-0cd4-4eb7-8288-8594eafd5a86"
	VNICProfileID string `json:"vnicProfileID"`

	// Storage domain id which contains the shared resources that must be available to all datacenter hosts.
	// +deckhouse:ru:description:value="ID домена хранения."
	// +kubebuilder:validation:Pattern=`^[\da-fA-F]{8}\-[\da-fA-F]{4}\-[\da-fA-F]{4}\-[\da-fA-F]{4}\-[\da-fA-F]{12}$`
	// +deckhouse:XDocExamples:value="49bb4594-0cd4-4eb7-8288-8594eafd5a86"
	// +optional
	StorageDomainID string `json:"storageDomainID,omitempty"`
}

// InstanceClassStatus stores information about the ZvirtInstanceClass resource.
// +deckhouse:ru:description:value="Содержит информацию о группах узлов, использующих данный экземпляр класса."
type InstanceClassStatus struct {
	// NodeGroupConsumers lists the names of NodeGroups that use this instance class.
	// +deckhouse:ru:description:value="Список имён NodeGroup, использующих этот instance class."
	// +optional
	NodeGroupConsumers []string `json:"nodeGroupConsumers,omitempty"`
}

// GroupVersionKind returns the GroupVersionKind for the resource.
func (c *ZvirtInstanceClass) GroupVersionKind() cpapi.GroupVersionKind {
	return cpapi.GroupVersionKind{Group: ZvirtInstanceClassGroupName, Version: ZvirtInstanceClassVersion, Kind: ZvirtInstanceClassKind}
}

// GetEtcdDisk returns the etcd disk value for error reporting, or nil when the class
// defines no dedicated etcd disk.
func (c *ZvirtInstanceClass) GetEtcdDisk() any {
	if c == nil || c.Spec.EtcdDiskSizeGb == nil {
		return nil
	}

	return c.Spec.EtcdDiskSizeGb
}

// EtcdDiskFieldName reports the field zVirt declares the etcd disk in, so the shared rules point
// the operator at etcdDiskSizeGb rather than at the etcdDisk other providers use.
func (c *ZvirtInstanceClass) EtcdDiskFieldName() string {
	return "etcdDiskSizeGb"
}

// GetNodeGroupConsumers returns names of NodeGroups that use the class.
func (c *ZvirtInstanceClass) GetNodeGroupConsumers() []string {
	if c == nil {
		return nil
	}

	return c.Status.NodeGroupConsumers
}
