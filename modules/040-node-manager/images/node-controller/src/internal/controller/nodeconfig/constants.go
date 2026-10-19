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

package nodeconfig

const (
	controllerName = "node-config"

	// allRequestName fans a NodeGroup change out to every node of the group.
	allRequestName = "__all__"

	// managedByLabel marks the NodeConfig objects this controller owns, so a
	// leftover object of a deleted node can be found and removed.
	managedByLabel = "node-manager.deckhouse.io/managed-by"
	managedByValue = "node-controller"

	// fieldManager owns, by server-side apply, every NodeConfig spec field this
	// controller renders (controller.go applySpec, ownership.go takeOver).
	fieldManager = "node-controller"

	kubeSystemNS           = "kube-system"
	cloudInstanceManagerNS = "d8-cloud-instance-manager"

	// imagesDigestsConfigMapName carries the digests of every image the release
	// ships, including the system extensions an immutable node runs.
	// bashible-apiserver consumes the same ConfigMap.
	imagesDigestsConfigMapName = "bashible-apiserver-files"
	imagesDigestsKey           = "images_digests.json"

	// platformTemplatesConfigMapName carries the candi templates helm ships with the
	// release (templates/bashible/bootstrap-templates-cm.yaml); platformDocumentKey
	// is candi/nodelet/nodeconfig-mutable.yaml.tpl in it.
	platformTemplatesConfigMapName = "bashible-bootstrap-templates"
	platformDocumentKey            = "nodeconfig-mutable.yaml.tpl"

	// registryPackagesProxyTokenSecret authenticates the node against the
	// registry packages proxy it pulls system extensions from.
	registryPackagesProxyTokenSecret = "registry-packages-proxy-token"
	registryPackagesProxyTokenKey    = "token"

	// deckhouseRegistrySecret describes the registry the cluster was installed
	// from: where its images live and how to authenticate. It is the same secret
	// every Deckhouse pod carries as an imagePullSecret.
	d8SystemNS              = "d8-system"
	deckhouseRegistrySecret = "deckhouse-registry"
	registryAddressKey      = "address"
	registryPathKey         = "path"
	registrySchemeKey       = "scheme"
	registryCAKey           = "ca"
	registryImagesKey       = "imagesRegistry"
	registryDockerConfigKey = ".dockerconfigjson"

	// registryConfig* name the registry module's resolved configuration, which
	// carries the upstream that module fetches from. Read as unstructured: the
	// kind is that module's CRD, absent from a cluster that does not run it.
	registryConfigAPIVersion = "deckhouse.io/v1alpha1"
	registryConfigKind       = "RegistryConfig"
	registryConfigName       = "registry"

	// registryBashibleConfigSecret is what the registry module writes for
	// bashible; the presence of its "agent" key is the one signal that
	// containerd's registry.d belongs to that module's node agent rather than to
	// nodelet (go_lib/registry/models/bashible/config.go on rewrite-registry).
	// Absent on a cluster whose registry module never said so, which is not an
	// error but the ordinary answer.
	registryBashibleConfigSecret = "registry-bashible-config"
	registryBashibleConfigKey    = "config"
	registryBashibleAgentKey     = "agent"

	// registryAgentStaticPodName is the NodeStaticPodRequest the registry module
	// publishes to put its node agent on an Immutable node, and so the name of the
	// manifest in a node's spec.staticPods.
	//
	// The platform has to know it, because on such a node that object is the only way
	// the agent can arrive: bashible, which installs it everywhere else, never runs
	// here. So it is what says whether the node actually HAS the agent it is being
	// asked to hand containerd's registry.d to. See renderContainerRuntime.
	registryAgentStaticPodName = "registry-agent"

	clusterConfigSecretName = "d8-cluster-configuration"
	clusterConfigKey        = "cluster-configuration.yaml"

	// staticConfigSecretName and providerConfigSecretName carry the network the
	// cluster addresses its nodes in. A cluster has one of the two, never both,
	// so neither read may fail the pass when its secret is absent.
	staticConfigSecretName   = "d8-static-cluster-configuration"
	staticConfigKey          = "static-cluster-configuration.yaml"
	providerConfigSecretName = "d8-provider-cluster-configuration"
	providerConfigKey        = "cloud-provider-cluster-configuration.yaml"

	// dnsAppLabel finds the in-cluster DNS service, and kubeDNSServiceName is the
	// one that wins when several carry the label.
	dnsAppLabel        = "k8s-app"
	kubeDNSServiceName = "kube-dns"

	// apiServerEndpointSliceNS, apiServerEndpointSliceName and apiServerPortName
	// locate the EndpointSlice the API server publishes its own addresses in.
	apiServerEndpointSliceNS   = "default"
	apiServerEndpointSliceName = "kubernetes"
	apiServerPortName          = "https"

	// clusterCAConfigMap carries the cluster CA every ServiceAccount is given.
	// The node needs it after every reboot: kubelet verifies the API server
	// against it and refuses to start without the file.
	clusterCAConfigMap = "kube-root-ca.crt"
	clusterCAKey       = "ca.crt"

	apiserverPort = 6443

	containerdExtension = "containerd"
	kubeletExtension    = "kubelet"
	cniExtension        = "kubernetes-cni"
	nodeletExtension    = "nodelet"
	// guestAgentExtension goes on every Engine node, hardware included: the node
	// decides at boot whether to start the agent, because a unit that systemd
	// skips over an unmet condition is what stops a rollout dead. See the engine
	// repository, docs/qemu-guest-agent.md.
	guestAgentExtension = "qemu-guest-agent"

	// nodeletSysextImage is the agent's image in images_digests.json. Unlike the
	// other three it carries no version, so it is read by exact key.
	nodeletSysextImage = "nodeletSysext"
	// guestAgentSysextImage is the QEMU guest agent's image there, versionless
	// for the same reason and read the same way.
	guestAgentSysextImage = "qemuGuestAgentSysext"

	// resourceReservationModeAuto and resourceReservationModeStatic are the
	// NodeGroup kubeReserved modes this render reasons about; Static has no
	// counterpart on an immutable node, and Off only ever passes through.
	resourceReservationModeAuto   = "Auto"
	resourceReservationModeStatic = "Static"

	// registryPackagesDigestsKey is the module the sysext images are built in.
	registryPackagesDigestsKey = "registrypackages"

	phaseReady = "Ready"

	// extensionStateReady, extensionStateFailed and extensionStatePending mirror the
	// agent's enum for NodeConfig.status.extensions[].state; Pending means the node is
	// still working, or waits for the update window, and counts as neither outcome.
	extensionStateReady   = "Ready"
	extensionStateFailed  = "Failed"
	extensionStatePending = "Pending"

	// staticPodStateWritten and staticPodStateFailed are the whole enum of
	// status.staticPods[].state in crds/nodeconfig.yaml: the node holds the file the spec
	// asked for, or it does not.
	staticPodStateWritten = "Written"
	staticPodStateFailed  = "Failed"

	// cgroupLabel tells the cluster which cgroup layout the node runs;
	// cgroupV2Value is the only answer a Deckhouse Engine node has. Read by
	// hooks/cntrd_v2_support.go; the installer writes the same pair.
	cgroupLabel   = "node.deckhouse.io/cgroup"
	cgroupV2Value = "cgroup2fs"

	// controlPlaneRoleLabel marks a node that runs the control plane. Such a
	// node was provisioned from an installer payload rather than from a
	// rendered NodeConfig, and parts of that payload cannot be reproduced here.
	controlPlaneRoleLabel = "node-role.kubernetes.io/control-plane"

	// settingClampedEvent names the Warning event that reports a NodeGroup
	// setting the render could not carry over as written (recordClampedSettings).
	settingClampedEvent = "SettingClamped"

	// configurationPendingReason is ConfigurationApplied=False while the pass waits for
	// something outside the node; unknownFieldsReason is True with fields the agent does
	// not know and so does not apply.
	configurationPendingReason = "Pending"
	unknownFieldsReason        = "UnknownFields"

	// disruptionApprovalExhaustedEvent names the Warning that says the cluster
	// has stopped interrupting a node for a configuration revision.
	disruptionApprovalExhaustedEvent = "DisruptionApprovalExhausted"
)

