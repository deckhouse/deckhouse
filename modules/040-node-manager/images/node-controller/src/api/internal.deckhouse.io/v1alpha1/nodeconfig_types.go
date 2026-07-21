/*
Copyright 2026 Flant JSC

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MaxKubeletPods is a sanity limit for NodeConfig. Kubernetes defaults to 110;
// 500 leaves room for intentionally dense nodes while rejecting values that
// cannot be backed by the finite user-namespace ID ranges on the host.
const MaxKubeletPods = 500

// NodeConfig is the top-level object stored at /config/nodeconfig.yaml and, in
// the cluster, a CRD (internal.deckhouse.io/v1alpha1). It describes the desired
// state of a Deckhouse olcedar node. The on-node loader parses the same type
// from the config file; the CRD is generated from it (see `task gen`).
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=nc
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
type NodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NodeSpec `json:"spec"`

	// Status is reported by the on-node agent after each reconcile pass.
	// +optional
	Status NodeConfigStatus `json:"status,omitempty"`
}

// NodeConfigStatus is the observed state reported by the node agent.
type NodeConfigStatus struct {
	// ObservedGeneration is the latest spec generation the node has processed —
	// seen and decided what to do about. It reaches the newest generation as
	// soon as the node has looked at it, even while that generation is still
	// held for approval and not yet running (see AppliedGeneration).
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// AppliedGeneration is the spec generation the node is actually running. It
	// lags ObservedGeneration while a disruptive config is held. The rollout's
	// "this node has converged" test is AppliedGeneration == metadata.generation,
	// not ObservedGeneration.
	// +optional
	AppliedGeneration int64 `json:"appliedGeneration,omitempty"`
	// Phase summarises the node. Ready: running the published config, healthy.
	// Pending: healthy but not yet running the published config (held for
	// approval). Degraded: a subsystem failed, the config was rejected, or the
	// node rolled back.
	// +optional
	// +kubebuilder:validation:Enum=Ready;Pending;Degraded
	Phase string `json:"phase,omitempty"`
	// Conditions are the node-level reconcile outcomes (ConfigurationApplied,
	// DisruptionRequired) plus the gate subsystems (APIEndpointsReachable,
	// SysctlApplied). Per-extension and per-unit outcomes live in Extensions and
	// Units instead, one entry each rather than a single aggregate condition.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Extensions is the outcome of each configured system extension, one entry
	// per extension, so a reader sees which one failed rather than a single
	// aggregate ExtensionsReady.
	// +optional
	// +listType=map
	// +listMapKey=name
	Extensions []ExtensionStatus `json:"extensions,omitempty"`
	// Units is the outcome of each managed systemd unit (containerd, kubelet and
	// every unit an extension ships), one entry per unit.
	// +optional
	// +listType=map
	// +listMapKey=name
	Units []UnitStatus `json:"units,omitempty"`
	// MaintenanceToken authenticates a push to the node's maintenance endpoint.
	// The on-node agent generates it at startup and republishes it here; an
	// operator reads it from the last status the node published while the API was
	// reachable and presents it when pushing a config to the node's :50000
	// endpoint after the node has lost the API.
	// +optional
	MaintenanceToken string `json:"maintenanceToken,omitempty"`
}

// ExtensionStatus is the reconcile outcome of one system extension.
type ExtensionStatus struct {
	// Name is the extension name, matching spec.extensions[].name.
	Name string `json:"name"`
	// Digest is the image digest the node installed for it.
	// +optional
	Digest string `json:"digest,omitempty"`
	// State is Ready when the extension is installed and merged, Pending while it
	// is being fetched or waiting for the update window, or Failed with the cause
	// in Message.
	// +kubebuilder:validation:Enum=Ready;Pending;Failed
	State string `json:"state"`
	// Message carries the cause when State is Failed.
	// +optional
	Message string `json:"message,omitempty"`
}

// UnitStatus is the reconcile outcome of one managed systemd unit.
type UnitStatus struct {
	// Name is the systemd unit name (e.g. containerd.service).
	Name string `json:"name"`
	// State is Active when the unit is running, Pending when it is queued to be
	// started later this pass, or Failed with the cause in Message.
	// +kubebuilder:validation:Enum=Active;Pending;Failed
	State string `json:"state"`
	// Message carries the cause when State is Failed.
	// +optional
	Message string `json:"message,omitempty"`
}

// NodeConfigList is a list of NodeConfig objects.
//
// +kubebuilder:object:root=true
type NodeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeConfig `json:"items"`
}

// NodeSpec is the desired state of the node.
type NodeSpec struct {
	// NodeName is the Kubernetes node name this config applies to.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	NodeName string `json:"nodeName"`
	// OSImage is the system image the node boots (resolved from the release
	// channel).
	OSImage string `json:"osImage"`
	// Storage selects the target disk for the OS install. The partition layout
	// is fixed (boot/config/data), so only the whole-disk device is needed.
	// +optional
	Storage Storage `json:"storage,omitempty"`
	// Extensions are the signed verity sysext images to merge onto the root.
	// +optional
	// +listType=map
	// +listMapKey=name
	Extensions []Extension `json:"extensions,omitempty"`
	// Kernel holds sysctl settings and kernel modules to load.
	// +optional
	Kernel Kernel `json:"kernel,omitempty"`
	// Network holds hostname, DNS, NTP, interfaces and routes.
	// +optional
	Network Network `json:"network,omitempty"`
	// Kubelet holds kubelet configuration parameters.
	// +optional
	Kubelet Kubelet `json:"kubelet,omitempty"`
	// ContainerRuntime holds containerd configuration.
	// +optional
	ContainerRuntime ContainerRuntime `json:"containerRuntime,omitempty"`
	// APIServerEndpoints is the list of API server URLs the node connects to
	// (via the node-local API proxy).
	// +optional
	// +kubebuilder:validation:items:Pattern=`^(https?://)?(\\[[0-9A-Fa-f:]+\\]|[A-Za-z0-9]([-A-Za-z0-9]*[A-Za-z0-9])?([.][A-Za-z0-9]([-A-Za-z0-9]*[A-Za-z0-9])?)*):(6553[0-5]|655[0-2][0-9]|65[0-4][0-9]{2}|6[0-4][0-9]{3}|[1-5][0-9]{4}|[1-9][0-9]{0,3})/?$`
	APIServerEndpoints []string `json:"apiServerEndpoints,omitempty"`
	// UpdatePolicy controls how and when the node is updated.
	// +optional
	UpdatePolicy UpdatePolicy `json:"updatePolicy,omitempty"`

	// RegistryPackagesProxyAccessTokenB64 is a base64-encoded token used to
	// authenticate against the registry packages proxy.
	// +optional
	// +kubebuilder:validation:Pattern=`^(([A-Za-z0-9+/]{4})*([A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?)?$`
	RegistryPackagesProxyAccessTokenB64 string `json:"registryPackagesProxyAccessTokenB64,omitempty"`
}

// Storage selects the target disk for the OS install. The partition layout is
// fixed (boot ESP + config + data), so only the whole-disk device is needed —
// no per-partition configuration. Consumed by the initramfs disk provisioner at
// install time (see the initramfs repo docs/disk-provisioning-plan.md).
type Storage struct {
	// Device is the whole-disk block device to install onto, e.g. "/dev/sda",
	// "/dev/nvme0n1", or a stable "/dev/disk/by-id/..." path. Ignored when
	// diskSelector is set.
	// +optional
	// +kubebuilder:validation:Pattern=`^/dev/[A-Za-z0-9._/-]+$`
	Device string `json:"device,omitempty"`
	// DiskSelector picks the target disk by attributes instead of a fixed path.
	// It takes priority over device (matching Talos semantics). All specified
	// conditions must match; the first disk that matches is used.
	// +optional
	DiskSelector *DiskSelector `json:"diskSelector,omitempty"`
	// Wipe controls whether an already-provisioned disk is wiped and
	// re-partitioned. Default false: only an unprovisioned (or non-matching)
	// disk is set up, so an existing correct layout is left intact and a reboot
	// never destroys data.
	// +optional
	Wipe bool `json:"wipe,omitempty"`
}

// DiskSelector matches a target disk by attributes (all specified fields are
// AND-ed). String fields with a glob (name/model/serial/wwid/busPath) match
// shell-style patterns. Attributes are read from `lsblk`/`/sys/block`.
type DiskSelector struct {
	// Size matches the disk capacity, optionally with a comparison operator,
	// e.g. ">=100Gi", ">1Ti", "512Gi".
	// +optional
	Size string `json:"size,omitempty"`
	// Type matches the disk kind.
	// +optional
	// +kubebuilder:validation:Enum=SSD;HDD;NVMe;SD
	Type string `json:"type,omitempty"`
	// Rotational matches spinning (true) vs solid-state (false) disks.
	// +optional
	Rotational *bool `json:"rotational,omitempty"`
	// Model matches the device model (glob), e.g. "Samsung*".
	// +optional
	Model string `json:"model,omitempty"`
	// Serial matches the disk serial number (glob).
	// +optional
	Serial string `json:"serial,omitempty"`
	// WWID matches the World Wide Identifier (glob).
	// +optional
	WWID string `json:"wwid,omitempty"`
	// Name matches the kernel device name (glob), e.g. "nvme0n1".
	// +optional
	Name string `json:"name,omitempty"`
	// BusPath matches the hardware bus path (glob).
	// +optional
	BusPath string `json:"busPath,omitempty"`
}

// Extension is a signed verity sysext built from a release channel. It is
// fetched from the registry-packages-proxy by digest. The optional repository
// (a registry host, e.g. "cr.flant.com") selects the proxy's per-registry
// client config; when empty the proxy's default config is used. The optional
// additionalPath (e.g. "deckhouse/sysext/containerd") is forwarded to the
// proxy as the "path" query parameter to locate the artifact within the
// repository.
type Extension struct {
	// Name is the extension name (also the sysext image basename).
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`
	// Digest is the sha256 digest of the sysext image.
	// +kubebuilder:validation:Pattern=`^sha256:[a-f0-9]{64}$`
	Digest string `json:"digest"`
	// Repository optionally selects the proxy's per-registry client config.
	// +optional
	Repository string `json:"repository,omitempty"`
	// AdditionalPath is forwarded to the proxy as the "path" query parameter.
	// +optional
	AdditionalPath string `json:"additionalPath,omitempty"`
	// RequestedBy records who requested the extension (e.g. "node-manager").
	// +optional
	RequestedBy string `json:"requestedBy,omitempty"`
}

// SysctlValue bounds a single sysctl value for predictable CRD CEL cost.
// +kubebuilder:validation:MaxLength=4096
type SysctlValue string

// DeepCopy lets controller-gen copy maps whose values use this scalar type.
func (in SysctlValue) DeepCopy() *SysctlValue {
	out := new(SysctlValue)
	*out = in
	return out
}

// Kernel describes sysctl settings and kernel modules.
type Kernel struct {
	// +optional
	// +kubebuilder:validation:MaxProperties=256
	// +kubebuilder:validation:XValidation:rule="!('vm.overcommit_memory' in self) || self['vm.overcommit_memory'] == '1'",message="vm.overcommit_memory is required to be 1"
	// +kubebuilder:validation:XValidation:rule="self.all(key, key.matches('^[A-Za-z0-9_-]+([.][A-Za-z0-9_-]+)+$'))",message="sysctl keys must use dotted notation"
	// +kubebuilder:validation:XValidation:rule="self.all(key, self[key].trim() != '')",message="sysctl values must not be empty or whitespace"
	Sysctl map[string]SysctlValue `json:"sysctl,omitempty"`
	// +optional
	Modules []KernelModule `json:"modules,omitempty"`
}

// KernelModule is a module to load with optional parameters.
type KernelModule struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +optional
	Params []string `json:"params,omitempty"`
}

// Network describes hostname, DNS, NTP, interfaces and routes.
type Network struct {
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// +optional
	DNS DNS `json:"dns,omitempty"`
	// +optional
	NTP NTP `json:"ntp,omitempty"`
	// +optional
	Interfaces []NetworkInterface `json:"interfaces,omitempty"`
	// +optional
	Routes []Route `json:"routes,omitempty"`
}

// DNS resolver configuration.
type DNS struct {
	// +optional
	Servers []string `json:"servers,omitempty"`
	// +optional
	Search []string `json:"search,omitempty"`
}

// NTP time-sync configuration.
type NTP struct {
	// +optional
	Servers []string `json:"servers,omitempty"`
}

// NetworkInterface describes a single NIC.
type NetworkInterface struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// DHCP enables DHCPv4 on the interface.
	DHCP bool `json:"dhcp"`
	// Addresses are static CIDR addresses (used when DHCP is false).
	// +optional
	Addresses []string `json:"addresses,omitempty"`
	// +optional
	Gateway string `json:"gateway,omitempty"`
}

// NodeLabelValue mirrors Kubernetes label-value validation in the CRD.
// +kubebuilder:validation:MaxLength=63
// +kubebuilder:validation:Pattern=`^([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?)?$`
type NodeLabelValue string

// DeepCopy lets controller-gen copy maps whose values use this scalar type.
func (in NodeLabelValue) DeepCopy() *NodeLabelValue {
	out := new(NodeLabelValue)
	*out = in
	return out
}

// Route is a static route.
type Route struct {
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	Networks []string `json:"networks,omitempty"`
	// +optional
	Gateway string `json:"gateway,omitempty"`
}

// Kubelet configuration parameters.
type Kubelet struct {
	// ClusterDomain is the DNS domain for this cluster (e.g. "cluster.local").
	// +optional
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	ClusterDomain string `json:"clusterDomain,omitempty"`
	// ClusterDNS is the list of DNS server IP addresses for the cluster.
	// +optional
	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:validation:items:MaxLength=45
	// +kubebuilder:validation:XValidation:rule="self.all(address, isIP(address))",message="clusterDNS entries must be valid IP addresses"
	ClusterDNS []string `json:"clusterDNS,omitempty"`
	// MaxPods is the maximum number of pods per node.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=500
	// +kubebuilder:default=110
	MaxPods int `json:"maxPods,omitempty"`
	// ContainerLogMaxSize is the maximum log file size before rotation
	// (e.g. "50Mi").
	// +optional
	// +kubebuilder:default="50Mi"
	// +kubebuilder:validation:XValidation:rule="isQuantity(self) && sign(quantity(self)) > 0",message="containerLogMaxSize must be a positive Kubernetes quantity"
	ContainerLogMaxSize string `json:"containerLogMaxSize,omitempty"`
	// ContainerLogMaxFiles is the number of rotated log files to retain.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=4
	ContainerLogMaxFiles int `json:"containerLogMaxFiles,omitempty"`
	// CACert is the base64-encoded cluster CA certificate used in the
	// bootstrap-kubelet.conf to verify the API server.
	// +optional
	CACert string `json:"caCert,omitempty"`
	// BootstrapToken is the bootstrap token used by kubelet to obtain its
	// client certificate on first boot.
	// +optional
	BootstrapToken string `json:"bootstrapToken,omitempty"`
	// RegisterWithTaints is a list of taints to add to the node object when
	// kubelet registers itself. Only takes effect on initial registration.
	// +optional
	RegisterWithTaints []Taint `json:"registerWithTaints,omitempty"`
	// ExternalCloudProvider enables --cloud-provider=external so the
	// cloud-controller-manager manages the node (zone, region, providerID).
	// +optional
	ExternalCloudProvider bool `json:"externalCloudProvider,omitempty"`
	// NodeLabels are labels added to the node object when kubelet registers.
	// +optional
	// +kubebuilder:validation:MaxProperties=64
	// +kubebuilder:validation:XValidation:rule="self.all(key, size(key) <= 317 && size(self[key]) <= 63)",message="nodeLabels keys and values are too long"
	// +kubebuilder:validation:XValidation:rule="self.all(key, !format.qualifiedName().validate(key).hasValue())",message="nodeLabels keys must be qualified names"
	NodeLabels map[string]NodeLabelValue `json:"nodeLabels,omitempty"`
}

// Taint represents a Kubernetes taint applied to a node during registration.
type Taint struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=317
	// +kubebuilder:validation:Pattern=`^(([a-z0-9]([-a-z0-9]*[a-z0-9])?\.)*[a-z0-9]([-a-z0-9]*[a-z0-9])?/)?[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$`
	Key string `json:"key"`
	// +optional
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?)?$`
	Value string `json:"value,omitempty"`
	// +kubebuilder:validation:Enum=NoSchedule;PreferNoSchedule;NoExecute
	Effect string `json:"effect"`
}

// ContainerRuntime configuration for the containerd runtime. nodelet renders
// these into /run/etc/containerd/config.toml before starting containerd.
type ContainerRuntime struct {
	// SandboxImage is the pause image used for pod sandboxes.
	// +optional
	// +kubebuilder:default="registry.k8s.io/pause:3.10"
	// +kubebuilder:validation:Pattern=`^[^[:space:]]+$`
	SandboxImage string `json:"sandboxImage,omitempty"`
	// MaxConcurrentDownloads limits parallel image layer downloads.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	MaxConcurrentDownloads int `json:"maxConcurrentDownloads,omitempty"`
	// RegistryMirrors lists mirror endpoints per registry host (not yet rendered
	// into hosts.toml; reserved for future use).
	// +optional
	RegistryMirrors map[string]RegistryMirror `json:"registryMirrors,omitempty"`
}

// RegistryMirror lists mirror endpoints for a registry.
type RegistryMirror struct {
	// +optional
	Endpoints []string `json:"endpoints,omitempty"`
}

// UpdatePolicy controls how/when the node is updated.
type UpdatePolicy struct {
	// Mode is the update mode.
	// +optional
	// +kubebuilder:validation:Enum=Automatic;Manual
	Mode string `json:"mode,omitempty"`
	// Window is the maintenance window for updates.
	// +optional
	Window UpdateWindow `json:"window,omitempty"`
}

// UpdateWindow is the maintenance window for updates.
type UpdateWindow struct {
	// From is the window start time, "HH:MM" (24h).
	// +optional
	// +kubebuilder:validation:Pattern=`^([01][0-9]|2[0-3]):[0-5][0-9]$`
	From string `json:"from,omitempty"`
	// To is the window end time, "HH:MM" (24h).
	// +optional
	// +kubebuilder:validation:Pattern=`^([01][0-9]|2[0-3]):[0-5][0-9]$`
	To string `json:"to,omitempty"`
	// Days are the weekdays the window applies to.
	// +optional
	// +kubebuilder:validation:items:Enum=Mon;Tue;Wed;Thu;Fri;Sat;Sun
	Days []string `json:"days,omitempty"`
}

func init() {
	SchemeBuilder.Register(&NodeConfig{}, &NodeConfigList{})
}
