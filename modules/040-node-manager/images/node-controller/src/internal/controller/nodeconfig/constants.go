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

	// dnsAppLabel finds the in-cluster DNS service.
	dnsAppLabel = "k8s-app"

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

	// registryPackagesDigestsKey is the module the sysext images are built in.
	registryPackagesDigestsKey = "registrypackages"

	// phaseReady is what the node reports once it has reconciled the spec it
	// was given.
	phaseReady = "Ready"

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

// defaultOSImage is the olcedar image the node boots from. It is pinned to a
// known-good build (a tag, not an @digest); resolving it from the Deckhouse
// release channel is deferred until the OS image is published there and is
// tracked outside the code.
const defaultOSImage = "registry.deckhouse.io/deckhouse/olcedar:v0.1"

// Defaults mirroring the NodeConfig CRD field defaults. render applies them so
// the bootstrap file path — which marshals the spec to a file rather than
// creating it through the API server, where CRD defaulting runs — produces the
// same values as a day-2 object.
const (
	defaultMaxPods                = 110
	defaultContainerLogMaxSize    = "50Mi"
	defaultContainerLogMaxFiles   = 4
	defaultSandboxImage           = "registry.k8s.io/pause:3.10"
	defaultMaxConcurrentDownloads = 3
)
