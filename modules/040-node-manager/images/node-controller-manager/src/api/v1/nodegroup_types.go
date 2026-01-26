package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NodeType defines the type of nodes in the group
// +kubebuilder:validation:Enum=CloudEphemeral;CloudPermanent;CloudStatic;Static
type NodeType string

const (
	NodeTypeCloudEphemeral NodeType = "CloudEphemeral"
	NodeTypeCloudPermanent NodeType = "CloudPermanent"
	NodeTypeCloudStatic    NodeType = "CloudStatic"
	NodeTypeStatic         NodeType = "Static"
)

// DisruptionApprovalMode defines the approval mode for disruptive updates
// +kubebuilder:validation:Enum=Manual;Automatic;RollingUpdate
type DisruptionApprovalMode string

const (
	DisruptionApprovalModeManual        DisruptionApprovalMode = "Manual"
	DisruptionApprovalModeAutomatic     DisruptionApprovalMode = "Automatic"
	DisruptionApprovalModeRollingUpdate DisruptionApprovalMode = "RollingUpdate"
)

// CRIType defines the container runtime type
// +kubebuilder:validation:Enum=Docker;Containerd;ContainerdV2;NotManaged
type CRIType string

const (
	CRITypeDocker       CRIType = "Docker"
	CRITypeContainerd   CRIType = "Containerd"
	CRITypeContainerdV2 CRIType = "ContainerdV2"
	CRITypeNotManaged   CRIType = "NotManaged"
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
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.nodeType`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Nodes",type=integer,JSONPath=`.status.nodes`
// +kubebuilder:printcolumn:name="UpToDate",type=integer,JSONPath=`.status.upToDate`
// +kubebuilder:printcolumn:name="Instances",type=integer,JSONPath=`.status.instances`
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desired`
// +kubebuilder:printcolumn:name="Min",type=integer,JSONPath=`.status.min`
// +kubebuilder:printcolumn:name="Max",type=integer,JSONPath=`.status.max`
// +kubebuilder:printcolumn:name="Standby",type=integer,JSONPath=`.status.standby`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditionSummary.statusMessage`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// NodeGroup defines a group of nodes with common configuration
type NodeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeGroupSpec   `json:"spec,omitempty"`
	Status NodeGroupStatus `json:"status,omitempty"`
}

// NodeGroupSpec defines the desired state of NodeGroup
type NodeGroupSpec struct {
	// NodeType defines the type of nodes in the group
	// +kubebuilder:validation:Required
	NodeType NodeType `json:"nodeType"`

	// CRI defines container runtime parameters
	// +optional
	CRI *CRISpec `json:"cri,omitempty"`

	// CloudInstances defines parameters for cloud-based VMs
	// +optional
	CloudInstances *CloudInstancesSpec `json:"cloudInstances,omitempty"`

	// StaticInstances defines parameters for static machines
	// +optional
	StaticInstances *StaticInstancesSpec `json:"staticInstances,omitempty"`

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

	// Update defines update settings
	// +optional
	Update *UpdateSpec `json:"update,omitempty"`

	// Fencing defines fencing controller settings
	// +optional
	Fencing *FencingSpec `json:"fencing,omitempty"`

	// GPU defines GPU parameters
	// +optional
	GPU *GPUSpec `json:"gpu,omitempty"`

	// NodeDrainTimeoutSecond defines maximum duration for draining
	// +optional
	NodeDrainTimeoutSecond *int `json:"nodeDrainTimeoutSecond,omitempty"`
}

// CRISpec defines container runtime parameters
type CRISpec struct {
	// Type defines the container runtime type
	// +optional
	Type CRIType `json:"type,omitempty"`

	// Containerd defines containerd runtime parameters
	// +optional
	Containerd *ContainerdSpec `json:"containerd,omitempty"`

	// ContainerdV2 defines containerdV2 runtime parameters
	// +optional
	ContainerdV2 *ContainerdSpec `json:"containerdV2,omitempty"`

	// Docker defines docker runtime parameters
	// +optional
	Docker *DockerSpec `json:"docker,omitempty"`

	// NotManaged defines settings for not managed CRI
	// +optional
	NotManaged *NotManagedCRISpec `json:"notManaged,omitempty"`
}

