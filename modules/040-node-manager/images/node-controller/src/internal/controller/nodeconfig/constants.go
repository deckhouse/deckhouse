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

	// clusterConfigSecretName holds the cluster domain and pod subnet layout.
	clusterConfigSecretName = "d8-cluster-configuration"
	clusterConfigKey        = "cluster-configuration.yaml"

	// dnsAppLabel finds the in-cluster DNS service.
	dnsAppLabel = "k8s-app"

	// apiserverPort is where the node-local API proxy forwards to.
	apiserverPort = 6443

	// containerdExtension, kubeletExtension and cniExtension are the system
	// extensions every immutable node runs.
	containerdExtension = "containerd"
	kubeletExtension    = "kubelet"
	cniExtension        = "kubernetes-cni"

	// registryPackagesDigestsKey is the module the sysext images are built in.
	registryPackagesDigestsKey = "registrypackages"
)

// defaultOSImage is the immutable OS image the node boots from.
//
// TODO: resolve this from the Deckhouse release channel once the OS image is
// published there; until then the image is pinned to a known-good build so the
// rest of the pipeline can be exercised end to end.
const defaultOSImage = "registry.deckhouse.io/deckhouse/olcedar@v0.1"
