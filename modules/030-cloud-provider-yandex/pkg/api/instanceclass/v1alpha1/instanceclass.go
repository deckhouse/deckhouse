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

// Package v1alpha1 contains the YandexInstanceClass CRD root type.
//
// +groupName=deckhouse.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

const (
	YandexInstanceClassGroupName = "deckhouse.io"
	YandexInstanceClassVersion   = "v1alpha1"
	YandexInstanceClassKind      = "YandexInstanceClass"
)

var (
	_ cpapi.InstanceClassObject = (*YandexInstanceClass)(nil)

	GroupVersionKind = schema.GroupVersionKind{Group: YandexInstanceClassGroupName, Version: YandexInstanceClassVersion, Kind: YandexInstanceClassKind}
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=cloudinstanceclasses
// +kubebuilder:metadata:labels="heritage=deckhouse"
// +kubebuilder:metadata:labels="module=cloud-provider-yandex"
// +deckhouse:ru:description:value="Ресурс YandexInstanceClass содержит описание шаблона виртуальной машины Yandex Compute Cloud для создания узлов."
type YandexInstanceClass struct {
	// +deckhouse:XDocSkip
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec InstanceClassSpec `json:"spec"`
}

// +deckhouse:ru:description:value="Определяет параметры виртуальной машины Yandex Compute Cloud."
// InstanceClassSpec defines the desired state of the YandexInstanceClass.
type InstanceClassSpec struct {
	// Amount of CPU cores to provision on a Yandex Compute Instance.
	// +deckhouse:ru:description:value="Количество ядер CPU, выделяемых виртуальной машине."
	// +deckhouse:XDocExamples:value="4"
	Cores int `json:"cores"`

	// Percent of reserved CPU capacity on a Yandex Compute Instance.
	//
	// [Details...](https://cloud.yandex.com/en/docs/compute/concepts/performance-levels)
	// +deckhouse:ru:description:value="Базовый уровень производительности каждого ядра CPU у создаваемых виртуальных машин."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="[Подробнее об уровнях производительности](https://cloud.yandex.ru/docs/compute/concepts/performance-levels)."
	// +kubebuilder:validation:Enum=5;20;50;100
	// +deckhouse:XDocExamples:value="20"
	// +deckhouse:XDocDefault:value="100"
	// +kubebuilder:default=100
	// +optional
	CoreFraction int `json:"coreFraction,omitempty"`

	// Number of GPUs on a Yandex Compute Instance.
	// +deckhouse:ru:description:value="Количество графических адаптеров у создаваемых виртуальных машин."
	// +deckhouse:XDocExamples:value="4"
	// +deckhouse:XDocDefault:value="0"
	// +kubebuilder:default=0
	// +optional
	GPUs int `json:"gpus,omitempty"`

	// Amount of primary memory in MB provision on a Yandex Compute Instance.
	// +deckhouse:ru:description:value="Количество оперативной памяти (в мегабайтах) у создаваемых виртуальных машин."
	// +deckhouse:XDocExamples:value="8192"
	Memory int `json:"memory"`

	// Image ID to use while provisioning Yandex Compute Instances.
	//
	// The [masterNodeGroup.instanceClass.imageID](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-instanceclass-imageid) parameter will be used by default.
	// +deckhouse:ru:description:value="Идентификатор образа, который будет установлен в заказанные виртуальные машины."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="По умолчанию используется образ, указанный в параметре [masterNodeGroup.instanceClass.imageID](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-instanceclass-imageid)."
	// +deckhouse:XDocExamples:value="fd83ica41cade1mj35sr"
	ImageID string `json:"imageID"`

	// Platform ID.
	//
	// [List of available platforms...](https://cloud.yandex.com/en-ru/docs/compute/concepts/vm-platforms)
	// +deckhouse:ru:description:value="ID платформы."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="[Список существующих платформ](https://cloud.yandex.com/docs/compute/concepts/vm-platforms)."
	// +deckhouse:XDocDefault:value="standard-v2"
	// +kubebuilder:default="standard-v2"
	// +optional
	PlatformID string `json:"platformID,omitempty"`

	// Should a provisioned Yandex Compute Instance be preemptible.
	//
	// For more information about preemptible virtual machines, read the [provider's documentation](https://cloud.yandex.com/en/docs/compute/concepts/preemptible-vm).
	// +deckhouse:ru:description:value="Необходимость заказа прерываемых виртуальных машин (preemptible-инстансов)."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Подробнее о прерываемых виртуальных машинах можно узнать в [документации провайдера](https://cloud.yandex.ru/docs/compute/concepts/preemptible-vm)."
	// +deckhouse:XDocDefault:value="false"
	// +kubebuilder:default=false
	// +optional
	Preemptible bool `json:"preemptible,omitempty"`

	// Instance disk type.
	//
	// Size of `network-ssd-nonreplicated` and `network-ssd-io-m3` disks must be a multiple of 93 GB.
	//
	// For more information about possible disk types, read the [provider's documentation](https://cloud.yandex.com/en-ru/docs/compute/concepts/disk#disks_types).
	// +deckhouse:ru:description:value="Тип диска у виртуальных машин."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Размер дисков `network-ssd-nonreplicated` и `network-ssd-io-m3` должен быть кратен 93 GB."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Подробнее о возможных типах дисков можно узнать в [документации провайдера](https://cloud.yandex.com/docs/compute/concepts/disk#disks_types)."
	// +kubebuilder:validation:Enum=network-ssd;network-hdd;network-ssd-nonreplicated
	// +deckhouse:XDocExamples:value="network-hdd"
	// +deckhouse:XDocDefault:value="\"network-hdd\""
	// +kubebuilder:default="network-hdd"
	// +optional
	DiskType string `json:"diskType,omitempty"`

	// Yandex Compute Instance disk size in gibibytes.
	// +deckhouse:ru:description:value="Размер диска у виртуальных машин. Значение указывается в `ГиБ`."
	// +deckhouse:XDocExamples:value="50"
	// +deckhouse:XDocDefault:value="20"
	// +kubebuilder:default=20
	// +optional
	DiskSizeGB int `json:"diskSizeGB,omitempty"`

	// Should a public external IPv4 address be assigned to a provisioned Yandex Compute Instance.
	// +deckhouse:ru:description:value="Необходимость присвоения публичных IP-адресов виртуальным машинам."
	// +deckhouse:XDocExamples:value="false"
	// +deckhouse:XDocDefault:value="false"
	// +kubebuilder:default=false
	// +optional
	AssignPublicIPAddress bool `json:"assignPublicIPAddress,omitempty"`

	// Subnet ID that VirtualMachines' primary NIC will connect to.
	//
	// If the parameter is not specified, the main network is determined automatically according to the following logic:
	// if a list of networks is set in the [existingZoneToSubnetIDMap](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap) parameter,
	// then the network is selected from the specified list; otherwise, the created Deckhouse network is used.
	// +deckhouse:ru:description:value="Имя основной сети (ID), к которой будет подключен основной сетевой интерфейс виртуальной машины."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Если параметр не задан, то основная сеть определяется автоматически согласно следующей логике: если задан список сетей в параметре [existingZoneToSubnetIDMap](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap), то сеть определяется из указанного списка, иначе используется созданная Deckhouse сеть."
	// +deckhouse:XDocExamples:value="e9bnc7g9mu9mper9clk4"
	// +optional
	MainSubnet string `json:"mainSubnet,omitempty"`

	// Subnet IDs that VirtualMachines' secondary NICs will connect to.
	//
	// For `CloudEphemeral` nodes, every subnet in the list is attached as a separate network interface.
	//
	// For `CloudPermanent` nodes, a single subnet is selected from the list by the node index,
	// so the list must contain at least as many subnets as there are nodes in the group.
	// +deckhouse:ru:description:value="Список дополнительных подсетей, которые будут подключены к виртуальной машине."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Для узлов типа `CloudEphemeral` каждая подсеть из списка подключается как отдельный сетевой интерфейс."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Для узлов типа `CloudPermanent` из списка выбирается одна подсеть по индексу узла, поэтому список должен содержать не меньше подсетей, чем узлов в группе."
	// +deckhouse:XDocExamples:value="[b0csh41c1or82vuch89v, e2lgddi5svochh5fbq96]"
	// +optional
	AdditionalSubnets []string `json:"additionalSubnets,omitempty"`

	// Additional labels to be attached to the Yandex Compute Instance resource.
	// +deckhouse:ru:description:value="Дополнительные лейблы, которые будут присвоены созданным виртуальным машинам."
	// +deckhouse:XDocExamples:value="{project: cms-production, severity: critical}"
	// +optional
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"`

	// Network type: STANDARD or SOFTWARE_ACCELERATED
	// +deckhouse:ru:description:value="Тип сети: обычная или [программно-ускоренная](https://cloud.yandex.ru/docs/vpc/concepts/software-accelerated-network)."
	// +kubebuilder:default="STANDARD"
	// +kubebuilder:validation:Enum=STANDARD;SOFTWARE_ACCELERATED
	// +optional
	NetworkType string `json:"networkType,omitempty"`
}

// GroupVersionKind returns the GroupVersionKind for the resource.
func (c *YandexInstanceClass) GroupVersionKind() cpapi.GroupVersionKind {
	return cpapi.GroupVersionKind{Group: YandexInstanceClassGroupName, Version: YandexInstanceClassVersion, Kind: YandexInstanceClassKind}
}

// GetEtcdDisk returns the etcd disk value for error reporting, or nil when the class
// defines no dedicated etcd disk. The v1alpha1 spec has no etcd disk field at all —
// it appeared in v1 — so this is always nil rather than an unfinished stub.
func (c *YandexInstanceClass) GetEtcdDisk() any {
	return nil
}

// GetNodeGroupConsumers returns names of NodeGroups that use the class.
// The v1alpha1 resource carries no status subresource, so there is nothing to report.
func (c *YandexInstanceClass) GetNodeGroupConsumers() []string {
	return nil
}