// ContainerdSpec defines containerd parameters
type ContainerdSpec struct {
	// MaxConcurrentDownloads sets the max concurrent downloads for each pull
	// +optional
	MaxConcurrentDownloads *int `json:"maxConcurrentDownloads,omitempty"`
}

// DockerSpec defines docker parameters
type DockerSpec struct {
	// MaxConcurrentDownloads sets the max concurrent downloads for each pull
	// +optional
	MaxConcurrentDownloads *int `json:"maxConcurrentDownloads,omitempty"`

	// Manage enables Docker maintenance from bashible
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
	// Zones is the list of availability zones to create instances in
	// +optional
	Zones []string `json:"zones,omitempty"`

	// MinPerZone is the minimum number of instances per zone
	// +kubebuilder:validation:Minimum=0
	MinPerZone int32 `json:"minPerZone"`

	// MaxPerZone is the maximum number of instances per zone
	// +kubebuilder:validation:Minimum=0
	MaxPerZone int32 `json:"maxPerZone"`

	// MaxUnavailablePerZone is the maximum number of unavailable instances per zone
	// +optional
	MaxUnavailablePerZone *int32 `json:"maxUnavailablePerZone,omitempty"`

	// MaxSurgePerZone is the maximum number of instances to rollout simultaneously per zone
	// +optional
	MaxSurgePerZone *int32 `json:"maxSurgePerZone,omitempty"`

	// Standby is the number of overprovisioned nodes
	// +optional
	Standby *intstr.IntOrString `json:"standby,omitempty"`

	// StandbyHolder defines amount of reserved resources
	// +optional
	StandbyHolder *StandbyHolderSpec `json:"standbyHolder,omitempty"`

	// ClassReference is the reference to the InstanceClass object
	// +kubebuilder:validation:Required
	ClassReference ClassReference `json:"classReference"`

	// Priority is the priority of the node group for autoscaler
	// +optional
	Priority *int `json:"priority,omitempty"`

	// QuickShutdown lowers drain timeout (deprecated)
	// +optional
	QuickShutdown *bool `json:"quickShutdown,omitempty"`
}

// ClassReference defines the reference to InstanceClass
type ClassReference struct {
	// Kind is the object type
	Kind string `json:"kind"`

	// Name is the name of the InstanceClass object
	Name string `json:"name"`
}

// StandbyHolderSpec defines standby holder parameters
type StandbyHolderSpec struct {
	// OverprovisioningRate is the percentage of reserved resources
	// +optional
	OverprovisioningRate *int `json:"overprovisioningRate,omitempty"`

	// NotHeldResources describes resources that will not be held (deprecated)
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

// StaticInstancesSpec defines parameters for static machines
type StaticInstancesSpec struct {
	// LabelSelector is the label query over staticInstances
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// Count is the number of staticInstances to bootstrap
	// +optional
	Count *int32 `json:"count,omitempty"`
}

// NodeTemplate defines fields maintained in all nodes of the group
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

	// Period is the time interval for chaos monkey
	// +optional
	Period string `json:"period,omitempty"`
}

// OperatingSystemSpec defines OS settings
type OperatingSystemSpec struct {
	// ManageKernel enables kernel maintenance (deprecated)
	// +optional
	ManageKernel *bool `json:"manageKernel,omitempty"`
}

// DisruptionsSpec defines disruption settings
type DisruptionsSpec struct {
	// ApprovalMode is the approval mode for disruptive updates
	// +optional
	ApprovalMode DisruptionApprovalMode `json:"approvalMode,omitempty"`

	// Automatic defines parameters for Automatic mode
	// +optional
	Automatic *AutomaticDisruptionSpec `json:"automatic,omitempty"`

	// RollingUpdate defines parameters for RollingUpdate mode
	// +optional
	RollingUpdate *RollingUpdateDisruptionSpec `json:"rollingUpdate,omitempty"`
}

