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

// Package v1 contains the YandexInstanceClass CRD root type.
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

const (
	YandexInstanceClassGroupName = "deckhouse.io"
	YandexInstanceClassVersion   = "v1"
	YandexInstanceClassKind      = "YandexInstanceClass"
)

var (
	_ cpapi.InstanceClassObject = (*YandexInstanceClass)(nil)

	GroupVersionKind = schema.GroupVersionKind{Group: YandexInstanceClassGroupName, Version: YandexInstanceClassVersion, Kind: YandexInstanceClassKind}
)

type YandexInstanceClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   YandexInstanceClassSpec   `json:"spec"`
	Status YandexInstanceClassStatus `json:"status,omitempty"`
}

// YandexInstanceClassSpec defines the desired state of the YandexInstanceClass.
type YandexInstanceClassSpec struct {
	// Amount of CPU cores to provision on a Yandex Compute Instance.
	Cores int `json:"cores"`

	// Percent of reserved CPU capacity on a Yandex Compute Instance.
	//
	// [Details...](https://cloud.yandex.com/en/docs/compute/concepts/performance-levels)
	CoreFraction int `json:"coreFraction,omitempty"`

	// Number of GPUs on a Yandex Compute Instance.
	GPUs int `json:"gpus,omitempty"`

	// Amount of primary memory in MB provision on a Yandex Compute Instance.
	Memory int `json:"memory"`

	// Image ID to use while provisioning Yandex Compute Instances.
	//
	// The [masterNodeGroup.instanceClass.imageID](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-instanceclass-imageid) parameter will be used by default.
	ImageID string `json:"imageID"`

	// Platform ID.
	//
	// [List of available platforms...](https://cloud.yandex.com/en-ru/docs/compute/concepts/vm-platforms)
	PlatformID string `json:"platformID,omitempty"`

	// Should a provisioned Yandex Compute Instance be preemptible.
	//
	// For more information about preemptible virtual machines, read the [provider's documentation](https://cloud.yandex.com/en/docs/compute/concepts/preemptible-vm).
	Preemptible bool `json:"preemptible,omitempty"`

	// Instance [disk type](https://cloud.yandex.com/en-ru/docs/compute/concepts/disk#disks_types).
	//
	// Size of `network-ssd-nonreplicated` and `network-ssd-io-m3` disks must be a multiple of 93 GB.
	DiskType string `json:"diskType,omitempty"`

	// Yandex Compute Instance disk size in gibibytes.
	DiskSizeGB int `json:"diskSizeGB,omitempty"`

	// Should a public external IPv4 address be assigned to a provisioned Yandex Compute Instance.
	AssignPublicIPAddress bool `json:"assignPublicIPAddress,omitempty"`

	// Subnet ID that VirtualMachines' primary NIC will connect to.
	//
	// If the parameter is not specified, the main network is determined automatically according to the following logic:
	// if a list of networks is set in the [existingZoneToSubnetIDMap](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap) parameter,
	// then the network is selected from the specified list; otherwise, the created Deckhouse network is used.
	MainSubnet string `json:"mainSubnet,omitempty"`

	// Subnet IDs that VirtualMachines' secondary NICs will connect to.
	//
	// For `CloudEphemeral` nodes, every subnet in the list is attached as a separate network interface.
	//
	// For `CloudPermanent` nodes, a single subnet is selected from the list by the node index,
	// so the list must contain at least as many subnets as there are nodes in the group.
	AdditionalSubnets []string `json:"additionalSubnets,omitempty"`

	// Additional labels to be attached to the Yandex Compute Instance resource.
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"`

	// Network type: `Standard` or [`SoftwareAccelerated`](https://cloud.yandex.com/en/docs/vpc/concepts/software-accelerated-network).
	NetworkType string `json:"networkType,omitempty"`

	// Disk size for etcd is set only for InstanceClass intended for master NodeGroup. The value is specified in `GiB`.
	EtcdDiskSizeGB *int `json:"etcdDiskSizeGB,omitempty" yaml:"etcdDiskSizeGB,omitempty"`
}

// YandexInstanceClassStatus stores information about the YandexInstanceClass resource.
type YandexInstanceClassStatus struct {
	// NodeGroupConsumers lists the names of NodeGroups that use this instance class.
	NodeGroupConsumers []string `json:"nodeGroupConsumers,omitempty"`
}

// GroupVersionKind returns the GroupVersionKind for the resource.
func (c *YandexInstanceClass) GroupVersionKind() cpapi.GroupVersionKind {
	return cpapi.GroupVersionKind{Group: YandexInstanceClassGroupName, Version: YandexInstanceClassVersion, Kind: YandexInstanceClassKind}
}

// GetEtcdDisk returns the etcd disk value for error reporting, or nil when the class
// defines no dedicated etcd disk.
func (c *YandexInstanceClass) GetEtcdDisk() any {
	if c == nil || c.Spec.EtcdDiskSizeGB == nil {
		return nil
	}
	return c.Spec.EtcdDiskSizeGB
}

// GetNodeGroupConsumers returns names of NodeGroups that use the class.
func (c *YandexInstanceClass) GetNodeGroupConsumers() []string {
	if c == nil {
		return nil
	}

	return c.Status.NodeGroupConsumers
}
