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

// Package immutable builds the cloud-init payloads that boot control-plane
// nodes on the immutable olcedar OS (no sshd, no bashible). No cluster key travels
// in a payload: the node generates the PKI; dhctl gets kubeconfig via handoff.go.
package immutable

// objectMeta is the metadata dhctl emits for the payload documents. Only the
// fields the agent reads are rendered: a full metav1.objectMeta would add a
// "creationTimestamp: null" line for nothing.
type objectMeta struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
}

// nodeConfig is the document written to /config/nodeconfig.yaml.
type nodeConfig struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Metadata   objectMeta `json:"metadata"`
	Spec       nodeSpec   `json:"spec"`
}

// osImage names the image the node must run. Mirrors the agent contract —
// internal/config/types.go in the nodelet repository — and the initramfs one,
// images/init/src/0.1/nodeconfig.go: both parse this document, and a shape they
// disagree with fails the parse and drops the node into an emergency shell.
type osImage struct {
	Digest         string `json:"digest"`
	Repository     string `json:"repository,omitempty"`
	AdditionalPath string `json:"additionalPath,omitempty"`
}

// nodeSpec carries only the fields dhctl emits. The contract lives in the
// agent's own type, which has more; anything absent here keeps its default on
// the node.
type nodeSpec struct {
	NodeName           string           `json:"nodeName"`
	OSImage            osImage          `json:"osImage"`
	Storage            storage          `json:"storage,omitempty"`
	Extensions         []extension      `json:"extensions,omitempty"`
	Kernel             kernel           `json:"kernel,omitempty"`
	Network            network          `json:"network,omitempty"`
	Kubelet            kubelet          `json:"kubelet,omitempty"`
	ContainerRuntime   containerRuntime `json:"containerRuntime,omitempty"`
	APIServerEndpoints []string         `json:"apiServerEndpoints,omitempty"`
	UpdatePolicy       updatePolicy     `json:"updatePolicy,omitempty"`
	Registry           *registrySpec    `json:"registry,omitempty"`
}

// storage is what the node is told about its disks. Neither disk is named by
// path — the document is rendered before the machine exists — so each is
// described by attributes matched against what the machine actually has.
type storage struct {
	// DiskSelector names the disk the OS is installed onto. Read by the
	// initramfs; the agent parses the same document and would reject an
	// unknown field, so this has to match the NodeConfig CRD exactly.
	DiskSelector *diskSelector `json:"diskSelector,omitempty"`
	Mounts       []mount       `json:"mounts,omitempty"`
}

// diskSelector picks a whole disk by its attributes.
type diskSelector struct {
	Size string `json:"size,omitempty"`
}

// mount is one filesystem the node puts in place before kubelet starts.
type mount struct {
	Name              string             `json:"name"`
	PartitionSelector *partitionSelector `json:"partitionSelector,omitempty"`
	// BindTo is the path the filesystem is mounted at, when it is not the node's
	// to choose: /var/lib/etcd is what the etcd static pod carries as a hostPath.
	BindTo string `json:"bindTo,omitempty"`
	// Mode is the mode of the filesystem root, octal. A freshly made ext4 has its
	// root at 0755, which is a mode etcd refuses to start on.
	Mode string `json:"mode,omitempty"`
}

// partitionSelector names a device by what it looks like rather than by path.
type partitionSelector struct {
	Size string `json:"size,omitempty"`
	// Blank makes whole disks selectable, and only the ones carrying nothing.
	Blank bool `json:"blank,omitempty"`
}

// registrySpec is the node's own path to the registry: it pulls the control-plane
// images and the system extensions directly, without the in-cluster
// registry-packages-proxy that does not exist yet during bootstrap.
type registrySpec struct {
	Address string `json:"address"`
	Path    string `json:"path"`
	Scheme  string `json:"scheme,omitempty"`
	CA      string `json:"ca,omitempty"`
	Auth    string `json:"auth,omitempty"`
}

// extension is a signed verity sysext merged onto the read-only root.
type extension struct {
	Name        string `json:"name"`
	Digest      string `json:"digest"`
	RequestedBy string `json:"requestedBy,omitempty"`
}

// kernel holds sysctl settings applied before kubelet starts.
type kernel struct {
	Sysctl map[string]string `json:"sysctl,omitempty"`
}

// network holds the hostname and the interfaces the node brings up.
type network struct {
	Hostname   string             `json:"hostname,omitempty"`
	Interfaces []networkInterface `json:"interfaces,omitempty"`
}

// networkInterface describes a single NIC.
type networkInterface struct {
	Name      string   `json:"name"`
	DHCP      bool     `json:"dhcp"`
	Addresses []string `json:"addresses,omitempty"`
	Gateway   string   `json:"gateway,omitempty"`
}