// AutomaticDisruptionSpec defines automatic disruption parameters
type AutomaticDisruptionSpec struct {
	// DrainBeforeApproval drains pods before approving disruption
	// +optional
	DrainBeforeApproval *bool `json:"drainBeforeApproval,omitempty"`

	// Windows defines time windows for disruptive updates
	// +optional
	Windows []DisruptionWindow `json:"windows,omitempty"`
}

// RollingUpdateDisruptionSpec defines rolling update disruption parameters
type RollingUpdateDisruptionSpec struct {
	// Windows defines time windows for disruptive updates
	// +optional
	Windows []DisruptionWindow `json:"windows,omitempty"`
}

// DisruptionWindow defines a time window for disruptions
type DisruptionWindow struct {
	// From is the start time of the window (UTC)
	From string `json:"from"`

	// To is the end time of the window (UTC)
	To string `json:"to"`

	// Days defines days of the week
	// +optional
	Days []string `json:"days,omitempty"`
}

// KubeletSpec defines kubelet settings
type KubeletSpec struct {
	// MaxPods sets the max count of pods per node
	// +optional
	MaxPods *int32 `json:"maxPods,omitempty"`

	// RootDir is the directory path for kubelet files
	// +optional
	RootDir string `json:"rootDir,omitempty"`

	// ContainerLogMaxSize is the maximum log file size
	// +optional
	ContainerLogMaxSize string `json:"containerLogMaxSize,omitempty"`

	// ContainerLogMaxFiles is the number of rotated log files to store
	// +optional
	ContainerLogMaxFiles *int32 `json:"containerLogMaxFiles,omitempty"`

	// ResourceReservation defines resource reservation settings
	// +optional
	ResourceReservation *ResourceReservationSpec `json:"resourceReservation,omitempty"`

	// TopologyManager controls topology manager
	// +optional
	TopologyManager *TopologyManagerSpec `json:"topologyManager,omitempty"`

	// MemorySwap defines swap memory configuration
	// +optional
	MemorySwap *MemorySwapSpec `json:"memorySwap,omitempty"`
}

// ResourceReservationSpec defines resource reservation settings
type ResourceReservationSpec struct {
	// Mode is the resource reservation mode
	// +optional
	Mode string `json:"mode,omitempty"`

	// Static defines static reservation parameters
	// +optional
	Static *StaticResourceReservation `json:"static,omitempty"`
}

// StaticResourceReservation defines static resource reservation
type StaticResourceReservation struct {
	// CPU reservation
	// +optional
	CPU *intstr.IntOrString `json:"cpu,omitempty"`

	// Memory reservation
	// +optional
	Memory *intstr.IntOrString `json:"memory,omitempty"`

	// EphemeralStorage reservation
	// +optional
	EphemeralStorage *intstr.IntOrString `json:"ephemeralStorage,omitempty"`
}

// TopologyManagerSpec defines topology manager settings
type TopologyManagerSpec struct {
	// Enabled enables topology management
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Scope defines granularity of resource alignment
	// +optional
	Scope string `json:"scope,omitempty"`

	// Policy is the resource/topology alignment policy
	// +optional
	Policy string `json:"policy,omitempty"`
}

// MemorySwapSpec defines swap memory configuration
type MemorySwapSpec struct {
	// SwapBehavior defines how swap memory is handled
	// +optional
	SwapBehavior string `json:"swapBehavior,omitempty"`

	// Swappiness defines kernel's tendency to use swap
	// +optional
	Swappiness *int `json:"swappiness,omitempty"`

	// LimitedSwap defines limited swap configuration
	// +optional
	LimitedSwap *LimitedSwapSpec `json:"limitedSwap,omitempty"`
}

// LimitedSwapSpec defines limited swap configuration
type LimitedSwapSpec struct {
	// Size is the swap file size
	Size string `json:"size"`
}

