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

// NodeConfig is the desired state of a Deckhouse olcedar node, stored at
// /config/nodeconfig.yaml and as a cluster CRD (crds/nodeconfig.yaml, generated).
// Keep identical with nodelet's internal/config/types.go and dhctl's spec-only mirrors.
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
	// ObservedGeneration is the latest spec generation the node has processed:
	// it reaches the newest generation as soon as the node has looked at it,
	// even while it is still held for approval (see AppliedGeneration).
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// AppliedGeneration is the spec generation the node is actually running; it
	// lags ObservedGeneration while a disruptive config is held for approval.
	// "This node has converged" means AppliedGeneration == metadata.generation.
	// +optional
	AppliedGeneration int64 `json:"appliedGeneration,omitempty"`
	// Phase summarises the node. Ready: running the published config, healthy.
	// Pending: healthy but not yet running the published config (held for
	// approval). Degraded: a subsystem failed, config rejected, or rolled back.
	// +optional
	// +kubebuilder:validation:Enum=Ready;Pending;Degraded
	Phase string `json:"phase,omitempty"`
	// Conditions are the node-level reconcile outcomes (ConfigurationApplied,
	// DisruptionRequired) plus the gate subsystems (APIEndpointsReachable,
	// SysctlApplied); per-extension and per-unit outcomes live in Extensions and Units.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Extensions is the outcome of each configured system extension, one entry
	// per extension. Republished on every pass, empty when the pass checked
	// nothing — never "the previous outcome still holds".
	// +optional
	// +listType=map
	// +listMapKey=name
	Extensions []ExtensionStatus `json:"extensions,omitempty"`
	// Units is the outcome of each managed systemd unit (containerd, kubelet and
	// every unit an extension ships), one entry per unit. Republished on every
	// pass like Extensions, and empty when the pass checked nothing.
	// +optional
	// +listType=map
	// +listMapKey=name
	Units []UnitStatus `json:"units,omitempty"`
	// LastReconcileTime is when the node last finished a reconcile pass — the
	// only thing in the status that ages, so it is what tells a dead agent from
	// a healthy one. Republished coarsely: judge staleness in tens of minutes.
	// +optional
	LastReconcileTime metav1.Time `json:"lastReconcileTime,omitempty"`
	// OSImage is how far the node has got with spec.osImage. Mirrors
	// config.OSImageStatus in the nodelet repository (internal/config/types.go):
	// a field missing here is pruned and the node's whole status apply fails.
	// +optional
	OSImage *OSImageStatus `json:"osImage,omitempty"`
	// MaintenanceToken is the bearer token an operator presents to the node's
	// :50000 config-push port once the node has lost the API. It is a
	// root-equivalent credential, hence x-kubernetes-sensitive-data in the CRD.
	// +optional
	MaintenanceToken string `json:"maintenanceToken,omitempty"`
}

// OSImageStatus is the node's side of a rootfs update: what it runs, what it is
// trying, and what it has already refused.
type OSImageStatus struct {
	// Digest is the image the node is running, as recorded on its config
	// partition. Empty on a node installed before that record existed.
	// +optional
	Digest string `json:"digest,omitempty"`
	// Slot is the A/B slot that image lives in ("a" or "b").
	// +optional
	Slot string `json:"slot,omitempty"`
	// TrialDigest is the image staged for the next boot, or on trial in this one.
	// Set only while an update is in flight, which is exactly when Digest still
	// names the old image.
	// +optional
	TrialDigest string `json:"trialDigest,omitempty"`
	// AttemptsLeft is how many boots the trial image has left to prove itself
	// before the initramfs rolls back.
	// +optional
	AttemptsLeft int `json:"attemptsLeft,omitempty"`
	// FailedDigest is an image this node booted and rolled back from. It is never
	// tried again: a digest names immutable content, so a fixed image is a
	// different digest.
	// +optional
	FailedDigest string `json:"failedDigest,omitempty"`
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
	// OSImage is the image the cluster believes this node should run. Required: a
	// rootfs update is decided by comparing it with what the node recorded at
	// install, and a config that names no image is one no update can start from.
	OSImage OSImage `json:"osImage"`
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
	// +kubebuilder:validation:items:Pattern=`^(https?://)?(\[[0-9A-Fa-f:]+\]|[A-Za-z0-9]([-A-Za-z0-9]*[A-Za-z0-9])?([.][A-Za-z0-9]([-A-Za-z0-9]*[A-Za-z0-9])?)*):(6553[0-5]|655[0-2][0-9]|65[0-4][0-9]{2}|6[0-4][0-9]{3}|[1-5][0-9]{4}|[1-9][0-9]{0,3})/?$`
	APIServerEndpoints []string `json:"apiServerEndpoints,omitempty"`
	// UpdatePolicy controls how and when the node is updated.
	// +optional
	UpdatePolicy UpdatePolicy `json:"updatePolicy,omitempty"`

	// RegistryPackagesProxyAccessTokenB64 is a base64-encoded token used to
	// authenticate against the registry packages proxy. Deliberately not marked
	// sensitive: nodelet must read it, and the token is identical on every node.
	// +optional
	// +kubebuilder:validation:Pattern=`^(([A-Za-z0-9+/]{4})*([A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?)?$`
	RegistryPackagesProxyAccessTokenB64 string `json:"registryPackagesProxyAccessTokenB64,omitempty"`

	// Registry is the container registry the node talks to directly, without
	// the registry-packages-proxy. A node bootstrapping a control plane has no
	// proxy yet but must pull images and sysexts; workers leave this empty.
	// +optional
	Registry *Registry `json:"registry,omitempty"`
}

