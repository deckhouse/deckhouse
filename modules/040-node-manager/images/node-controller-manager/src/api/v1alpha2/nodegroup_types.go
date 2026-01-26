package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NodeType defines the type of nodes in the group (v1alpha2 uses Cloud/Static/Hybrid like v1alpha1)
// +kubebuilder:validation:Enum=Cloud;Static;Hybrid
type NodeType string

const (
	NodeTypeCloud  NodeType = "Cloud"
	NodeTypeStatic NodeType = "Static"
	NodeTypeHybrid NodeType = "Hybrid"
)

// DisruptionApprovalMode defines the approval mode
// +kubebuilder:validation:Enum=Manual;Automatic;RollingUpdate
type DisruptionApprovalMode string

const (
	DisruptionApprovalModeManual        DisruptionApprovalMode = "Manual"
	DisruptionApprovalModeAutomatic     DisruptionApprovalMode = "Automatic"
	DisruptionApprovalModeRollingUpdate DisruptionApprovalMode = "RollingUpdate"
)

// CRIType defines the container runtime type
// +kubebuilder:validation:Enum=Docker;Containerd;NotManaged
type CRIType string

const (
	CRITypeDocker     CRIType = "Docker"
	CRITypeContainerd CRIType = "Containerd"
	CRITypeNotManaged CRIType = "NotManaged"
)

// ChaosMode defines the chaos monkey mode
// +kubebuilder:validation:Enum=Disabled;DrainAndDelete
type ChaosMode string

const (
	ChaosModeDisabled       ChaosMode = "Disabled"
	ChaosModeDrainAndDelete ChaosMode = "DrainAndDelete"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=ng
// +kubebuilder:subresource:status

// NodeGroup defines a group of nodes (v1alpha2 version)
type NodeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeGroupSpec   `json:"spec,omitempty"`
	Status NodeGroupStatus `json:"status,omitempty"`
}

// NodeGroupSpec defines the desired state of NodeGroup (v1alpha2)
type NodeGroupSpec struct {
	// NodeType defines the type of nodes (Cloud, Static, Hybrid)
	// +kubebuilder:validation:Required
	NodeType NodeType `json:"nodeType"`

	// CRI defines container runtime parameters
	// +optional
	CRI *CRISpec `json:"cri,omitempty"`

	// CloudInstances defines parameters for cloud-based VMs
	// +optional
	CloudInstances *CloudInstancesSpec `json:"cloudInstances,omitempty"`

	// NodeTemplate defines fields that will be maintained in all nodes
	// +optional
	NodeTemplate *NodeTemplate `json:"nodeTemplate,omitempty"`

	// Chaos defines chaos monkey settings
	// +optional
	Chaos *ChaosSpec `json:"chaos,omitempty"`

	// OperatingSystem defines OS settings for nodes
	// +optional
	OperatingSystem *OperatingSystemSpec `json:"operatingSystem,omitempty"`

	// Disruptions defines disruption settings for nodes
	// +optional
	Disruptions *DisruptionsSpec `json:"disruptions,omitempty"`

	// Kubelet defines kubelet settings for nodes
	// +optional
	Kubelet *KubeletSpec `json:"kubelet,omitempty"`
}

// CRISpec defines container runtime parameters (v1alpha2 has oneOf for types)
type CRISpec struct {
	// Type defines the container runtime type
	// +optional
	Type CRIType `json:"type,omitempty"`

	// Containerd defines containerd parameters
	// +optional
	Containerd *ContainerdSpec `json:"containerd,omitempty"`

	// Docker defines docker parameters
	// +optional
	Docker *DockerSpec `json:"docker,omitempty"`

	// NotManaged defines settings for not managed CRI
	// +optional
	NotManaged *NotManagedCRISpec `json:"notManaged,omitempty"`
}

// ContainerdSpec defines containerd parameters
type ContainerdSpec struct {
	// MaxConcurrentDownloads sets the max concurrent downloads
	// +optional
	MaxConcurrentDownloads *int `json:"maxConcurrentDownloads,omitempty"`
}

// DockerSpec defines docker parameters
type DockerSpec struct {
	// MaxConcurrentDownloads sets the max concurrent downloads
	// +optional
	MaxConcurrentDownloads *int `json:"maxConcurrentDownloads,omitempty"`

	// Manage enables Docker maintenance
	// +optional
	Manage *bool `json:"manage,omitempty"`
}

// NotManagedCRISpec defines settings for not managed CRI
type NotManagedCRISpec struct {
	// CRISocketPath is the path to CRI socket
	// +optional
	CRISocketPath string `json:"criSocketPath,omitempty"`
}

// CloudInstancesSpec defines parameters for cloud-based VMs
type CloudInstancesSpec struct {
	// Zones is the list of availability zones
	// +optional
	Zones []string `json:"zones,omitempty"`

	// MinPerZone is the minimum number of instances per zone
	// +kubebuilder:validation:Minimum=0
	MinPerZone int32 `json:"minPerZone"`

	// MaxPerZone is the maximum number of instances per zone
	// +kubebuilder:validation:Minimum=0
	MaxPerZone int32 `json:"maxPerZone"`

	// MaxUnavailablePerZone is the max unavailable instances per zone
	// +optional
	MaxUnavailablePerZone *int32 `json:"maxUnavailablePerZone,omitempty"`

	// MaxSurgePerZone is the max surge instances per zone
	// +optional
	MaxSurgePerZone *int32 `json:"maxSurgePerZone,omitempty"`

	// Standby is the number of overprovisioned nodes
	// +optional
	Standby *intstr.IntOrString `json:"standby,omitempty"`

	// StandbyHolder defines reserved resources
	// +optional
	StandbyHolder *StandbyHolderSpec `json:"standbyHolder,omitempty"`

	// ClassReference is the reference to InstanceClass
	// +kubebuilder:validation:Required
	ClassReference ClassReference `json:"classReference"`
}

