/*
Copyright 2022 Flant JSC

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeType type of node
type NodeType string

func (nt NodeType) String() string {
	return string(nt)
}

// NodeGroup is a group of nodes in Kubernetes.
type NodeGroup struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a node group.
	Spec NodeGroupSpec `json:"spec"`

	// Most recently observed status of the node.
	// Populated by the system.

	Status NodeGroupStatus `json:"status,omitempty"`
}

type NodeGroupSpec struct {
	// Type of nodes in group: CloudEphemeral, CloudPermanent, CloudStatic, Static. Field is required
	NodeType NodeType `json:"nodeType,omitempty"`

	// cloudInstances. Optional.
	CloudInstances CloudInstances `json:"cloudInstances,omitempty"`
}

// CloudInstances is an extra parameters for NodeGroup with type Cloud.
type CloudInstances struct {
	// Minimal amount of instances for the group in each zone. Required.
	MinPerZone *int32 `json:"minPerZone,omitempty"`

	// Maximum amount of instances for the group in each zone. Required.
	MaxPerZone *int32 `json:"maxPerZone,omitempty"`
}

type NodeGroupStatus struct {
	// Number of ready Kubernetes nodes in the group.
	Ready int32 `json:"ready,omitempty"`

	// Number of Kubernetes nodes (in any state) in the group.
	Nodes int32 `json:"nodes,omitempty"`

	// Number of instances (in any state) in the group.
	Instances int32 `json:"instances,omitempty"`

	// Number of desired machines in the group.
	Desired int32 `json:"desired,omitempty"`

	// Minimal amount of instances in the group.
	Min int32 `json:"min,omitempty"`

	// Maximum amount of instances in the group.
	Max int32 `json:"max,omitempty"`

	// Number of up-to-date nodes in the group.
	UpToDate int32 `json:"upToDate,omitempty"`

	// Number of overprovisioned instances in the group.
	Standby int32 `json:"standby,omitempty"`

	// Error message about possible problems with the group handling.
	Error string `json:"error,omitempty"`

	// A list of last failures of handled Machines.
	LastMachineFailures []MachineFailure `json:"lastMachineFailures,omitempty"`

	// Status' summary.
	ConditionSummary ConditionSummary `json:"conditionSummary,omitempty"`
}

type MachineFailure struct {
	// Machine's name.
	Name string `json:"name,omitempty"`

	// Machine's ProviderID.
	ProviderID string `json:"providerID,omitempty"`

	// Machine owner's name.
	OwnerRef string `json:"ownerRef,omitempty"`

	// Last operation with machine.
	LastOperation MachineOperation `json:"lastOperation,omitempty"`
}

type MachineOperation struct {
	// Last operation's description.
	Description string `json:"description,omitempty"`

	// Timestamp of last status update for operation.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`

	// Machine's operation state.
	State string `json:"state,omitempty"`

	// Type of operation.
	Type string `json:"type,omitempty"`
}

type ConditionSummary struct {
	// Status message about group handling.
	StatusMessage string `json:"statusMessage,omitempty"`

	// Summary for the NodeGroup status: True or False
	Ready string `json:"ready,omitempty"`
}