// nodeManagerDigestsKey and osImageName locate the Deckhouse Engine image of this
// release. Pinned by digest, not by tag: the node decides a rootfs update by
// comparing it with what it recorded at install, and a tag that moves lies.
const (
	nodeManagerDigestsKey = "nodeManager"
	osImageName           = "engine"
)

// systemDiskSelectorSize is the fallback diskSelector when the provisioner
// named no disk: excludes cloud-init/config drives (megabytes) but nothing
// else. A machine with several real disks needs a provisioner that names one.
const systemDiskSelectorSize = ">=2Gi"

// defaultPodSubnetNodeCIDRPrefix is the slice of the pod subnet a node gets
// when ClusterConfiguration names none (the default bashible reads too).
const defaultPodSubnetNodeCIDRPrefix = 24

// maxPodsCeiling is what the agent's schema accepts (Maximum=1000), which is the
// top of the bashible ladder too, so only an operator's own number can reach it.
const maxPodsCeiling = 1000

// reservedLabelNamespaces are the label namespaces a node may not put itself
// into; kubeletLabelNamespaces are the ones inside them kubelet does accept
// (kubernetes/pkg/kubelet/apis/well_known_labels.go).
var (
	reservedLabelNamespaces = []string{"kubernetes.io", "k8s.io"}
	kubeletLabelNamespaces  = []string{"kubelet.kubernetes.io", "node.kubernetes.io"}
)

// kubeletAllowedLabels is the set kubelet accepts on --node-labels despite
// living in a reserved namespace (kubernetes/pkg/kubelet/apis/well_known_labels.go).
var kubeletAllowedLabels = map[string]struct{}{
	"beta.kubernetes.io/arch":                  {},
	"beta.kubernetes.io/instance-type":         {},
	"beta.kubernetes.io/os":                    {},
	"failure-domain.beta.kubernetes.io/region": {},
	"failure-domain.beta.kubernetes.io/zone":   {},
	"kubernetes.io/arch":                       {},
	"kubernetes.io/hostname":                   {},
	"kubernetes.io/os":                         {},
	"node.kubernetes.io/instance-type":         {},
	"topology.kubernetes.io/region":            {},
	"topology.kubernetes.io/zone":              {},
}
