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

// Maintainers: the schema's source is nodelet's api/internal.deckhouse.io/v1alpha1, and
// crds/nodeconfig.yaml, which validates, is copied from it (hack/sync-nodeconfig-crd.sh). These
// types keep its shape (TestTheGoTypesAndTheShippedCRDAgree), as do dhctl/pkg/immutable/types.go.

// NodeConfig is the desired state of a Deckhouse node agent (nodelet), one object per node. An
// Engine node reads the same document from /config/nodeconfig.yaml before it reaches the cluster.
// +kubebuilder:object:root=true
type NodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NodeSpec `json:"spec"`

	// Status is reported by the node agent after each reconcile pass.
	Status NodeConfigStatus `json:"status,omitempty"`
}

// NodeConfigStatus is the observed state reported by the node agent.
type NodeConfigStatus struct {
	// ObservedGeneration is the latest spec generation the node has looked at. It reaches
	// the newest generation even while the node holds that generation for approval; see
	// appliedGeneration for what the node runs.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// AppliedGeneration is the spec generation the node has applied. The current spec is applied
	// when it equals metadata.generation and ConfigurationApplied is True at that generation.
	AppliedGeneration int64 `json:"appliedGeneration,omitempty"`
	// Phase summarises the node: Ready once it applied the published configuration, Pending
	// while it has not yet, Degraded when a subsystem failed or the configuration was refused.
	Phase string `json:"phase,omitempty"`
	// Features that ran on the node during its last pass, sorted by name: the gates
	// switched on in spec.featureGates plus the defaults of the node type.
	Features []string `json:"features,omitempty"`
	// Conditions of the node: Ready, ConfigurationApplied, DisruptionApproved and AddressResolved
	// at the generation it processed, one per subsystem at the generation it runs. The reasons are
	// listed in crds/nodeconfig.yaml.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Extensions is the outcome of each configured extension, one entry per extension.
	// Republished on every pass, and empty when the pass checked nothing.
	Extensions []ExtensionStatus `json:"extensions,omitempty"`
	// Units is the outcome of each managed systemd unit (containerd, kubelet and every
	// unit an extension ships), one entry per unit. Republished on every pass, and empty
	// when the pass checked nothing.
	Units []UnitStatus `json:"units,omitempty"`

	// Maintainers: no omitempty on purpose: an empty pass has to say so explicitly, and the
	// node turns the nil slice into [] before it applies the status.

	// StaticPods is the outcome of each static pod, one entry per spec.staticPods item.
	// Republished on every pass, and empty when the pass checked no static pods.
	StaticPods []StaticPodStatus `json:"staticPods"`
	// Network is what the node resolved for itself: the interface it took its cluster
	// address from, and that address. Written by the node alone.
	Network *NetworkStatus `json:"network,omitempty"`
	// LastReconcileTime is when the node last finished a reconcile pass. It is the only
	// field of the status that ages, so it tells a stopped agent from a healthy one.
	// Republished coarsely: judge staleness in tens of minutes.
	LastReconcileTime metav1.Time `json:"lastReconcileTime,omitempty"`
	// OSImage is the state of the operating system image on the node. Filled in by the node.
	OSImage *OSImageStatus `json:"osImage,omitempty"`

	// MaintenanceToken is the bearer token an operator presents to the node's :50000
	// configuration port once the node has lost the API. The node creates it once and keeps
	// it in /var/lib/nodelet/maintenance-token.
	MaintenanceToken string `json:"maintenanceToken,omitempty"`
	// Agent describes the node agent that wrote this status.
	Agent *AgentStatus `json:"agent,omitempty"`
}

// AgentStatus describes the node agent that wrote the status.
type AgentStatus struct {
	// Version is the build of the agent: a release tag such as v0.1.16, or a short commit
	// for a development build. For display only.
	Version string `json:"version,omitempty"`
}