// ClassReference defines the reference to InstanceClass
type ClassReference struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// StandbyHolderSpec defines standby holder parameters
type StandbyHolderSpec struct {
	// NotHeldResources describes resources not held
	// +optional
	NotHeldResources *NotHeldResourcesSpec `json:"notHeldResources,omitempty"`
}

// NotHeldResourcesSpec describes resources not held by standby holder
type NotHeldResourcesSpec struct {
	// CPU describes the amount of CPU not held
	// +optional
	CPU *intstr.IntOrString `json:"cpu,omitempty"`

	// Memory describes the amount of memory not held
	// +optional
	Memory *intstr.IntOrString `json:"memory,omitempty"`
}

// NodeTemplate defines fields maintained in all nodes
type NodeTemplate struct {
	// Labels to be added to nodes
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations to be added to nodes
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Taints to be added to nodes
	// +optional
	Taints []corev1.Taint `json:"taints,omitempty"`
}

// ChaosSpec defines chaos monkey settings
type ChaosSpec struct {
	// Mode is the chaos monkey mode
	// +optional
	Mode ChaosMode `json:"mode,omitempty"`

	// Period is the time interval
	// +optional
	Period string `json:"period,omitempty"`
}

// OperatingSystemSpec defines OS settings
type OperatingSystemSpec struct {
	// ManageKernel enables kernel maintenance
	// +optional
	ManageKernel *bool `json:"manageKernel,omitempty"`
}

// DisruptionsSpec defines disruption settings
type DisruptionsSpec struct {
	// ApprovalMode is the approval mode
	// +optional
	ApprovalMode DisruptionApprovalMode `json:"approvalMode,omitempty"`

	// Automatic defines automatic mode parameters
	// +optional
	Automatic *AutomaticDisruptionSpec `json:"automatic,omitempty"`

	// RollingUpdate defines rolling update parameters
	// +optional
	RollingUpdate *RollingUpdateDisruptionSpec `json:"rollingUpdate,omitempty"`
}

// AutomaticDisruptionSpec defines automatic disruption parameters
type AutomaticDisruptionSpec struct {
	// DrainBeforeApproval drains pods before approving
	// +optional
	DrainBeforeApproval *bool `json:"drainBeforeApproval,omitempty"`

	// Windows defines time windows
	// +optional
	Windows []DisruptionWindow `json:"windows,omitempty"`
}

// RollingUpdateDisruptionSpec defines rolling update parameters
type RollingUpdateDisruptionSpec struct {
	// Windows defines time windows
	// +optional
	Windows []DisruptionWindow `json:"windows,omitempty"`
}

// DisruptionWindow defines a time window
type DisruptionWindow struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Days []string `json:"days,omitempty"`
}

// KubeletSpec defines kubelet settings
type KubeletSpec struct {
	// MaxPods sets the max count of pods per node
	// +optional
	MaxPods *int32 `json:"maxPods,omitempty"`

	// RootDir is the kubelet root directory
	// +optional
	RootDir string `json:"rootDir,omitempty"`

	// ContainerLogMaxSize is the maximum log file size
	// +optional
	ContainerLogMaxSize string `json:"containerLogMaxSize,omitempty"`

	// ContainerLogMaxFiles is the number of rotated log files
	// +optional
	ContainerLogMaxFiles *int32 `json:"containerLogMaxFiles,omitempty"`
}

// NodeGroupStatus defines the observed state of NodeGroup
type NodeGroupStatus struct {
	// Ready is the number of ready nodes
	// +optional
	Ready int32 `json:"ready,omitempty"`

	// Nodes is the total number of nodes
	// +optional
	Nodes int32 `json:"nodes,omitempty"`

	// Instances is the number of instances
	// +optional
	Instances int32 `json:"instances,omitempty"`

	// Desired is the number of desired machines
	// +optional
	Desired int32 `json:"desired,omitempty"`

	// Min is the minimal amount of instances
	// +optional
	Min int32 `json:"min,omitempty"`

	// Max is the maximum amount of instances
	// +optional
	Max int32 `json:"max,omitempty"`

	// UpToDate is the number of up-to-date nodes
	// +optional
	UpToDate int32 `json:"upToDate,omitempty"`

	// Standby is the number of overprovisioned instances
	// +optional
	Standby int32 `json:"standby,omitempty"`

	// Error is the error message
	// +optional
	Error string `json:"error,omitempty"`

	// KubernetesVersion is the current kubernetes version
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// LastMachineFailures contains last machine failures
	// +optional
	LastMachineFailures []MachineFailure `json:"lastMachineFailures,omitempty"`

	// ConditionSummary contains condition summary
	// +optional
	ConditionSummary *ConditionSummary `json:"conditionSummary,omitempty"`
}

// MachineFailure describes a machine failure
type MachineFailure struct {
	Name          string                `json:"name,omitempty"`
	ProviderID    string                `json:"providerID,omitempty"`
	OwnerRef      string                `json:"ownerRef,omitempty"`
	LastOperation *MachineLastOperation `json:"lastOperation,omitempty"`
}

// MachineLastOperation describes the last machine operation
type MachineLastOperation struct {
	Description    string `json:"description,omitempty"`
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	State          string `json:"state,omitempty"`
	Type           string `json:"type,omitempty"`
}

// ConditionSummary contains condition summary
type ConditionSummary struct {
	StatusMessage string `json:"statusMessage,omitempty"`
	Ready         string `json:"ready,omitempty"`
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
