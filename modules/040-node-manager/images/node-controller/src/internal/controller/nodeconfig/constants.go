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

import (
	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

const (
	controllerName = "node-config"

	// allRequestName fans a NodeGroup change out to every node of the group.
	allRequestName = "__all__"

	nodeGroupNameLabel = nodecommon.NodeGroupLabel

	// managedByLabel marks the NodeConfig objects this controller owns, so a
	// leftover object of a deleted node can be found and removed.
	managedByLabel = "node-manager.deckhouse.io/managed-by"
	managedByValue = "node-controller"

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

	// containerdExtension, kubeletExtension and cniExtension are the system
	// extensions every immutable node runs.
	containerdExtension = "containerd"
	kubeletExtension    = "kubelet"
	cniExtension        = "kubernetes-cni"

	// platformExtensionRequestedBy records who asked for a platform extension.
	// It names the module rather than whichever component wrote the object,
	// because two of them write the same three extensions — the installer into
	// the first master's payload, this controller into every node afterwards —
	// and a field that names a different author on one node than on the rest is
	// a difference nobody can act on. Keep it in step with dhctl's copy
	// (dhctl/pkg/immutable/nodeconfig.go).
	platformExtensionRequestedBy = "node-manager"

	// nerRequestedByPrefix qualifies an extension that came from a
	// NodeExtensionRequest. The field also carries the platform marker above, so
	// a bare name leaves a reader guessing which of the two kinds they are
	// looking at — and which object to go read.
	nerRequestedByPrefix = "NodeExtensionRequest/"

	// resourceReservationModeAuto and resourceReservationModeStatic are the
	// NodeGroup kubeReserved modes this render has to reason about: Auto is the
	// default on both sides, and Static has no counterpart on an immutable node.
	// Off needs no constant — it only ever passes through.
	resourceReservationModeAuto   = "Auto"
	resourceReservationModeStatic = "Static"

	// registryPackagesDigestsKey is the module the sysext images are built in.
	registryPackagesDigestsKey = "registrypackages"

	// phaseReady is what the node reports once it has reconciled the spec it
	// was given.
	phaseReady = "Ready"

	// extensionStateReady and extensionStateFailed are the per-extension states a
	// node publishes in NodeConfig.status.extensions[].state. They mirror the
	// agent's own enum; its third value, Pending, is a node still working and
	// counts as neither outcome.
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

	// cgroupLabel is how a node tells the cluster which cgroup layout it runs,
	// and cgroupV2Value is the only answer an olcedar node has. node-manager
	// reads the label off the Node to decide whether the node can run containerd
	// v2 (modules/040-node-manager/hooks/cntrd_v2_support.go); the installer
	// writes the same pair into the first master's payload.
	cgroupLabel   = "node.deckhouse.io/cgroup"
	cgroupV2Value = "cgroup2fs"

	// controlPlaneRoleLabel marks a node that runs the control plane. Such a
	// node was provisioned from an installer payload rather than from a
	// rendered NodeConfig, and parts of that payload cannot be reproduced here.
	controlPlaneRoleLabel = "node-role.kubernetes.io/control-plane"

	// operationNodeLabel names the node an operation was created for; shared with
	// the reconciler (nodeoperation) so the lookup contract cannot drift.
	operationNodeLabel = v1alpha1.NodeOperationNodeLabel

	// disruptionRequiredCondition is how the agent says it cannot apply the
	// config it was given without interrupting the node.
	disruptionRequiredCondition = "DisruptionRequired"

	// configurationAppliedCondition is how the agent says whether the node is
	// running the spec that was published to it, as opposed to a rolled-back,
	// quarantined or half-applied one.
	configurationAppliedCondition = "ConfigurationApplied"
)

// osImageNameAndTag is the olcedar image the node boots from, pinned to a
// known-good build (a tag, not an @digest). Only the name and the tag are
// constant: the repository in front of them comes from the cluster's own
// registry secret — the same source spec.registry is built from — and dhctl
// composes the first master's copy the same way (dhctl/pkg/immutable/
// nodeconfig.go). Naming the public registry here instead rewrote that master's
// spec.osImage on its first day-2 render, replacing the registry the cluster was
// installed from with one an air-gapped cluster cannot reach.
const osImageNameAndTag = "olcedar:v0.1"

// systemDiskSelectorSize is the diskSelector this controller renders for a
// node whose provisioner named no disk. It cannot name the disk — a NodeGroup
// has no disk field — so it says the one true thing it can: any real disk, as
// opposed to the attach junk (cloud-init drives and config drives are
// megabytes; no real system disk is). The machines this reaches have exactly
// one real disk, which makes the selector unambiguous; a machine with several
// needs a provisioner that names one, the boot path refuses to guess.
const systemDiskSelectorSize = ">=2Gi"

// How many pods a node advertises for each slice of the pod subnet, and the
// prefix assumed when the cluster configuration names none. The brackets are
// bashible's (candi/bashible/common-steps/all/064_configure_kubelet.sh.tpl), so
// an immutable node and a bashible node beside it advertise the same capacity.
const (
	defaultPodSubnetNodeCIDRPrefix = 24
	maxPodsPerNodeCIDR24           = 120
	maxPodsPerNodeCIDR23           = 250
	maxPodsPerNodeCIDR22           = 500
	maxPodsPerNodeCIDR21           = 1000
)

// Defaults mirroring the NodeConfig CRD field defaults. render applies them so
// the bootstrap file path — which marshals the spec to a file rather than
// creating it through the API server, where CRD defaulting runs — produces the
// same values as a day-2 object.
const (
	// maxPodsCeiling is what the agent's schema accepts (Maximum=500), so it
	// bounds both an operator's number and the one derived from the pod subnet.
	maxPodsCeiling                = 500
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
