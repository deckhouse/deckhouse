// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package immutable builds the cloud-init payload that boots the first
// control-plane node of a Deckhouse cluster on the immutable olcedar OS.
//
// Such a node runs no sshd, no bash and no bashible: the on-node agent
// (nodelet) reads its desired state from /config/nodeconfig.yaml and
// /config/controlplane.yaml, issues its own leaf certificates on top of the CA
// dhctl generates, lays the control-plane manifests down and waits for its own
// apiserver. dhctl only hands over the payload and then talks to the finished
// API server.
package immutable

// PayloadAPIVersion and the kinds below are the contract with the on-node
// agent. It parses both documents with UnmarshalStrict, so every field name
// here must match the agent's types byte for byte — see
// modules/040-node-manager/images/node-controller/src/api/internal.deckhouse.io/v1alpha1.
const (
	PayloadAPIVersion = "internal.deckhouse.io/v1alpha1"

	NodeConfigKind         = "NodeConfig"
	ControlPlaneConfigKind = "ControlPlaneConfig"
)

// ObjectMeta is the metadata dhctl emits for the payload documents. Only the
// fields the agent reads are rendered: a full metav1.ObjectMeta would add a
// "creationTimestamp: null" line for nothing.
type ObjectMeta struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
}

// NodeConfig is the document written to /config/nodeconfig.yaml.
type NodeConfig struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Metadata   ObjectMeta `json:"metadata"`
	Spec       NodeSpec   `json:"spec"`
}

// NodeSpec carries only the fields the first master needs. The agent's own type
// has more; anything omitted here simply keeps its default on the node.
type NodeSpec struct {
	NodeName           string           `json:"nodeName"`
	OSImage            string           `json:"osImage"`
	Storage            Disk             `json:"storage,omitempty"`
	Extensions         []Extension      `json:"extensions,omitempty"`
	Kernel             Kernel           `json:"kernel,omitempty"`
	Network            Network          `json:"network,omitempty"`
	Kubelet            Kubelet          `json:"kubelet,omitempty"`
	ContainerRuntime   ContainerRuntime `json:"containerRuntime,omitempty"`
	APIServerEndpoints []string         `json:"apiServerEndpoints,omitempty"`
	UpdatePolicy       UpdatePolicy     `json:"updatePolicy,omitempty"`
	Registry           *Registry        `json:"registry,omitempty"`
}

// Disk picks one whole block device, either by path or by attributes. The same
// shape names the system disk in NodeConfig and the control-plane state disk in
// ControlPlaneConfig.
type Disk struct {
	Device       string        `json:"device,omitempty"`
	DiskSelector *DiskSelector `json:"diskSelector,omitempty"`
	Wipe         bool          `json:"wipe,omitempty"`
}

// DiskSelector matches a disk by attributes. Only Size is used here: dhctl
// builds the payload before the VM exists, so no serial or device path is known
// yet.
type DiskSelector struct {
	Size string `json:"size,omitempty"`
}

// Registry is the node's own path to the registry: it pulls the control-plane
// images and the system extensions directly, without the in-cluster
// registry-packages-proxy that does not exist yet during bootstrap.
type Registry struct {
	Address string `json:"address"`
	Path    string `json:"path"`
	Scheme  string `json:"scheme,omitempty"`
	CA      string `json:"ca,omitempty"`
	Auth    string `json:"auth,omitempty"`
}

// Extension is a signed verity sysext merged onto the read-only root.
type Extension struct {
	Name        string `json:"name"`
	Digest      string `json:"digest"`
	RequestedBy string `json:"requestedBy,omitempty"`
}

// Kernel holds sysctl settings applied before kubelet starts.
type Kernel struct {
	Sysctl map[string]string `json:"sysctl,omitempty"`
}

// Network holds the hostname and the interfaces the node brings up.
type Network struct {
	Hostname   string             `json:"hostname,omitempty"`
	DNS        *DNS               `json:"dns,omitempty"`
	Interfaces []NetworkInterface `json:"interfaces,omitempty"`
}

// DNS is the resolver configuration.
type DNS struct {
	Servers []string `json:"servers,omitempty"`
	Search  []string `json:"search,omitempty"`
}