// kubelet configures the node's kubelet.
type kubelet struct {
	ClusterDomain string   `json:"clusterDomain,omitempty"`
	ClusterDNS    []string `json:"clusterDNS,omitempty"`
	MaxPods       int      `json:"maxPods,omitempty"`
	// KubernetesVersion decides kubelet's feature gates on the node.
	KubernetesVersion     string            `json:"kubernetesVersion,omitempty"`
	ContainerLogMaxSize   string            `json:"containerLogMaxSize,omitempty"`
	ContainerLogMaxFiles  int               `json:"containerLogMaxFiles,omitempty"`
	ExternalCloudProvider bool              `json:"externalCloudProvider,omitempty"`
	NodeLabels            map[string]string `json:"nodeLabels,omitempty"`
	// ServerTLSBootstrap is a pointer so that the explicit "false" the first
	// master needs survives marshalling: nobody can approve its serving CSR
	// until Deckhouse is installed.
	ServerTLSBootstrap  *bool                `json:"serverTLSBootstrap,omitempty"`
	ResourceReservation *resourceReservation `json:"resourceReservation,omitempty"`
	// CACert (cluster CA, base64) and BootstrapToken are empty for the first
	// master and required for joining nodes. A non-empty CACert also marks a
	// join payload: the node refuses to bootstrap a new cluster with one.
	CACert         string `json:"caCert,omitempty"`
	BootstrapToken string `json:"bootstrapToken,omitempty"`
}

// resourceReservation controls how much of the node is kept for the system.
type resourceReservation struct {
	Mode string `json:"mode"`
}

// containerRuntime configures containerd.
type containerRuntime struct {
	SandboxImage           string `json:"sandboxImage,omitempty"`
	MaxConcurrentDownloads int    `json:"maxConcurrentDownloads,omitempty"`
}

// updatePolicy controls how and when the node updates itself.
type updatePolicy struct {
	Mode string `json:"mode,omitempty"`
}

// controlPlaneConfig is the document written to /config/controlplane.yaml. The
// node generates the cluster PKI, writes the manifests carried here and brings
// its own apiserver up.
type controlPlaneConfig struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   objectMeta       `json:"metadata"`
	Spec       controlPlaneSpec `json:"spec"`
}

// controlPlaneSpec is the desired state of the node's control plane.
type controlPlaneSpec struct {
	// Bootstrap marks the very first control-plane node: the one that has to
	// create the initial cluster objects nobody else can create yet.
	Bootstrap bool `json:"bootstrap"`
	// Cluster are the cluster-wide inputs behind the certificate SANs. Only what
	// the node still decides for itself is here: everything the components are
	// started with has already been rendered into Manifests.
	Cluster clusterParamsSpec `json:"cluster"`
	// Manifests are the static pods, rendered, in the order they must be
	// written. The node writes them as they are, with its own address in place
	// of nodeAddressPlaceholder.
	Manifests []renderedFile `json:"manifests"`
	// ExtraFiles are the files the manifests reference by path — the ones that
	// live in /etc/kubernetes/deckhouse/extra-files.
	ExtraFiles []renderedFile `json:"extraFiles,omitempty"`
	// handoff is the one-shot channel dhctl collects the admin kubeconfig
	// through once the node has a control plane.
	Handoff handoff `json:"handoff"`
}

// renderedFile is one file the node writes without reading it.
type renderedFile struct {
	// Name is the file name, not a path: the directory is the node's to choose.
	Name    string `json:"name"`
	Content string `json:"content"`
}

// clusterParamsSpec are the cluster-wide settings the node needs to issue its
// own certificates.
type clusterParamsSpec struct {
	ClusterDomain     string `json:"clusterDomain"`
	ServiceSubnetCIDR string `json:"serviceSubnetCIDR"`
	// EncryptionAlgorithm is empty when the cluster does not pin one; the node
	// then falls back to the PKI library default.
	EncryptionAlgorithm string `json:"encryptionAlgorithm"`
	// CertSANs are the extra names and addresses the apiserver certificate has
	// to cover, the same list control-plane-manager keeps under the "cert-sans"
	// key of its config secret.
	CertSANs []string `json:"certSANs,omitempty"`
}

// handoff is the TLS material and the bearer token of the one-shot endpoint the
// node serves the admin kubeconfig on. The key protects one read of one file
// and is worthless after bootstrap; no cluster key travels in this document.
type handoff struct {
	Token      string `json:"token"`
	ServerCert string `json:"serverCert"`
	ServerKey  string `json:"serverKey"`
	// ClientCSR is the installer's request for a cluster client certificate.
	// The node signs it with the cluster CA; only the public certificate comes
	// back, while the private key never leaves the installer.
	ClientCSR string `json:"clientCSR"`
}
