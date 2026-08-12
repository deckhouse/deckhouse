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

	// nodeConfigUIDLabel names the NodeConfig an approval was issued for. Node
	// name plus generation is not identity: a recreated config counts from 1
	// again and matched a day-old operation for the object before it.
	nodeConfigUIDLabel = "node-manager.deckhouse.io/node-config-uid"

	// kubeSystemNS and cloudInstanceManagerNS hold the objects the rendered
	// config is built from.
	kubeSystemNS           = "kube-system"
	cloudInstanceManagerNS = "d8-cloud-instance-manager"

	// imagesDigestsConfigMapName carries the digests of every image the release
	// ships, including the system extensions an immutable node runs.
	// bashible-apiserver consumes the same ConfigMap.
	imagesDigestsConfigMapName = "bashible-apiserver-files"
	imagesDigestsKey           = "images_digests.json"

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

	// pauseDigestGroup and pauseDigestName locate the pause image in the digest
	// map. The group is a key in that map, not a path segment: every Deckhouse
	// image lives in one repository and is addressed by digest alone.
	pauseDigestGroup = "common"
	pauseDigestName  = "pause"

	// clusterConfigSecretName holds the cluster domain and pod subnet layout.
	clusterConfigSecretName = "d8-cluster-configuration"
	clusterConfigKey        = "cluster-configuration.yaml"

	// defaultClusterDomain is what ClusterConfiguration defaults clusterDomain to.
	defaultClusterDomain = "cluster.local"

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

	// apiserverPort is where the node-local API proxy forwards to.
	apiserverPort = 6443

	// containerdExtension, kubeletExtension, cniExtension and nodeletExtension
	// are the system extensions every immutable node runs.
	containerdExtension = "containerd"
	kubeletExtension    = "kubelet"
	cniExtension        = "kubernetes-cni"
	nodeletExtension    = "nodelet"

	// nodeletSysextImage is the agent's image in images_digests.json. Unlike the
	// other three it carries no version, so it is read by exact key.
	nodeletSysextImage = "nodeletSysext"

	// platformExtensionRequestedBy names the module, not the writing component:
	// dhctl and this controller write the same three extensions, so the field
	// must not differ between nodes. Keep in step with dhctl/pkg/immutable/digests.go.
	platformExtensionRequestedBy = "node-manager"

	// nerRequestedByPrefix qualifies an extension that came from a
	// NodeExtensionRequest, distinguishing it from the platform marker above.
	nerRequestedByPrefix = "NodeExtensionRequest/"

	// resourceReservationModeAuto and resourceReservationModeStatic are the
	// NodeGroup kubeReserved modes this render reasons about; Static has no
	// counterpart on an immutable node, and Off only ever passes through.
	resourceReservationModeAuto   = "Auto"
	resourceReservationModeStatic = "Static"

	// registryPackagesDigestsKey is the module the sysext images are built in.
	registryPackagesDigestsKey = "registrypackages"

	// phaseReady is what the node reports once it has reconciled the spec it
	// was given.
	phaseReady = "Ready"

	// extensionStateReady and extensionStateFailed mirror the agent's enum for
	// NodeConfig.status.extensions[].state; the third value, Pending, means the
	// node is still working and counts as neither outcome.
	extensionStateReady  = "Ready"
	extensionStateFailed = "Failed"

	// kubernetesLabelNamespace and k8sLabelNamespace are the label namespaces a
	// node may not put itself into, beyond kubeletAllowedLabels below.
	kubernetesLabelNamespace = "kubernetes.io"
	k8sLabelNamespace        = "k8s.io"

	// kubeletLabelNamespace and nodeLabelNamespace are the two prefixes inside
	// them that kubelet does accept from a node.
	kubeletLabelNamespace = "kubelet.kubernetes.io"
	nodeLabelNamespace    = "node.kubernetes.io"

	// cgroupLabel tells the cluster which cgroup layout the node runs;
	// cgroupV2Value is the only answer an olcedar node has. Read by
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

	// disruptionRequiredCondition is how the agent says it cannot apply the
	// config it was given without interrupting the node.
	disruptionRequiredCondition = "DisruptionRequired"

	// configurationAppliedCondition is how the agent says whether the node is
	// running the spec that was published to it, as opposed to a rolled-back,
	// quarantined or half-applied one.
	configurationAppliedCondition = "ConfigurationApplied"
)

// osImageNameAndTag is the olcedar image the node boots from, pinned to a tag.
// The repository comes from the cluster's registry secret (same as dhctl does);
// hardcoding a public registry would break air-gapped clusters on day-2 render.
const osImageNameAndTag = "olcedar:v0.1"

// systemDiskSelectorSize is the fallback diskSelector when the provisioner
// named no disk: excludes cloud-init/config drives (megabytes) but nothing
// else. A machine with several real disks needs a provisioner that names one.
const systemDiskSelectorSize = ">=2Gi"

// defaultPodSubnetNodeCIDRPrefix is the slice of the pod subnet a node gets
// when ClusterConfiguration names none (the default bashible reads too).
const defaultPodSubnetNodeCIDRPrefix = 24

// Defaults mirroring the NodeConfig CRD field defaults, applied in render so
// the bootstrap file path (which bypasses API-server CRD defaulting) produces
// the same values as a day-2 object.
const (
	// maxPodsCeiling is what the agent's schema accepts (Maximum=1000), which is
	// the top of the bashible ladder too, so only an operator's own number can
	// still reach it.
	maxPodsCeiling                = 1000
	defaultContainerLogMaxSize    = "50Mi"
	defaultContainerLogMaxFiles   = 4
	defaultMaxConcurrentDownloads = 3
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
