package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeGroupSpec defines the desired state of NodeGroup
type NodeGroupSpec struct {
	// NodeType defines the type of nodes in the group
	// +kubebuilder:validation:Enum=Static;Cloud
	NodeType string `json:"nodeType,omitempty"`

	// CloudInstances defines cloud instance configuration
	CloudInstances *CloudInstancesSpec `json:"cloudInstances,omitempty"`

	// NodeTemplate defines the template for nodes in the group
	NodeTemplate *NodeTemplateSpec `json:"nodeTemplate,omitempty"`

	// Chaos defines chaos engineering configuration
	Chaos *ChaosSpec `json:"chaos,omitempty"`

	// Disruptions defines disruption configuration
	Disruptions *DisruptionsSpec `json:"disruptions,omitempty"`

	// KubernetesVersion defines the Kubernetes version for nodes
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// CRI defines the container runtime interface
	CRI *CRISpec `json:"cri,omitempty"`

	// StaticInstances defines static instance configuration
	StaticInstances *StaticInstancesSpec `json:"staticInstances,omitempty"`
}

// CloudInstancesSpec defines cloud instance configuration
type CloudInstancesSpec struct {
	// MaxPerZone defines the maximum number of instances per zone
	MaxPerZone int32 `json:"maxPerZone,omitempty"`

	// MinPerZone defines the minimum number of instances per zone
	MinPerZone int32 `json:"minPerZone,omitempty"`

	// Zones defines the availability zones
	Zones []string `json:"zones,omitempty"`

	// InstanceClass defines the instance class
	InstanceClass string `json:"instanceClass,omitempty"`

	// AdditionalLabels defines additional labels for instances
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"`

	// AdditionalTags defines additional tags for instances
	AdditionalTags map[string]string `json:"additionalTags,omitempty"`

	// SecurityGroups defines security groups
	SecurityGroups []string `json:"securityGroups,omitempty"`

	// SubnetID defines the subnet ID
	SubnetID string `json:"subnetId,omitempty"`

	// AssignPublicIPAddress defines whether to assign public IP
	AssignPublicIPAddress bool `json:"assignPublicIPAddress,omitempty"`

	// DiskType defines the disk type
	DiskType string `json:"diskType,omitempty"`

	// DiskSizeGb defines the disk size in GB
	DiskSizeGb int32 `json:"diskSizeGb,omitempty"`

	// ImageID defines the image ID
	ImageID string `json:"imageId,omitempty"`

	// SSHPublicKey defines the SSH public key
	SSHPublicKey string `json:"sshPublicKey,omitempty"`

	// LoadBalancer defines load balancer configuration
	LoadBalancer *LoadBalancerSpec `json:"loadBalancer,omitempty"`
}

// StaticInstancesSpec defines static instance configuration
type StaticInstancesSpec struct {
	// Instances defines the list of static instances
	Instances []StaticInstanceSpec `json:"instances,omitempty"`
}

// StaticInstanceSpec defines a static instance
type StaticInstanceSpec struct {
	// Name defines the instance name
	Name string `json:"name,omitempty"`

	// IPAddress defines the IP address
	IPAddress string `json:"ipAddress,omitempty"`

	// SSHPublicKey defines the SSH public key
	SSHPublicKey string `json:"sshPublicKey,omitempty"`

	// Labels defines instance labels
	Labels map[string]string `json:"labels,omitempty"`
}

// NodeTemplateSpec defines the template for nodes
type NodeTemplateSpec struct {
	// Labels defines node labels
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations defines node annotations
	Annotations map[string]string `json:"annotations,omitempty"`

	// Taints defines node taints
	Taints []TaintSpec `json:"taints,omitempty"`
}

// TaintSpec defines a node taint
type TaintSpec struct {
	// Key defines the taint key
	Key string `json:"key,omitempty"`

	// Value defines the taint value
	Value string `json:"value,omitempty"`

	// Effect defines the taint effect
	Effect string `json:"effect,omitempty"`
}

