/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// Package v1 mirrors the ZvirtInstanceClass spec for the module hooks.
//
// The canonical, marker-annotated definition lives in pkg/api/instanceclass/v1 and drives
// crds/instance_class.yaml. Hooks compile inside the repository-root go.mod, which does not
// require the module's own go.mod, so the shape is repeated here without the generator markers.
//
// Only v1 is mirrored: v1alpha1 is frozen and the migration creates v1 objects exclusively.
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ZvirtInstanceClassGroupName = "deckhouse.io"
	ZvirtInstanceClassVersion   = "v1"
	ZvirtInstanceClassKind      = "ZvirtInstanceClass"
)

var (
	GroupVersionKind = schema.GroupVersionKind{Group: ZvirtInstanceClassGroupName, Version: ZvirtInstanceClassVersion, Kind: ZvirtInstanceClassKind}
)

// Parameters of a group of zVirt VirtualMachines used by `machine-controller-manager` (the [node-manager](https://deckhouse.io/modules/node-manager/) module).
//
// The `CloudInstanceClass` resource of the `node-manager` module refers to this resource.
type ZvirtInstanceClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceClassSpec   `json:"spec"`
	Status InstanceClassStatus `json:"status,omitempty"`
}

// InstanceClassSpec defines the desired state of the ZvirtInstanceClass.
type InstanceClassSpec struct {
	// Number of vCPUs to allocate to the VM.
	NumCPUs int `json:"numCPUs"`

	// Memory in MiB to allocate to the VM.
	Memory int `json:"memory"`

	// Root disk size in GiB to use in zVirt VirtualMachines.
	//
	// The disk will be automatically resized if its size in the template differs from specified.
	RootDiskSizeGb int `json:"rootDiskSizeGb,omitempty"`

	// Etcd disk size in GiB.
	//
	// Set only for an InstanceClass intended for the master NodeGroup.
	EtcdDiskSizeGb *int `json:"etcdDiskSizeGb,omitempty"`

	// Template name to be cloned.
	Template string `json:"template"`

	// Virtual NIC profile ID on the basis of which the virtual NIC will be created.
	VNICProfileID string `json:"vnicProfileID"`

	// Storage domain id which contains the shared resources that must be available to all datacenter hosts.
	StorageDomainID string `json:"storageDomainID,omitempty"`
}

// InstanceClassStatus stores information about the ZvirtInstanceClass resource.
type InstanceClassStatus struct {
	// NodeGroupConsumers lists the names of NodeGroups that use this instance class.
	NodeGroupConsumers []string `json:"nodeGroupConsumers,omitempty"`
}