// UpdateSpec defines update settings
type UpdateSpec struct {
	// MaxConcurrent is the maximum number of concurrently updating nodes
	// +optional
	MaxConcurrent *intstr.IntOrString `json:"maxConcurrent,omitempty"`
}

// FencingSpec defines fencing controller settings
type FencingSpec struct {
	// Mode is the fencing mode
	Mode string `json:"mode"`
}

// GPUSpec defines GPU parameters
type GPUSpec struct {
	// Sharing is the GPU sharing strategy
	// +optional
	Sharing string `json:"sharing,omitempty"`

	// MIG defines MIG sharing parameters
	// +optional
	MIG *MIGSpec `json:"mig,omitempty"`

	// TimeSlicing defines time slicing parameters
	// +optional
	TimeSlicing *TimeSlicingSpec `json:"timeSlicing,omitempty"`

	// Exclusive defines exclusive mode parameters
	// +optional
	Exclusive *ExclusiveGPUSpec `json:"exclusive,omitempty"`
}

// MIGSpec defines MIG parameters
type MIGSpec struct {
	// PartedConfig is the MIG configuration name
	// +optional
	PartedConfig string `json:"partedConfig,omitempty"`
}

// TimeSlicingSpec defines time slicing parameters
type TimeSlicingSpec struct {
	// PartitionCount is the count of partition per GPU
	// +optional
	PartitionCount *int `json:"partitionCount,omitempty"`
}

// ExclusiveGPUSpec defines exclusive GPU parameters
type ExclusiveGPUSpec struct{}

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

	// Conditions contains node group conditions
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Deckhouse contains deckhouse-specific status
	// +optional
	Deckhouse *DeckhouseStatus `json:"deckhouse,omitempty"`
}

// MachineFailure describes a machine failure
type MachineFailure struct {
	// Name is the machine's name
	// +optional
	Name string `json:"name,omitempty"`

	// ProviderID is the machine's provider ID
	// +optional
	ProviderID string `json:"providerID,omitempty"`

	// OwnerRef is the machine owner's name
	// +optional
	OwnerRef string `json:"ownerRef,omitempty"`

	// LastOperation contains last operation details
	// +optional
	LastOperation *MachineLastOperation `json:"lastOperation,omitempty"`
}

// MachineLastOperation describes the last machine operation
type MachineLastOperation struct {
	// Description is the operation description
	// +optional
	Description string `json:"description,omitempty"`

	// LastUpdateTime is the timestamp of last update
	// +optional
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`

	// State is the machine's operation state
	// +optional
	State string `json:"state,omitempty"`

	// Type is the operation type
	// +optional
	Type string `json:"type,omitempty"`
}

// ConditionSummary contains condition summary
type ConditionSummary struct {
	// StatusMessage is the status message
	// +optional
	StatusMessage string `json:"statusMessage,omitempty"`

	// Ready is the ready status
	// +optional
	Ready string `json:"ready,omitempty"`
}

// DeckhouseStatus contains deckhouse-specific status
type DeckhouseStatus struct {
	// Synced indicates if the resource was successfully applied
	// +optional
	Synced string `json:"synced,omitempty"`

	// Observed contains observation details
	// +optional
	Observed *ObservedStatus `json:"observed,omitempty"`

	// Processed contains processing details
	// +optional
	Processed *ProcessedStatus `json:"processed,omitempty"`
}

// ObservedStatus contains observation details
type ObservedStatus struct {
	// LastTimestamp is when the resource was last observed
	// +optional
	LastTimestamp string `json:"lastTimestamp,omitempty"`

	// CheckSum is the checksum of the observed resource
	// +optional
	CheckSum string `json:"checkSum,omitempty"`
}

// ProcessedStatus contains processing details
type ProcessedStatus struct {
	// LastTimestamp is when the resource was last processed
	// +optional
	LastTimestamp string `json:"lastTimestamp,omitempty"`

	// CheckSum is the checksum of the processed resource
	// +optional
	CheckSum string `json:"checkSum,omitempty"`
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