// OSImageStatus is the node's side of an operating system image update: what it runs,
// what it is trying, and what it has already refused.
type OSImageStatus struct {
	// Digest of the operating system image the node runs, read from its configuration
	// partition. Absent on a node installed before the digest was recorded there.
	Digest string `json:"digest,omitempty"`
	// Slot is the A/B slot (a or b) holding the image the node runs.
	Slot string `json:"slot,omitempty"`
	// RootHash is the dm-verity root hash of the image the node runs, read from the active
	// slot. The digests of the desired and the running image may differ after a republish
	// while equal root hashes say the root filesystem has not changed.
	RootHash string `json:"rootHash,omitempty"`
	// TrialDigest is the image prepared for the next boot or on trial in the current one.
	// Set only during an update, while digest still names the previous image.
	TrialDigest string `json:"trialDigest,omitempty"`
	// AttemptsLeft is how many boots the image on trial has left before the initramfs rolls
	// back to the previous image.
	AttemptsLeft int `json:"attemptsLeft,omitempty"`
	// FailedDigest is an image the node booted and rolled back from. It is never tried
	// again: a digest names immutable content, so a fixed image has a different digest.
	FailedDigest string `json:"failedDigest,omitempty"`
}

// NetworkStatus is the address the node joined the cluster on and the interface it sits on.
type NetworkStatus struct {
	// ClusterInterface is the network interface the address was taken from.
	ClusterInterface string `json:"clusterInterface,omitempty"`
	// Address is the address kubelet registered the node with.
	Address string `json:"address,omitempty"`
}

// ExtensionStatus is the reconcile outcome of one extension.
type ExtensionStatus struct {
	// Name is the extension name: a spec.extensions[].name, or the name of an extension
	// removed from spec whose uninstall failed on a Mutable node.
	Name string `json:"name"`
	// Digest depends on state: installed when Ready, being installed when Pending, refused or not
	// installable when Failed.
	Digest string `json:"digest,omitempty"`
	// State of the extension: Ready once installed, Pending while it is fetched or waits for the
	// update window, Failed with the cause in message.
	State string `json:"state"`
	// Message carries the cause when state is Failed.
	Message string `json:"message,omitempty"`
}

// UnitStatus is the reconcile outcome of one managed systemd unit.
type UnitStatus struct {
	// Name is the systemd unit name, for example containerd.service.
	Name string `json:"name"`
	// State is Active when the unit runs, Pending when it is queued to start later in this
	// pass, or Failed with the cause in message.
	State string `json:"state"`
	// Message carries the cause when state is Failed.
	Message string `json:"message,omitempty"`
}

