/*
Copyright 2021 Flant JSC

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
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	nm "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/pkg/schema"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeType type of node
type NodeType string

const (
	NodeTypeStatic         NodeType = "Static"
	NodeTypeCloudEphemeral NodeType = "CloudEphemeral"
	NodeTypeCloudPermanent NodeType = "CloudPermanent"
	NodeTypeCloudStatic    NodeType = "CloudStatic"
)

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

	// CRI parameters. Optional.
	CRI CRI `json:"cri,omitempty"`

	// staticInstances. Optional.
	StaticInstances *StaticInstances `json:"staticInstances,omitempty"`

	// cloudInstances. Optional.
	CloudInstances CloudInstances `json:"cloudInstances,omitempty"`

	// Default labels, annotations and taints for Nodes in NodeGroup. Optional.
	NodeTemplate nm.NodeTemplate `json:"nodeTemplate,omitempty"`

	// Chaos monkey settings. Optional.
	Chaos Chaos `json:"chaos,omitempty"`

	// OperatingSystem. Optional.
	OperatingSystem OperatingSystem `json:"operatingSystem,omitempty"`

	// Disruptions settings for nodes. Optional.
	Disruptions Disruptions `json:"disruptions,omitempty"`

	// Update settings for NodeGroups. Optional
	Update Update `json:"update,omitempty"`

	// Kubelet settings for nodes. Optional.
	Kubelet Kubelet `json:"kubelet,omitempty"`

	// Fencing settings for nodes. Optional.
	Fencing Fencing `json:"fencing,omitempty"`
}

type CRI struct {
	// Container runtime type. Docker, Containerd or NotManaged
	Type string `json:"type,omitempty"`

	// Containerd runtime parameters.
	Containerd *Containerd `json:"containerd,omitempty"`

	// Docker settings for nodes.
	Docker *Docker `json:"docker,omitempty"`

	// NotManaged settings for nodes.
	NotManaged *NotManaged `json:"notManaged,omitempty"`
}

func (c CRI) IsEmpty() bool {
	return c.Type == "" && c.Containerd == nil && c.Docker == nil
}

type Containerd struct {
	// Set the max concurrent downloads for each pull (default 3).
	MaxConcurrentDownloads *int32 `json:"maxConcurrentDownloads,omitempty"`
}

type Docker struct {
	// Set the max concurrent downloads for each pull (default 3).
	MaxConcurrentDownloads *int32 `json:"maxConcurrentDownloads,omitempty"`

	// Enable docker maintenance from bashible.
	Manage *bool `json:"manage,omitempty"`
}

type NotManaged struct {
	// Set custom path to CRI socket
	CriSocketPath *string `json:"criSocketPath,omitempty"`
}

// StaticInstances is an extra parameters for NodeGroup with type Static.
type StaticInstances struct {
	// Label selector for StaticInstance resources. Optional.
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// Minimal amount of instances for the group. Required.
	Count int32 `json:"count"`
}

type InfrastructureTemplateReference struct {
	// Kind of a InfrastructureTemplateReference resource: StaticMachineTemplate
	Kind string `json:"kind,omitempty"`

	// Name of a InfrastructureTemplateReference resource.
	Name string `json:"name,omitempty"`
}

func (i InfrastructureTemplateReference) IsEmpty() bool {
	return i.Kind == "" && i.Name == ""
}

// CloudInstances is an extra parameters for NodeGroup with type Cloud.
type CloudInstances struct {
	// Quick shutdown results in faster drain. Optional
	QuickShutdown *bool `json:"quickShutdown,omitempty"`

	// List of availability zones to create instances in.
	Zones []string `json:"zones"`

	// Minimal amount of instances for the group in each zone. Required.
	MinPerZone *int32 `json:"minPerZone,omitempty"`

	// Maximum amount of instances for the group in each zone. Required.
	MaxPerZone *int32 `json:"maxPerZone,omitempty"`

	// Maximum amount of unavailable instances (during rollout) in the group in each zone.
	MaxUnavailablePerZone *int32 `json:"maxUnavailablePerZone,omitempty"`

	// Maximum amount of instances to rollout simultaneously in the group in each zone.
	MaxSurgePerZone *int32 `json:"maxSurgePerZone,omitempty"`

	// Overprovisioned Nodes for this NodeGroup.
	Standby *intstr.IntOrString `json:"standby,omitempty"`

	// Settings for overprovisioned Node holder.
	StandbyHolder StandbyHolder `json:"standbyHolder,omitempty"`

	// Reference to a ClassInstance resource. Required.
	ClassReference ClassReference `json:"classReference"`

	// Priority setting for autoscaler expander
	Priority *int32 `json:"priority,omitempty"`
}

func (c CloudInstances) IsEmpty() bool {
	return c.Zones == nil &&
		c.MinPerZone == nil &&
		c.MaxPerZone == nil &&
		c.MaxUnavailablePerZone == nil &&
		c.MaxSurgePerZone == nil &&
		c.Standby == nil &&
		c.StandbyHolder.IsEmpty() &&
		c.ClassReference.IsEmpty()
}

type StandbyHolder struct {
	// Percent of the node-group's node capacity which will be overprovisioned with standby-holder pod.
	OverprovisioningRate *int64 `json:"overprovisioningRate,omitempty"`
	// Deprecated: Describes the amount of resources, that will not be held by standby holder.
	NotHeldResources Resources `json:"notHeldResources,omitempty"`
}

func (s StandbyHolder) IsEmpty() bool {
	return s.OverprovisioningRate == nil
}

type Resources struct {
	// Describes the amount of CPU that will not be held by standby holder on Nodes from this NodeGroup.
	CPU intstr.IntOrString `json:"cpu,omitempty"`

	// Describes the amount of memory that will not be held by standby holder on Nodes from this NodeGroup.
	Memory intstr.IntOrString `json:"memory,omitempty"`
}

func (r Resources) IsEmpty() bool {
	v := r.CPU.String() + r.Memory.String()
	return v == "" || v == "00"
}

type ClassReference struct {
	// Kind of a ClassReference resource: OpenStackInstanceClass, GCPInstanceClass, ...
	Kind string `json:"kind,omitempty"`

	// Name of a ClassReference resource.
	Name string `json:"name,omitempty"`
}

func (c ClassReference) IsEmpty() bool {
	return c.Kind == "" && c.Name == ""
}

// Chaos is a chaos-monkey settings.
type Chaos struct {
	// Chaos monkey mode: DrainAndDelete or Disabled (default).
	Mode string `json:"mode,omitempty"`

	// Chaos monkey wake up period. Default is 6h.
	Period string `json:"period,omitempty"`
}

func (c Chaos) IsEmpty() bool {
	return c.Mode == "" && c.Period == ""
}

type OperatingSystem struct {
	// Enable kernel maintenance from bashible (default true).
	// Deprecated
	ManageKernel *bool `json:"manageKernel,omitempty"`
}

func (o OperatingSystem) IsEmpty() bool {
	return o.ManageKernel == nil
}

type Disruptions struct {
	// Allow disruptive update mode: Manual or Automatic.
	ApprovalMode string `json:"approvalMode"`

	// Extra settings for Automatic mode.
	Automatic AutomaticDisruptions `json:"automatic,omitempty"`
	// Extra settings for RolloutRestart mode.
	RollingUpdate RollingUpdateDisruptions `json:"rollingUpdate,omitempty"`
}

func (d Disruptions) IsEmpty() bool {
	return d.ApprovalMode == "" && d.Automatic.IsEmpty()
}

type Update struct {
	MaxConcurrent *intstr.IntOrString `json:"maxConcurrent,omitempty"`
}

type AutomaticDisruptions struct {
	// Indicates if Pods should be drained from node before allow disruption.
	DrainBeforeApproval *bool `json:"drainBeforeApproval,omitempty"`
	// Node update windows
	Windows update.Windows `json:"windows,omitempty"`
}

type RollingUpdateDisruptions struct {
	// Node update windows
	Windows update.Windows `json:"windows,omitempty"`
}

func (a AutomaticDisruptions) IsEmpty() bool {
	return a.DrainBeforeApproval == nil && len(a.Windows) == 0
}

func (r RollingUpdateDisruptions) IsEmpty() bool {
	return len(r.Windows) == 0
}

type Kubelet struct {
	// Set the max count of pods per node. Default: 110
	MaxPods *int32 `json:"maxPods,omitempty"`

	// Directory path for managing kubelet files (volume mounts,etc).
	// Default: '/var/lib/kubelet'
	RootDir string `json:"rootDir,omitempty"`

	// Maximum log file size before it is rotated.
	// Default: '50Mi'
	ContainerLogMaxSize string `json:"containerLogMaxSize,omitempty"`

	// How many rotated log files to store before deleting them.
	// Default: '4'
	ContainerLogMaxFiles int `json:"containerLogMaxFiles,omitempty"`

	ResourceReservation KubeletResourceReservation `json:"resourceReservation"`
}

type KubeletResourceReservation struct {
	Mode KubeletResourceReservationMode `json:"mode"`

	Static *KubeletStaticResourceReservation `json:"static,omitempty"`
}

type KubeletStaticResourceReservation struct {
	CPU              resource.Quantity `json:"cpu,omitempty"`
	Memory           resource.Quantity `json:"memory,omitempty"`
	EphemeralStorage resource.Quantity `json:"ephemeralStorage,omitempty"`
}

type KubeletResourceReservationMode string

const (
	KubeletResourceReservationModeOff    KubeletResourceReservationMode = "Off"
	KubeletResourceReservationModeAuto   KubeletResourceReservationMode = "Auto"
	KubeletResourceReservationModeStatic KubeletResourceReservationMode = "Static"
)

func (k Kubelet) IsEmpty() bool {
	return k.MaxPods == nil && k.RootDir == "" && k.ContainerLogMaxSize == "" && k.ContainerLogMaxFiles == 0 &&
		k.ResourceReservation.Mode == "" && k.ResourceReservation.Static == nil
}

type Fencing struct {
	// Set custom settings for fencing controller
	Mode string `json:"mode,omitempty"`
}

func (f Fencing) IsEmpty() bool {
	return f.Mode == ""
}

type NodeGroupConditionType string

const (
	NodeGroupConditionTypeReady                        = "Ready"
	NodeGroupConditionTypeUpdating                     = "Updating"
	NodeGroupConditionTypeWaitingForDisruptiveApproval = "WaitingForDisruptiveApproval"
	NodeGroupConditionTypeScaling                      = "Scaling"
	NodeGroupConditionTypeError                        = "Error"
)

type ConditionStatus string

const (
	ConditionTrue  ConditionStatus = "True"
	ConditionFalse ConditionStatus = "False"
)

type NodeGroupCondition struct {
	// Type is the type of the condition.
	Type NodeGroupConditionType `json:"type"`
	// Status is the status of the condition.
	// Can be True, False
	Status ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Human-readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty"`
}

func (c *NodeGroupCondition) ToMap() map[string]interface{} {
	res := map[string]interface{}{
		"type":   c.Type,
		"status": c.Status,
	}

	if c.Message != "" {
		res["message"] = c.Message
	}

	if !c.LastTransitionTime.IsZero() {
		res["lastTransitionTime"] = c.LastTransitionTime.Format(time.RFC3339)
	}

	return res
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

	// The current version of kubernetes on the nodes, or the version to which the nodes will be upgraded.
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// Current nodegroup conditions
	Conditions []NodeGroupCondition `json:"conditions,omitempty"`
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

type nodeGroupKind struct{}

func (in *NodeGroupStatus) GetObjectKind() schema.ObjectKind {
	return &nodeGroupKind{}
}

func (f *nodeGroupKind) SetGroupVersionKind(_ schema.GroupVersionKind) {}
func (f *nodeGroupKind) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"}
}
