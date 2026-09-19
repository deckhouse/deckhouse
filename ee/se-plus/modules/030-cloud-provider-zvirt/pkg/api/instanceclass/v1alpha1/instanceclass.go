/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// Package v1alpha1 contains the deprecated ZvirtInstanceClass CRD root type.
//
// The schema is frozen: it keeps describing objects created before v1 became the storage version.
// New fields belong to v1 only, and the migration creates v1 objects exclusively.
//
// +groupName=deckhouse.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

const (
	ZvirtInstanceClassGroupName = "deckhouse.io"
	ZvirtInstanceClassVersion   = "v1alpha1"
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
// +kubebuilder:metadata:labels="heritage=deckhouse"
// +kubebuilder:metadata:labels="module=cloud-provider-zvirt"
// +deckhouse:ru:description:value="Параметры группы Zvirt servers, которые будет использовать `machine-controller-manager` (модуль [node-manager](https://deckhouse.ru/modules/node-manager/))."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="На этот ресурс ссылается ресурс `CloudInstanceClass` модуля `node-manager`."
type ZvirtInstanceClass struct {
	// +deckhouse:XDocSkip
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceClassSpec   `json:"spec"`
	Status InstanceClassStatus `json:"status,omitempty"`
}

type InstanceClassSpec struct {
	// Count of vCPUs to allocate to zVirt VirtualMachines.
	// +deckhouse:ru:description:value="Количество vCPU, выделяемых виртуальным машинам zVirt."
	// +deckhouse:XDocExamples:value=2
	NumCPUs int `json:"numCPUs"`

	// Memory in MiB to allocate to zVirt VirtualMachines.
	// +deckhouse:ru:description:value="Память в MiB для выделения виртуальным машинам zVirt VirtualMachines."
	// +deckhouse:XDocExamples:value=8192
	Memory int `json:"memory"`

	// Root disk size in GiB to use in zVirt VirtualMachines.
	//
	// The disk will be automatically resized if its size in the template differs from specified.
	// +deckhouse:ru:description:value="Размер корневого диска в GiB для использования в виртуальных машинах zVirt."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Размер диска будет изменён автоматически, если он отличается от размера диска в шаблоне."
	// +deckhouse:XDocExamples:value=30
	// +deckhouse:XDocDefault:value=30
	// +optional
	RootDiskSizeGb int `json:"rootDiskSizeGb,omitempty"`

	// Template name to be cloned.
	// +deckhouse:ru:description:value="Имя шаблона, из которого будут клонированы ВМ."
	// +deckhouse:XDocExamples:value="debian-bookworm"
	Template string `json:"template"`

	// Virtual NIC profile ID on the basis of which the virtual NIC will be created.
	// +deckhouse:ru:description:value="vNIC профиль id."
	// +kubebuilder:validation:Pattern=`^[0-9a-fA-F]{8}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{12}$`
	// +deckhouse:XDocExamples:value="49bb4594-0cd4-4eb7-8288-8594eafd5a86"
	VNICProfileID string `json:"vnicProfileID"`
}

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

// GetEtcdDisk always reports no dedicated etcd disk: v1alpha1 defines no field for one, so an
// object of this version never carries it.
func (c *ZvirtInstanceClass) GetEtcdDisk() any {
	return nil
}

// GetNodeGroupConsumers returns names of NodeGroups that use the class.
func (c *ZvirtInstanceClass) GetNodeGroupConsumers() []string {
	if c == nil {
		return nil
	}

	return c.Status.NodeGroupConsumers
}