// NetworkInterface describes a single NIC.
type NetworkInterface struct {
	Name      string   `json:"name"`
	DHCP      bool     `json:"dhcp"`
	Addresses []string `json:"addresses,omitempty"`
	Gateway   string   `json:"gateway,omitempty"`
}

// Kubelet configures the node's kubelet.
type Kubelet struct {
	ClusterDomain         string            `json:"clusterDomain,omitempty"`
	ClusterDNS            []string          `json:"clusterDNS,omitempty"`
	MaxPods               int               `json:"maxPods,omitempty"`
	ContainerLogMaxSize   string            `json:"containerLogMaxSize,omitempty"`
	ContainerLogMaxFiles  int               `json:"containerLogMaxFiles,omitempty"`
	RegisterWithTaints    []Taint           `json:"registerWithTaints,omitempty"`
	ExternalCloudProvider bool              `json:"externalCloudProvider,omitempty"`
	NodeLabels            map[string]string `json:"nodeLabels,omitempty"`
	// ServerTLSBootstrap is a pointer so that the explicit "false" the first
	// master needs survives marshalling: nobody can approve its serving CSR
	// until Deckhouse is installed.
	ServerTLSBootstrap  *bool                `json:"serverTLSBootstrap,omitempty"`
	NodeIP              string               `json:"nodeIP,omitempty"`
	ResourceReservation *ResourceReservation `json:"resourceReservation,omitempty"`
}

// ResourceReservation controls how much of the node is kept for the system.
type ResourceReservation struct {
	Mode string `json:"mode"`
}

// Taint is a Kubernetes taint applied while kubelet registers the node.
type Taint struct {
	Key    string `json:"key"`
	Value  string `json:"value,omitempty"`
	Effect string `json:"effect"`
}

// ContainerRuntime configures containerd.
type ContainerRuntime struct {
	SandboxImage           string `json:"sandboxImage,omitempty"`
	MaxConcurrentDownloads int    `json:"maxConcurrentDownloads,omitempty"`
}

// UpdatePolicy controls how and when the node updates itself.
type UpdatePolicy struct {
	Mode   string       `json:"mode,omitempty"`
	Window UpdateWindow `json:"window,omitempty"`
}

// UpdateWindow is the maintenance window updates are allowed in.
type UpdateWindow struct {
	From string   `json:"from,omitempty"`
	To   string   `json:"to,omitempty"`
	Days []string `json:"days,omitempty"`
}

// ControlPlaneConfig is the document written to /config/controlplane.yaml. The
// node issues every leaf certificate and kubeconfig itself on top of the CA
// bundle carried here, then lays the manifests down and waits for its own
// apiserver.
type ControlPlaneConfig struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   ObjectMeta       `json:"metadata"`
	Spec       ControlPlaneSpec `json:"spec"`
}

// ControlPlaneSpec is the desired state of the node's control plane.
type ControlPlaneSpec struct {
	// Bootstrap marks the very first control-plane node: the one that has to
	// create the initial cluster objects nobody else can create yet.
	Bootstrap bool `json:"bootstrap"`
	// Disk is the block device the control-plane state lives on
	// (/var/lib/etcd and /etc/kubernetes). It belongs here rather than in
	// NodeConfig because it only means anything on a control-plane node.
	Disk Disk `json:"disk,omitempty"`
	// CA maps a path relative to /etc/kubernetes/pki to its PEM contents.
	CA     map[string]string  `json:"ca"`
	Params ControlPlaneParams `json:"params"`
	// ExtraFiles maps a path relative to
	// /etc/kubernetes/deckhouse/extra-files to its contents.
	ExtraFiles map[string]string `json:"extraFiles,omitempty"`
	// Manifests maps a path relative to /etc/kubernetes/manifests to the
	// rendered static-pod manifest.
	Manifests map[string]string `json:"manifests"`
}

// ControlPlaneParams are the cluster-wide settings the node needs to issue its
// own certificates and kubeconfigs.
type ControlPlaneParams struct {
	ClusterDomain     string `json:"clusterDomain"`
	ServiceSubnetCIDR string `json:"serviceSubnetCIDR"`
	// EncryptionAlgorithm is empty when the cluster does not pin one; the node
	// then falls back to the PKI library default.
	EncryptionAlgorithm string `json:"encryptionAlgorithm"`
}
