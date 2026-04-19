/*
Copyright 2025 Flant JSC

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

package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NodeType defines the type of nodes in the group (v1alpha2 version)
// +kubebuilder:validation:Enum=Cloud;Static;Hybrid
type NodeType string

const (
	NodeTypeCloud  NodeType = "Cloud"
	NodeTypeStatic NodeType = "Static"
	NodeTypeHybrid NodeType = "Hybrid"
)

// CRIType defines the container runtime type (v1alpha2 version - includes NotManaged)
// +kubebuilder:validation:Enum=Docker;Containerd;NotManaged
type CRIType string

const (
	CRITypeDocker     CRIType = "Docker"
	CRITypeContainerd CRIType = "Containerd"
	CRITypeNotManaged CRIType = "NotManaged"
)

// DisruptionApprovalMode defines how disruptions are approved
// +kubebuilder:validation:Enum=Manual;Automatic;RollingUpdate
type DisruptionApprovalMode string

const (
	DisruptionApprovalModeManual        DisruptionApprovalMode = "Manual"
	DisruptionApprovalModeAutomatic     DisruptionApprovalMode = "Automatic"
	DisruptionApprovalModeRollingUpdate DisruptionApprovalMode = "RollingUpdate"
)

// ChaosMode defines the chaos testing mode
// +kubebuilder:validation:Enum=DrainAndDelete;Disabled
type ChaosMode string

const (
	ChaosModeDisabled       ChaosMode = "Disabled"
	ChaosModeDrainAndDelete ChaosMode = "DrainAndDelete"
)

// NodeGroupSpec defines the desired state of NodeGroup (v1alpha2 version)
type NodeGroupSpec struct {
	// NodeType specifies the type of nodes in this group
	// +kubebuilder:validation:Required
	NodeType NodeType `json:"nodeType"`

	// CRI specifies container runtime settings
	// +optional
	CRI *CRISpec `json:"cri,omitempty"`

	// CloudInstances specifies cloud instance settings
	// +optional
	CloudInstances *CloudInstancesSpec `json:"cloudInstances,omitempty"`

	// NodeTemplate specifies labels, annotations and taints for nodes
	// +optional
	NodeTemplate *NodeTemplate `json:"nodeTemplate,omitempty"`

	// Chaos specifies chaos testing settings
	// +optional
	Chaos *ChaosSpec `json:"chaos,omitempty"`

	// OperatingSystem specifies OS-level settings
	// +optional
	OperatingSystem *OperatingSystemSpec `json:"operatingSystem,omitempty"`

	// Disruptions specifies disruption handling settings
	// +optional
	Disruptions *DisruptionsSpec `json:"disruptions,omitempty"`

	// Kubelet specifies kubelet settings
	// +optional
	Kubelet *KubeletSpec `json:"kubelet,omitempty"`
}

// CRISpec defines container runtime settings (v1alpha2 version - supports NotManaged)
type CRISpec struct {
	// Type specifies the container runtime type
	// +optional
	Type CRIType `json:"type,omitempty"`

	// Containerd specifies containerd settings
	// +optional
	Containerd *ContainerdSpec `json:"containerd,omitempty"`

	// Docker specifies docker settings
	// +optional
	Docker *DockerSpec `json:"docker,omitempty"`

	// NotManaged specifies settings for unmanaged CRI (v1alpha2 addition)
	// +optional
	NotManaged *NotManagedCRISpec `json:"notManaged,omitempty"`
}

// ContainerdSpec defines containerd settings
type ContainerdSpec struct {
	// MaxConcurrentDownloads limits the number of concurrent downloads
	// +optional
	MaxConcurrentDownloads *int `json:"maxConcurrentDownloads,omitempty"`
}

// DockerSpec defines docker settings
type DockerSpec struct {
	// MaxConcurrentDownloads limits the number of concurrent downloads
	// +optional
	MaxConcurrentDownloads *int `json:"maxConcurrentDownloads,omitempty"`

	// Manage specifies whether to manage docker
	// +optional
	Manage *bool `json:"manage,omitempty"`
}

// NotManagedCRISpec defines settings for unmanaged CRI
type NotManagedCRISpec struct {
	// CRISocketPath specifies the path to the CRI socket
	// +optional
	CRISocketPath string `json:"criSocketPath,omitempty"`
}

// CloudInstancesSpec defines cloud instance settings
type CloudInstancesSpec struct {
	// Zones specifies the availability zones
	// +optional
	Zones []string `json:"zones,omitempty"`

	// MinPerZone specifies minimum instances per zone
	// +kubebuilder:validation:Minimum=0
	MinPerZone int32 `json:"minPerZone"`

	// MaxPerZone specifies maximum instances per zone
	// +kubebuilder:validation:Minimum=0
	MaxPerZone int32 `json:"maxPerZone"`

	// MaxUnavailablePerZone specifies maximum unavailable instances per zone
	// +optional
	MaxUnavailablePerZone *int32 `json:"maxUnavailablePerZone,omitempty"`

	// MaxSurgePerZone specifies maximum surge instances per zone
	// +optional
	MaxSurgePerZone *int32 `json:"maxSurgePerZone,omitempty"`

	// Standby specifies the number of standby instances
	// +optional
	Standby *intstr.IntOrString `json:"standby,omitempty"`

	// StandbyHolder specifies standby holder settings
	// +optional
	StandbyHolder *StandbyHolderSpec `json:"standbyHolder,omitempty"`

	// ClassReference specifies the instance class
	ClassReference ClassReference `json:"classReference"`
}

// ClassReference defines a reference to an instance class
type ClassReference struct {
	// Kind specifies the kind of the instance class
	Kind string `json:"kind"`

	// Name specifies the name of the instance class
	Name string `json:"name"`
}

// StandbyHolderSpec defines standby holder settings
type StandbyHolderSpec struct {
	// NotHeldResources specifies resources not held by standby
	// +optional
	NotHeldResources *NotHeldResourcesSpec `json:"notHeldResources,omitempty"`
}

// NotHeldResourcesSpec defines resources not held by standby
type NotHeldResourcesSpec struct {
	// CPU specifies CPU resources
	// +optional
	CPU *intstr.IntOrString `json:"cpu,omitempty"`

	// Memory specifies memory resources
	// +optional
	Memory *intstr.IntOrString `json:"memory,omitempty"`
}

// NodeTemplate defines labels, annotations and taints for nodes
type NodeTemplate struct {
	// Labels specifies node labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations specifies node annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Taints specifies node taints
	// +optional
	Taints []corev1.Taint `json:"taints,omitempty"`
}

// ChaosSpec defines chaos testing settings
type ChaosSpec struct {
	// Mode specifies the chaos mode
	// +optional
	Mode ChaosMode `json:"mode,omitempty"`

	// Period specifies the chaos period
	// +optional
	Period string `json:"period,omitempty"`
}

// OperatingSystemSpec defines OS-level settings
type OperatingSystemSpec struct {
	// ManageKernel specifies whether to manage kernel
	// +optional
	ManageKernel *bool `json:"manageKernel,omitempty"`
}

// DisruptionsSpec defines disruption handling settings
type DisruptionsSpec struct {
	// ApprovalMode specifies how disruptions are approved
	// +optional
	ApprovalMode DisruptionApprovalMode `json:"approvalMode,omitempty"`

	// Automatic specifies automatic disruption settings
	// +optional
	Automatic *AutomaticDisruptionSpec `json:"automatic,omitempty"`

	// RollingUpdate specifies rolling update settings
	// +optional
	RollingUpdate *RollingUpdateDisruptionSpec `json:"rollingUpdate,omitempty"`
}

// AutomaticDisruptionSpec defines automatic disruption settings
type AutomaticDisruptionSpec struct {
	// DrainBeforeApproval specifies whether to drain before approval
	// +optional
	DrainBeforeApproval *bool `json:"drainBeforeApproval,omitempty"`

	// Windows specifies maintenance windows
	// +optional
	Windows []DisruptionWindow `json:"windows,omitempty"`
}

// RollingUpdateDisruptionSpec defines rolling update settings
type RollingUpdateDisruptionSpec struct {
	// Windows specifies maintenance windows
	// +optional
	Windows []DisruptionWindow `json:"windows,omitempty"`
}

// DisruptionWindow defines a maintenance window
type DisruptionWindow struct {
	// From specifies the start time
	From string `json:"from"`

	// To specifies the end time
	To string `json:"to"`

	// Days specifies the days of the week
	// +optional
	Days []string `json:"days,omitempty"`
}

// KubeletSpec defines kubelet settings
type KubeletSpec struct {
	// MaxPods specifies maximum pods per node
	// +optional
	MaxPods *int32 `json:"maxPods,omitempty"`

	// RootDir specifies the kubelet root directory
	// +optional
	RootDir string `json:"rootDir,omitempty"`

	// ContainerLogMaxSize specifies max log size
	// +optional
	ContainerLogMaxSize string `json:"containerLogMaxSize,omitempty"`

	// ContainerLogMaxFiles specifies max log files
	// +optional
	ContainerLogMaxFiles *int32 `json:"containerLogMaxFiles,omitempty"`
}

// NodeGroupStatus defines the observed state of NodeGroup
type NodeGroupStatus struct {
	// Ready specifies the number of ready nodes
	Ready int32 `json:"ready,omitempty"`

	// Nodes specifies the total number of nodes
	Nodes int32 `json:"nodes,omitempty"`

	// Instances specifies the number of instances
	// +optional
	Instances int32 `json:"instances,omitempty"`

	// Desired specifies the desired number of nodes
	// +optional
	Desired int32 `json:"desired,omitempty"`

	// Min specifies the minimum number of nodes
	// +optional
	Min int32 `json:"min,omitempty"`

	// Max specifies the maximum number of nodes
	// +optional
	Max int32 `json:"max,omitempty"`

	// UpToDate specifies the number of up-to-date nodes
	// +optional
	UpToDate int32 `json:"upToDate,omitempty"`

	// Standby specifies the number of standby nodes
	// +optional
	Standby int32 `json:"standby,omitempty"`

	// Error contains error message if any
	// +optional
	Error string `json:"error,omitempty"`

	// LastMachineFailures contains recent machine failures
	// +optional
	LastMachineFailures []MachineFailure `json:"lastMachineFailures,omitempty"`

	// ConditionSummary contains a summary of conditions
	// +optional
	ConditionSummary *ConditionSummary `json:"conditionSummary,omitempty"`

	// KubernetesVersion specifies the kubernetes version
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
}

// MachineFailure describes a machine failure
type MachineFailure struct {
	// Name specifies the machine name
	Name string `json:"name,omitempty"`

	// ProviderID specifies the provider ID
	// +optional
	ProviderID string `json:"providerID,omitempty"`

	// OwnerRef specifies the owner reference
	// +optional
	OwnerRef string `json:"ownerRef,omitempty"`

	// LastOperation specifies the last operation
	// +optional
	LastOperation *MachineLastOperation `json:"lastOperation,omitempty"`
}

// MachineLastOperation describes the last operation on a machine
type MachineLastOperation struct {
	// Description describes the operation
	// +optional
	Description string `json:"description,omitempty"`

	// LastUpdateTime specifies when the operation was last updated
	// +optional
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`

	// State specifies the operation state
	// +optional
	State string `json:"state,omitempty"`

	// Type specifies the operation type
	// +optional
	Type string `json:"type,omitempty"`
}

// ConditionSummary contains a summary of conditions
type ConditionSummary struct {
	// StatusMessage contains the status message
	// +optional
	StatusMessage string `json:"statusMessage,omitempty"`

	// Ready specifies if the node group is ready
	// +optional
	Ready string `json:"ready,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// NodeGroup is the Schema for the nodegroups API (v1alpha2 version)
type NodeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeGroupSpec   `json:"spec,omitempty"`
	Status NodeGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NodeGroupList contains a list of NodeGroup
type NodeGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeGroup{}, &NodeGroupList{})
}