// Registry describes direct access to a container registry: address, scheme
// and credentials. It feeds the extension downloader and containerd hosts.toml,
// so a control-plane static pod without imagePullSecrets can pull its image.
type Registry struct {
	// Address is the registry host, optionally with a port, e.g.
	// "registry.deckhouse.io" or "registry.example.com:5000".
	// +kubebuilder:validation:MinLength=1
	Address string `json:"address"`
	// Path is the repository path within the registry, e.g. "/deckhouse/ce".
	// +optional
	Path string `json:"path,omitempty"`
	// Scheme is HTTPS (default) or HTTP.
	// +optional
	// +kubebuilder:validation:Enum=HTTPS;HTTP
	// +kubebuilder:default=HTTPS
	Scheme string `json:"scheme,omitempty"`
	// CA is a PEM certificate bundle to verify the registry with, for registries
	// signed by a private CA. The image carries the Mozilla bundle and nothing
	// else, so without this a self-signed registry is unreachable.
	// +optional
	CA string `json:"ca,omitempty"`
	// Auth is the base64-encoded "user:password" pair, as it appears in the
	// "auth" field of a docker config.
	// +optional
	Auth string `json:"auth,omitempty"`
}

// OSImage names the image the node must run. Mirrors the agent's contract
// (internal/config/types.go in the nodelet repository) and the initramfs one
// (images/init/src/0.1/nodeconfig.go); a shape they refuse strands the node.
type OSImage struct {
	// Digest pins the exact image, as for an extension. The node records it at
	// install and compares it on every pass, so a tag would say nothing.
	// +kubebuilder:validation:Pattern=`^sha256:[a-f0-9]{64}$`
	Digest string `json:"digest"`
	// Repository is the registry host the image is fetched from, handed to the
	// registry-packages-proxy as its "repository" parameter. Empty leaves the
	// parameter out, so the proxy uses its own default registry.
	// +optional
	Repository string `json:"repository,omitempty"`
	// AdditionalPath is the proxy's "path" parameter. Empty is right for
	// Deckhouse: every image of a release sits in the repository
	// spec.registry.path already names.
	// +optional
	AdditionalPath string `json:"additionalPath,omitempty"`
}

// Storage selects the target disk for the OS install. The partition layout is
// fixed (boot ESP + config + data), so only the whole-disk device is needed.
// Consumed by the initramfs disk provisioner at install time.
type Storage struct {
	Disk `json:",inline"`
	// Mounts are additional filesystems the node makes available, at /mnt/<name>
	// or wherever bindTo names. Nothing here is partitioned: only existing
	// partitions and blank whole disks are formatted (when empty) and mounted.
	// +optional
	// +listType=map
	// +listMapKey=name
	Mounts []Mount `json:"mounts,omitempty"`
}