// ChaosSpec defines chaos engineering configuration
type ChaosSpec struct {
	// Enabled defines whether chaos is enabled
	Enabled bool `json:"enabled,omitempty"`

	// Period defines the chaos period
	Period string `json:"period,omitempty"`

	// Duration defines the chaos duration
	Duration string `json:"duration,omitempty"`
}

// DisruptionsSpec defines disruption configuration
type DisruptionsSpec struct {
	// Enabled defines whether disruptions are enabled
	Enabled bool `json:"enabled,omitempty"`

	// Approve defines whether disruptions are approved
	Approve bool `json:"approve,omitempty"`
}

// CRISpec defines the container runtime interface
type CRISpec struct {
	// Type defines the CRI type
	Type string `json:"type,omitempty"`

	// Containerd defines containerd configuration
	Containerd *ContainerdSpec `json:"containerd,omitempty"`
}

// ContainerdSpec defines containerd configuration
type ContainerdSpec struct {
	// MaxConcurrentDownloads defines max concurrent downloads
	MaxConcurrentDownloads int32 `json:"maxConcurrentDownloads,omitempty"`

	// MaxConcurrentUploads defines max concurrent uploads
	MaxConcurrentUploads int32 `json:"maxConcurrentUploads,omitempty"`
}

// LoadBalancerSpec defines load balancer configuration
type LoadBalancerSpec struct {
	// Type defines the load balancer type
	Type string `json:"type,omitempty"`

	// HealthCheck defines health check configuration
	HealthCheck *HealthCheckSpec `json:"healthCheck,omitempty"`
}

// HealthCheckSpec defines health check configuration
type HealthCheckSpec struct {
	// Protocol defines the health check protocol
	Protocol string `json:"protocol,omitempty"`

	// Port defines the health check port
	Port int32 `json:"port,omitempty"`

	// Path defines the health check path
	Path string `json:"path,omitempty"`
}

// NodeGroupStatus defines the observed state of NodeGroup
type NodeGroupStatus struct {
	// Desired defines the desired number of nodes
	Desired int32 `json:"desired,omitempty"`

	// Ready defines the number of ready nodes
	Ready int32 `json:"ready,omitempty"`

	// Updating defines the number of updating nodes
	Updating int32 `json:"updating,omitempty"`

	// Failed defines the number of failed nodes
	Failed int32 `json:"failed,omitempty"`

	// Instances defines the list of instances
	Instances []InstanceStatus `json:"instances,omitempty"`

	// LastUpdated defines the last update time
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// Error defines the error message
	Error string `json:"error,omitempty"`

	// Conditions defines the conditions
	Conditions []NodeGroupCondition `json:"conditions,omitempty"`
}

// InstanceStatus defines the status of an instance
type InstanceStatus struct {
	// Name defines the instance name
	Name string `json:"name,omitempty"`

	// IPAddress defines the IP address
	IPAddress string `json:"ipAddress,omitempty"`

	// Status defines the instance status
	Status string `json:"status,omitempty"`

	// LastUpdated defines the last update time
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// Error defines the error message
	Error string `json:"error,omitempty"`
}

// NodeGroupCondition defines a condition
type NodeGroupCondition struct {
	// Type defines the condition type
	Type string `json:"type,omitempty"`

	// Status defines the condition status
	Status string `json:"status,omitempty"`

	// LastTransitionTime defines the last transition time
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason defines the reason
	Reason string `json:"reason,omitempty"`

	// Message defines the message
	Message string `json:"message,omitempty"`
}

// NodeGroup defines the NodeGroup resource
type NodeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeGroupSpec   `json:"spec,omitempty"`
	Status NodeGroupStatus `json:"status,omitempty"`
}

// NodeGroupList defines a list of NodeGroup resources
type NodeGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []NodeGroup `json:"items"`
}
