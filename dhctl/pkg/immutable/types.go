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
// /config/controlplane.yaml, generates the whole cluster PKI itself — the CA
// included — renders the control-plane manifests, brings its own apiserver up
// and creates the first cluster objects.
//
// The payload therefore carries inputs, never artifacts: nothing secret about
// the cluster travels through cloud-init, which would otherwise leave the
// cluster CA keys in a Secret, in the infrastructure state and in the
// installer's cache. dhctl collects the admin kubeconfig afterwards from the
// one-shot handoff endpoint the node opens for it — see handoff.go.
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
	ClusterDomain string   `json:"clusterDomain,omitempty"`
	ClusterDNS    []string `json:"clusterDNS,omitempty"`
	MaxPods       int      `json:"maxPods,omitempty"`
	// KubernetesVersion decides kubelet's feature gates on the node.
	KubernetesVersion     string            `json:"kubernetesVersion,omitempty"`
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
// node generates the cluster PKI, renders the manifests and brings its own
// apiserver up from the inputs carried here.
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
	// Cluster are the cluster-wide inputs behind the certificate SANs and the
	// command line of every control-plane component.
	Cluster ClusterParams `json:"cluster"`
	// Images are the digests of the four static-pod images. They come from the
	// digest map of the release the installer was built from, which the node
	// cannot reach.
	Images ControlPlaneImages `json:"images"`
	// Handoff is the one-shot channel dhctl collects the admin kubeconfig
	// through once the node has a control plane.
	Handoff Handoff `json:"handoff"`
}

// ClusterParams are the cluster-wide settings the node needs to issue its own
// certificates and render the control-plane manifests.
type ClusterParams struct {
	ClusterDomain     string `json:"clusterDomain"`
	ServiceSubnetCIDR string `json:"serviceSubnetCIDR"`
	PodSubnetCIDR     string `json:"podSubnetCIDR"`
	// PodSubnetNodeCIDRPrefix is the per-node prefix length, e.g. "24".
	PodSubnetNodeCIDRPrefix string `json:"podSubnetNodeCIDRPrefix"`
	// KubernetesVersion is the minor version, e.g. "1.34". "Automatic" is
	// resolved before it gets here: the node has no default to fall back on.
	KubernetesVersion string `json:"kubernetesVersion"`
	// ClusterType is Cloud or Static. Without Cloud the controller manager
	// never gets --cloud-provider=external and never hands node lifecycle to
	// the cloud-controller-manager.
	ClusterType string `json:"clusterType"`
	// EncryptionAlgorithm is empty when the cluster does not pin one; the node
	// then falls back to the PKI library default.
	EncryptionAlgorithm string `json:"encryptionAlgorithm"`
	// CertSANs are the extra names and addresses the apiserver certificate has
	// to cover, the same list control-plane-manager keeps under the "cert-sans"
	// key of its config secret.
	CertSANs []string `json:"certSANs,omitempty"`
}

// ControlPlaneImages are the digests of the four static-pod images. The node
// prepends the registry address and path from NodeConfig.spec.registry to build
// the reference, which is why only the digest travels here.
type ControlPlaneImages struct {
	Etcd                  string `json:"etcd"`
	KubeAPIServer         string `json:"kubeApiserver"`
	KubeControllerManager string `json:"kubeControllerManager"`
	KubeScheduler         string `json:"kubeScheduler"`
}

// Handoff is the TLS material and the bearer token of the one-shot endpoint the
// node serves the admin kubeconfig on.
//
// The key here belongs to that endpoint alone and is worth nothing once the
// bootstrap is over: it protects one read of one file, and the endpoint closes
// after it. No cluster key ever travels in this document.
type Handoff struct {
	Token      string `json:"token"`
	ServerCert string `json:"serverCert"`
	ServerKey  string `json:"serverKey"`
	// ClientCSR is the installer's request for a cluster client certificate.
	// The node signs it with the cluster CA it generates, so what comes back
	// over the channel is a certificate — public — while the key it belongs to
	// never leaves the installer. That is what keeps the channel free of
	// anything worth stealing.
	ClientCSR string `json:"clientCSR"`
}