// Mount is one additional filesystem, at /mnt/<name> unless bindTo says
// otherwise. Exactly one of device or partitionSelector names the partition; an
// empty one is formatted (label = name), an existing filesystem mounted as is.
// +kubebuilder:validation:XValidation:rule="has(self.device) != has(self.partitionSelector)",message="exactly one of device or partitionSelector must be set"
type Mount struct {
	// Name identifies the mount, and is both the mount point (/mnt/<name>) and
	// the filesystem label written when this node formats the partition. Capped
	// at 16 characters because that is the size of the ext4 volume label field.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=16
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`
	// Device is the partition to use, e.g. "/dev/sdb1" or a stable
	// "/dev/disk/by-id/...-part1" path.
	// +optional
	// +kubebuilder:validation:Pattern=`^/dev/[A-Za-z0-9._/-]+$`
	Device string `json:"device,omitempty"`
	// PartitionSelector picks the partition by attributes instead of a fixed path.
	// +optional
	PartitionSelector *PartitionSelector `json:"partitionSelector,omitempty"`
	// BindTo mounts the filesystem at this path instead of /mnt/<name>, for a
	// directory something else knows by name (/var/lib/etcd). The directory must
	// be empty: mounting over files hides them, and hidden etcd data is lost data.
	// +optional
	// +kubebuilder:validation:Pattern=`^/[A-Za-z0-9._/-]+$`
	BindTo string `json:"bindTo,omitempty"`
	// Mode is the mode of the filesystem root after mounting, as an octal string,
	// e.g. "0700". Left alone when unset. A freshly made ext4 has its root at
	// 0755, which is a mode etcd refuses to start on.
	// +optional
	// +kubebuilder:validation:Pattern=`^0[0-7]{3}$`
	Mode string `json:"mode,omitempty"`
	// Filesystem is what to create when the partition is empty. It says what to
	// create and is not a matching condition.
	// +optional
	// +kubebuilder:validation:Enum=ext4
	// +kubebuilder:default=ext4
	Filesystem string `json:"filesystem,omitempty"`
}

// PartitionSelector matches one partition by attributes (all specified fields
// are AND-ed); string fields are shell-style globs, except uuid and partUUID
// (literal, case-insensitive). Matching several devices is an error, never a choice.
type PartitionSelector struct {
	// Name matches the kernel device name (glob), e.g. "sdb1" or "nvme0n1p*".
	// +optional
	Name string `json:"name,omitempty"`
	// UUID matches the filesystem UUID exactly, ignoring case.
	// +optional
	UUID string `json:"uuid,omitempty"`
	// Label matches the filesystem label (glob).
	// +optional
	Label string `json:"label,omitempty"`
	// FSType matches the type of the filesystem already present (glob).
	// +optional
	FSType string `json:"fsType,omitempty"`
	// PartUUID matches the GPT partition UUID exactly, ignoring case.
	// +optional
	PartUUID string `json:"partUUID,omitempty"`
	// PartLabel matches the GPT partition name (glob).
	// +optional
	PartLabel string `json:"partLabel,omitempty"`
	// Size matches the size, optionally with a comparison operator, e.g.
	// ">=100Gi", ">1Ti", "512Gi". Without an operator the comparison is ">=";
	// "=" allows 1%, since a disk rarely reports an exact round size.
	// +optional
	Size string `json:"size,omitempty"`
	// Blank makes whole disks selectable, and only ones that carry nothing: no
	// partition table, no filesystem — a cloud disk never touched. Without it a
	// selector sees partitions only; a whole disk is where somebody's layout lives.
	// +optional
	Blank bool `json:"blank,omitempty"`
}

// Disk names one whole-disk block device, by path or by attributes, and says
// whether it may be reformatted.
type Disk struct {
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
	// disk is set up, so an existing layout stays and a reboot never destroys data.
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

// Extension is a signed verity sysext built from a release channel, fetched
// from the registry-packages-proxy by digest. Optional repository selects the
// proxy's per-registry config; optional additionalPath is its "path" parameter.
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
	// +kubebuilder:validation:Maximum=1000
	// +kubebuilder:default=120
	MaxPods int `json:"maxPods,omitempty"`
	// KubernetesVersion is the cluster's minor version, e.g. "1.34". It decides
	// which feature gates kubelet is started with (bashible turns DRA gates on
	// by version); without it DRA workloads cannot run on immutable nodes.
	// +optional
	// +kubebuilder:validation:MaxLength=16
	// +kubebuilder:validation:Pattern=`^[0-9]+\.[0-9]+$`
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
	// ContainerLogMaxSize is the maximum log file size before rotation
	// (e.g. "50Mi").
	// +optional
	// +kubebuilder:default="50Mi"
	// +kubebuilder:validation:XValidation:rule="isQuantity(self) && sign(quantity(self)) > 0",message="containerLogMaxSize must be a positive Kubernetes quantity"
	ContainerLogMaxSize string `json:"containerLogMaxSize,omitempty"`
	// ContainerLogMaxFiles is the number of rotated log files to retain.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
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
	// ServerTLSBootstrap makes kubelet request a serving certificate from the
	// cluster instead of signing its own. Default true. The zero master sets it
	// to false: nothing approves serving CSRs until Deckhouse is installed.
	// +optional
	ServerTLSBootstrap *bool `json:"serverTLSBootstrap,omitempty"`
	// NodeIP is the address kubelet registers the node with (--node-ip). Left
	// empty kubelet picks an address itself, which on a multi-homed node is not
	// always the one the cluster routes to.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == '' || isIP(self)",message="nodeIP must be a valid IP address"
	NodeIP string `json:"nodeIP,omitempty"`
	// ResourceReservation controls how much CPU, memory and disk are held back
	// from pods for the system itself (kubeReserved).
	// +optional
	ResourceReservation *ResourceReservation `json:"resourceReservation,omitempty"`
}

// ResourceReservation is the kubeReserved policy for the node.
type ResourceReservation struct {
	// Mode is Auto to compute the reservation from the node's capacity, or Off
	// to reserve nothing.
	// +kubebuilder:validation:Enum=Auto;Off
	// +kubebuilder:default=Auto
	Mode string `json:"mode,omitempty"`
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