// StaticPodStatus is the processing result of one static pod.
type StaticPodStatus struct {
	// Name of the static pod, as in spec.staticPods[].name.
	Name string `json:"name"`
	// State of the manifest: Written when the file on disk matches spec, Failed otherwise, with
	// the cause in reason.
	State string `json:"state"`
	// ManifestDigest is the sha256 of the manifest the node wrote, so the cluster can tell
	// which revision of the request the node runs: sha256:<hex> of spec.staticPods[].manifest
	// as published, before $MY_IP is expanded. Set next to state Written.
	ManifestDigest string `json:"manifestDigest,omitempty"`
	// Reason for the Failed state: ManifestRejected, WriteFailed, or RemoveFailed when the pod left
	// spec but its file could not be removed and the pod keeps running.
	Reason string `json:"reason,omitempty"`
	// Message is the detail of the error when state is Failed.
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

// The conditions node-manager reads off a NodeConfig, and the reasons of a False
// DisruptionApproved: the node asks to be interrupted, or node-manager stopped answering it.
const (
	NodeConfigConditionTypeConfigurationApplied = "ConfigurationApplied"
	NodeConfigConditionTypeDisruptionApproved   = "DisruptionApproved"

	NodeConfigReasonApprovalPending   = "ApprovalPending"
	NodeConfigReasonAttemptsExhausted = "AttemptsExhausted"
)

// SystemType says how a node is managed. The values are NodeGroup's spec.systemType;
// mirrors internal/config/systemtype.go in the nodelet repository.
type SystemType string

const (
	SystemTypeImmutable SystemType = "Immutable"
	SystemTypeMutable   SystemType = "Mutable"
)

// NodeSpec is the desired state of the node.
type NodeSpec struct {
	// SystemType is how the node is managed, and cannot be changed: Immutable, a Deckhouse Engine
	// node the agent configures whole, or Mutable, a bashible node where it runs only featureGates.
	SystemType SystemType `json:"systemType"`
	// FeatureGates switch node agent features on or off by name, set by node-controller. A missing
	// one is on for an Immutable node if its type supports it, and off for a Mutable node.
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
	// NodeName is the name of the Kubernetes node this configuration applies to. Equal to
	// the object name and cannot be changed.
	NodeName string `json:"nodeName"`

	// Maintainers: omitzero, as Storage, Kernel, Network, Kubelet, ContainerRuntime,
	// UpdatePolicy and RegistryPackagesProxy: an empty section is left out rather than sent
	// as {}, and a Mutable document must not carry osImage: {digest: ""}.

	// OSImage is the operating system image the node runs, required when systemType is Immutable.
	// The node compares its digest with /config/os-digest, saved at installation, to decide an update.
	OSImage OSImage `json:"osImage,omitzero"`
	// Storage selects the whole disk the system is installed on and the filesystems the node
	// mounts. device, wipe and diskSelector cannot be changed; mounts can.
	Storage Storage `json:"storage,omitzero"`
	// Extensions are what the node installs, each pinned by digest: a signed dm-verity system
	// extension on an Immutable node, a registry package on a Mutable one. An entry named nodelet
	// updates the node agent itself.
	Extensions []Extension `json:"extensions,omitempty"`

	// StaticPods are the manifests the node writes to /etc/kubernetes/manifests for kubelet. The
	// control plane manifests the node agent writes itself are not part of this list.
	StaticPods []StaticPod `json:"staticPods,omitempty"`
	// Kernel holds sysctl settings and kernel modules to load.
	Kernel Kernel `json:"kernel,omitzero"`
	// Network holds the hostname, DNS, NTP, interfaces and routes of the node. Written by
	// whoever provisions the machine and changed through the API afterwards.
	Network Network `json:"network,omitzero"`
	// Kubelet holds kubelet parameters.
	Kubelet Kubelet `json:"kubelet,omitzero"`
	// ContainerRuntime holds containerd parameters.
	ContainerRuntime ContainerRuntime `json:"containerRuntime,omitzero"`
	// APIServerEndpoints are the addresses of the Kubernetes API servers the node connects
	// to through its node-local API proxy, each as https://<host>:<port>.
	APIServerEndpoints []string `json:"apiServerEndpoints,omitempty"`

	// InternalNetworkCIDRs are the subnets the cluster's own traffic runs on. The node picks
	// the interface holding an address inside one of them and registers with that address.
	// A node with one network interface needs none.
	InternalNetworkCIDRs []string `json:"internalNetworkCIDRs,omitempty"`

	// StatusToken is the bearer token presented to the node's :50000 status port. Whoever
	// writes the document creates it (the installer, or the NodeConfigTemplate for a machine
	// added by hand). The cluster cannot give the node a new one, so it never changes.
	StatusToken string `json:"statusToken,omitempty"`
	// UpdatePolicy says how and when the node applies updates that interrupt it.
	UpdatePolicy UpdatePolicy `json:"updatePolicy,omitzero"`
	// RegistryPackagesProxy is how the node reaches the registry-packages-proxy it downloads
	// extensions through.
	RegistryPackagesProxy RegistryPackagesProxy `json:"registryPackagesProxy,omitzero"`
	// Registry is the container registry the node talks to directly, without the proxy: a node
	// bootstrapping the control plane has no proxy yet, and containerd pulls the pause image itself.
	Registry *Registry `json:"registry,omitempty"`
}

// RegistryPackagesProxy is how the node reaches the registry-packages-proxy.
type RegistryPackagesProxy struct {
	// Endpoints are the proxy addresses, with scheme and port, used instead of the API server
	// hosts. Set only in the first master's initial configuration; node-controller does not set them.
	Endpoints []string `json:"endpoints,omitempty"`

	// AccessToken is the token the node authenticates to the proxy with, base64-encoded.
	AccessToken []byte `json:"accessToken,omitempty"`
}

// Registry describes direct access to a container registry: address, scheme and
// credentials.
type Registry struct {
	// Address is the registry host, optionally with a port, for example
	// registry.deckhouse.io or registry.example.com:5000.
	Address string `json:"address"`
	// Path is the repository path within the registry, for example /deckhouse/ce.
	Path string `json:"path,omitempty"`
	// Scheme is the protocol the registry is reached with. Defaults to HTTPS when unset.
	Scheme string `json:"scheme,omitempty"`
	// CABundle is a PEM certificate bundle to verify a registry signed by a private
	// certificate authority. The node trusts the public authorities only, so a registry
	// with a self-signed certificate is unreachable without it.
	CABundle string `json:"caBundle,omitempty"`
	// Auth is the user:password pair, base64-encoded, as in the auth field of a docker
	// config.
	Auth []byte `json:"auth,omitempty"`
}

// ObjectOwner names the object that asked for an entry of the spec.
type ObjectOwner struct {
	// Kind of the object.
	Kind string `json:"kind"`
	// Name of the object.
	Name string `json:"name"`
}

// The kinds an ObjectOwner names.
const (
	ObjectOwnerKindNodeExtensionRequest = "NodeExtensionRequest"
	ObjectOwnerKindNodeStaticPodRequest = "NodeStaticPodRequest"
)

// Maintainers: mirrors the agent's contract (internal/config/types.go in the nodelet
// repository) and the initramfs one (images/init/src/0.1/nodeconfig.go); a shape they
// refuse strands the node.

// OSImage names the operating system image the node runs.
type OSImage struct {
	// Digest of the operating system image. The node saves it at installation and compares
	// it with this value to decide whether an update is needed.
	Digest string `json:"digest"`
	// Repository is the registry host the image is fetched from, handed to the
	// registry-packages-proxy as its "repository" parameter. Empty leaves the
	// parameter out, so the proxy uses its own default registry.
	Repository string `json:"repository,omitempty"`
	// Path of the image within the repository, passed to the registry-packages-proxy
	// as its path parameter. When unset, the repository root is used.
	Path string `json:"path,omitempty"`

	// Maintainers: optional, filled in from the image's own label when the registry can be
	// read. Its absence costs the node one download, which is why nothing fails when the
	// lookup does.

	// RootHash is the dm-verity root hash of the root filesystem, in hex: unlike the digest, it
	// stays the same when the image is published again. When unset, the node reads it from the image.
	RootHash string `json:"rootHash,omitempty"`
}

// Storage selects the disk the system is installed on and additional filesystems.
// Consumed by the initramfs disk provisioner at installation.
type Storage struct {
	Disk `json:",inline"`
	// Mounts are additional filesystems mounted at /mnt/<name> or at bindTo. Nothing is
	// partitioned: a blank partition or disk is formatted, a filesystem is mounted as it is.
	Mounts []Mount `json:"mounts,omitempty"`
}

// Mount is one additional filesystem, at /mnt/<name> unless bindTo names another path.
// Exactly one of device or partitionSelector names the partition. An empty partition is
// formatted with the label set to name, an existing filesystem is mounted as it is.
type Mount struct {
	// Name of the mount: the mount point /mnt/<name> and the filesystem label written when
	// the node formats the partition. At most 16 characters, the size of the ext4 volume
	// label.
	Name string `json:"name"`
	// Device is the partition to use, for example /dev/sdb1 or a stable
	// /dev/disk/by-id/...-part1 path.
	Device string `json:"device,omitempty"`
	// PartitionSelector picks the partition by attributes instead of a fixed path.
	PartitionSelector *PartitionSelector `json:"partitionSelector,omitempty"`
	// BindTo mounts the filesystem at this path instead of /mnt/<name>, for a directory
	// another component knows by name, such as /var/lib/etcd. The directory must be empty:
	// mounting over files hides them.
	BindTo string `json:"bindTo,omitempty"`
	// Mode of the filesystem root after mounting, as an octal string, for example 0700.
	// When unset, the mode is left as it is; a freshly made ext4 filesystem has its root at
	// 0755, a mode etcd refuses to start on.
	Mode string `json:"mode,omitempty"`
	// Filesystem to create when the partition is empty. Not a matching condition. Defaults
	// to Ext4 when unset.
	Filesystem string `json:"filesystem,omitempty"`
}

// PartitionSelector matches one partition by attributes. All specified fields must match.
// String fields are shell-style globs, except uuid and partUUID, which match exactly,
// ignoring case. Matching several partitions is an error, never a choice.
type PartitionSelector struct {
	// Name matches the kernel device name (glob), for example sdb1 or nvme0n1p*.
	Name string `json:"name,omitempty"`
	// UUID matches the filesystem UUID exactly, ignoring case.
	UUID string `json:"uuid,omitempty"`
	// Label matches the filesystem label (glob).
	Label string `json:"label,omitempty"`
	// FSType matches the type of the filesystem already present (glob).
	FSType string `json:"fsType,omitempty"`
	// PartUUID matches the GPT partition UUID exactly, ignoring case.
	PartUUID string `json:"partUUID,omitempty"`
	// PartLabel matches the GPT partition name (glob).
	PartLabel string `json:"partLabel,omitempty"`
	// Size matches the size, optionally with a comparison operator, for example >=100Gi,
	// >1Ti or 512Gi. Without an operator the comparison is >=; = allows 1%, since a disk
	// rarely reports an exact round size.
	Size string `json:"size,omitempty"`
	// Blank makes whole disks selectable, and only disks that carry nothing: no partition
	// table and no filesystem, such as a cloud disk never used. Without it the selector sees
	// partitions only.
	Blank bool `json:"blank,omitempty"`
}

// Disk names one whole-disk block device, by path or by attributes, and says whether it
// may be reformatted.
type Disk struct {
	// Device is the whole-disk block device to install onto, for example /dev/sda,
	// /dev/nvme0n1 or a stable /dev/disk/by-id/... path. Ignored when diskSelector is set.
	// Cannot be changed.
	Device string `json:"device,omitempty"`
	// DiskSelector picks the disk to install onto by attributes instead of a fixed path,
	// and takes priority over device. All specified fields must match; the first disk that
	// matches is used. Cannot be changed.
	DiskSelector *DiskSelector `json:"diskSelector,omitempty"`
	// Wipe makes the installer wipe and repartition a disk that is already provisioned.
	// When false, only a blank or non-matching disk is set up, so an existing layout stays
	// and a reboot never destroys data. Cannot be changed.
	Wipe bool `json:"wipe,omitempty"`
}

// DiskSelector matches a disk by attributes. All specified fields must match. Fields
// marked glob take shell-style patterns. Attributes are read from lsblk and /sys/block.
type DiskSelector struct {
	// Size matches the disk capacity, optionally with a comparison operator, for example
	// >=100Gi, >1Ti or 512Gi.
	Size string `json:"size,omitempty"`
	// Type matches the kind of disk.
	Type string `json:"type,omitempty"`
	// Rotational matches spinning (true) or solid-state (false) disks.
	Rotational *bool `json:"rotational,omitempty"`
	// Model matches the device model (glob), for example Samsung*.
	Model string `json:"model,omitempty"`
	// Serial matches the disk serial number (glob).
	Serial string `json:"serial,omitempty"`
	// WWID matches the World Wide Identifier (glob).
	WWID string `json:"wwid,omitempty"`
	// Name matches the kernel device name (glob), for example nvme0n1.
	Name string `json:"name,omitempty"`
	// BusPath matches the hardware bus path (glob).
	BusPath string `json:"busPath,omitempty"`
}

// Extension is one artifact the node installs, pinned by digest and fetched through the
// registry-packages-proxy.
type Extension struct {
	// Name of the extension: the system extension image name on an Immutable node, the
	// registry package name on a Mutable one.
	Name string `json:"name"`
	// Digest of the artifact image.
	Digest string `json:"digest"`
	// Repository optionally selects the proxy's per-registry client config.
	Repository string `json:"repository,omitempty"`
	// Path of the artifact within the repository, passed to the
	// registry-packages-proxy as its path parameter. When unset, the repository root is used.
	Path string `json:"path,omitempty"`
	// RequestedBy is the object that asked for the extension. Unset for the extensions of
	// the platform itself.
	RequestedBy *ObjectOwner `json:"requestedBy,omitempty"`
}

// StaticPod is one manifest for /etc/kubernetes/manifests. The node prepares the image in
// containerd and this file on disk, and nothing else.
type StaticPod struct {
	// Name of the manifest file without an extension, and the key of the list. Unrelated to
	// metadata.name of the pod in the manifest: kubelet names the mirror pod after the
	// manifest, not after the file.
	Name string `json:"name"`
	// Manifest is the complete pod manifest. When the manifest is written to disk, $MY_IP is
	// replaced with the node's IP address, as in the control plane manifests.
	Manifest string `json:"manifest"`
	// RequestedBy is the object that asked for the static pod. Unset for the static pods of
	// the platform itself.
	RequestedBy *ObjectOwner `json:"requestedBy,omitempty"`
}

// SysctlValue is one sysctl value.
type SysctlValue string

// DeepCopy lets controller-gen copy maps whose values use this scalar type.
func (in SysctlValue) DeepCopy() *SysctlValue {
	out := new(SysctlValue)
	*out = in
	return out
}

// Kernel holds sysctl settings and kernel modules to load.
type Kernel struct {
	// Sysctl maps sysctl keys in dotted notation to their values.
	Sysctl map[string]SysctlValue `json:"sysctl,omitempty"`
	// Modules are the kernel modules to load.
	Modules []KernelModule `json:"modules,omitempty"`
}

// KernelModule is a module to load with optional parameters.
type KernelModule struct {
	// Name of the kernel module.
	Name string `json:"name"`
	// Params are the module parameters as key=value strings.
	Params []string `json:"params,omitempty"`
}

// Network holds the hostname, DNS, NTP, interfaces and routes of the node.
type Network struct {
	// Hostname of the node. Equal to the Kubernetes node name.
	Hostname string `json:"hostname,omitempty"`
	// DNS is the resolver configuration.
	DNS DNS `json:"dns,omitempty"`
	// NTP is the time synchronization configuration.
	NTP NTP `json:"ntp,omitempty"`
	// Interfaces are the network interfaces of the node.
	Interfaces []NetworkInterface `json:"interfaces,omitempty"`
	// Routes are static routes.
	Routes []Route `json:"routes,omitempty"`
}

// DNS is the resolver configuration.
type DNS struct {
	// Servers are the IP addresses of the DNS servers.
	Servers []string `json:"servers,omitempty"`
	// Search are the DNS search domains.
	Search []string `json:"search,omitempty"`
}

// NTP is the time synchronization configuration.
type NTP struct {
	// Servers are the NTP servers, by host name or IP address.
	Servers []string `json:"servers,omitempty"`
}

// NetworkInterface describes one network interface.
type NetworkInterface struct {
	// Name of the interface.
	Name string `json:"name"`
	// DHCP enables DHCPv4 on the interface.
	DHCP bool `json:"dhcp"`
	// Addresses are the static addresses of the interface in CIDR notation, used when dhcp
	// is false.
	Addresses []string `json:"addresses,omitempty"`
	// Gateway is the IP address of the default gateway.
	Gateway string `json:"gateway,omitempty"`
	// Cluster marks this interface as the one the cluster's traffic runs on, for a machine
	// whose addresses spec.internalNetworkCIDRs cannot tell apart.
	Cluster bool `json:"cluster,omitempty"`
}

// NodeLabelValue is the value of a node label.
type NodeLabelValue string

// DeepCopy lets controller-gen copy maps whose values use this scalar type.
func (in NodeLabelValue) DeepCopy() *NodeLabelValue {
	out := new(NodeLabelValue)
	*out = in
	return out
}

// Route is a static route.
type Route struct {
	// Name of the route.
	Name string `json:"name,omitempty"`
	// Networks are the destination networks in CIDR notation.
	Networks []string `json:"networks,omitempty"`
	// Gateway is the IP address the networks are reached through.
	Gateway string `json:"gateway,omitempty"`
}

// Kubelet holds kubelet parameters.
type Kubelet struct {
	// ClusterDomain is the DNS domain of the cluster, for example cluster.local.
	ClusterDomain string `json:"clusterDomain,omitempty"`
	// ClusterDNS are the IP addresses of the cluster DNS servers.
	ClusterDNS []string `json:"clusterDNS,omitempty"`
	// MaxPods is the maximum number of pods on the node. Defaults to 120 when unset.
	MaxPods int `json:"maxPods,omitempty"`
	// KubernetesVersion is the minor version of the cluster, for example 1.34. It decides
	// which feature gates kubelet starts with.
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
	// ContainerLogMaxSize is the size a container log file reaches before it is rotated,
	// for example 50Mi. Defaults to 50Mi when unset.
	ContainerLogMaxSize string `json:"containerLogMaxSize,omitempty"`

	// ContainerLogMaxFiles is the number of rotated log files kept per container. Defaults
	// to 4 when unset.
	ContainerLogMaxFiles int `json:"containerLogMaxFiles,omitempty"`
	// ClusterCABundle is the PEM certificate bundle of the cluster certificate authority
	// kubelet verifies the API server with.
	ClusterCABundle string `json:"clusterCABundle,omitempty"`
	// BootstrapToken is the bootstrap token kubelet obtains its client certificate with on
	// the first boot. Present only in the initial configuration of a node.
	BootstrapToken string `json:"bootstrapToken,omitempty"`
	// RegisterWithTaints are the taints kubelet puts on the node object when it registers
	// the node. Take effect only at the first registration.
	RegisterWithTaints []Taint `json:"registerWithTaints,omitempty"`
	// ExternalCloudProvider starts kubelet with --cloud-provider=external, so that the
	// cloud-controller-manager manages the node (zone, region, providerID).
	ExternalCloudProvider bool `json:"externalCloudProvider,omitempty"`
	// NodeLabels are the labels kubelet puts on the node object when it registers the node.
	NodeLabels map[string]NodeLabelValue `json:"nodeLabels,omitempty"`
	// ServerTLSBootstrap makes kubelet request its serving certificate from the cluster
	// instead of signing its own. Defaults to true when unset. The first master sets it to
	// false: nothing approves serving certificate requests until Deckhouse is installed.
	ServerTLSBootstrap *bool `json:"serverTLSBootstrap,omitempty"`
	// ResourceReservation controls how much CPU, memory and disk are held back from pods
	// for the system itself (kubeReserved).
	ResourceReservation *ResourceReservation `json:"resourceReservation,omitempty"`
}

// ResourceReservation is the kubeReserved policy of the node.
type ResourceReservation struct {
	// Mode is the reservation policy: Auto, the default, computes it from the node's capacity,
	// and Off reserves nothing.
	Mode string `json:"mode,omitempty"`
}

// Taint is a taint kubelet puts on the node when it registers it.
type Taint struct {
	// Key of the taint.
	Key string `json:"key"`
	// Value of the taint.
	Value string `json:"value,omitempty"`
	// Effect of the taint.
	Effect string `json:"effect,omitempty"`
}

// RegistryConfigOwner says who writes containerd's registry configuration directory.
type RegistryConfigOwner string

const (
	RegistryConfigOwnerNodelet       RegistryConfigOwner = "Nodelet"
	RegistryConfigOwnerRegistryAgent RegistryConfigOwner = "RegistryAgent"
)

// Maintainers: the maxConcurrentDownloads default is bashible's
// (candi/bashible/common-steps/all/032_configure_containerd.sh.tpl). Zero is containerd's
// "no limit", and a NodeGroup takes any integer: a value it accepts must be accepted here.

// ContainerRuntime holds containerd parameters.
type ContainerRuntime struct {
	// SandboxImage is the pause image used for pod sandboxes. When unset, the pause image
	// imported by the containerd system extension is used.
	SandboxImage string `json:"sandboxImage,omitempty"`
	// MaxConcurrentDownloads limits parallel image layer downloads; 0 means no limit.
	// Defaults to 8 when unset.
	MaxConcurrentDownloads *int `json:"maxConcurrentDownloads,omitempty"`
	// RegistryConfigOwner manages containerd's registry.d, set by node-controller from the
	// registry module's mode: Nodelet, the default, writes it from spec.registry, and with
	// RegistryAgent the registry agent static pod does while the node agent leaves it alone.
	RegistryConfigOwner RegistryConfigOwner `json:"registryConfigOwner,omitempty"`
}

// UpdatePolicy says how and when the node applies updates that interrupt it.
type UpdatePolicy struct {
	// Mode is how a disruptive update is approved, as disruptions.approvalMode of the
	// NodeGroup: Automatic or Manual.
	Mode string `json:"mode,omitempty"`
	// Windows are the time windows in UTC when the node may apply an update. Any window
	// that matches allows it; when unset, an update is allowed at any time.
	Windows []UpdateWindow `json:"windows,omitempty"`
}

// UpdateWindow is one time window for updates.
type UpdateWindow struct {
	// From is the start of the window in UTC, H:MM or HH:MM (24h).
	From string `json:"from"`
	// To is the end of the window in UTC, in the same format as from.
	To string `json:"to"`
	// Days are the days of the week the window applies to. When unset, every day.
	Days []string `json:"days,omitempty"`
}

func init() {
	SchemeBuilder.Register(&NodeConfig{}, &NodeConfigList{})
}
