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

package immutable

// This file holds what dhctl and the node have to agree on: the payload
// documents, the bootstrap channel, and the names the node is addressed by.
// node-controller is a separate Go module, so the values it also knows are
// repeated here rather than imported; each such constant names its original.

// The payload documents. The on-node agent parses both with UnmarshalStrict, so
// every field name must match the agent's types
// (node-controller/src/api/internal.deckhouse.io/v1alpha1).
const (
	payloadAPIVersion = "internal.deckhouse.io/v1alpha1"

	// Mirrors node-controller/src/internal/controller/nodebootstrap/render.go,
	// which spells the same kinds out when it renders a day-2 payload.
	nodeConfigKind         = "NodeConfig"
	controlPlaneConfigKind = "ControlPlaneConfig"

	// nodeConfigPath is where the on-node loader reads the node config from. It
	// selects the entry by the "nodeconfig.yml"/"nodeconfig.yaml" suffix of the
	// path, so the name matters and the directory does not.
	// Mirrors node-controller/src/internal/controller/nodebootstrap/constants.go.
	nodeConfigPath = "/config/nodeconfig.yaml"

	// controlPlaneConfigPath names the payload entry. The node reads cloud-init
	// itself and picks the entry out by this name, so the path is a label and
	// the name is the contract.
	controlPlaneConfigPath = "/config/controlplane.yaml"
)

// systemTypeImmutable is the NodeGroup systemType that asks for a node dhctl
// cannot SSH into.
// Mirrors SystemTypeImmutable of node-controller/src/api/deckhouse.io/v1.
const systemTypeImmutable = "Immutable"

// The node's one-shot bootstrap channel: what it is doing, the credentials, and
// the confirmation that ends the handover.
const (
	// HandoffPort is the port the node's handoff endpoint listens on.
	HandoffPort = 50001

	statusPath    = "/bootstrap/status"
	handoffPath   = "/bootstrap/kubeconfig"
	collectedPath = "/bootstrap/collected"
)

// APIServerPort is where a control-plane node's own kube-apiserver listens.
const APIServerPort = 6443

// The labels the node registers itself with.
const (
	nodeTypeLabel = "node.deckhouse.io/type"
	cgroupLabel   = "node.deckhouse.io/cgroup"
)

// The system extensions an immutable node runs, and where their digests are
// looked up in images_digests.json.
const (
	containerdExtension = "containerd"
	cniExtension        = "kubernetes-cni"
	kubeletExtension    = "kubelet"

	// platformExtensionRequestedBy names the module that wants the extension, not
	// the process that wrote the file — which is why it stays "node-manager" when
	// node-controller re-renders this node.
	platformExtensionRequestedBy = "node-manager"

	// registryPackagesDigestsKey is the images_digests.json module the sysext
	// images are built in.
	registryPackagesDigestsKey = "registrypackages"

	// commonDigestsKey and pauseImageName locate the sandbox image. The
	// registry.k8s.io default of the nodeConfig CRD is unreachable from a node
	// that only talks to the Deckhouse registry.
	commonDigestsKey = "common"
	pauseImageName   = "pause"

	// controlPlaneDigestsKey is the images_digests.json module the control-plane
	// images are built in.
	controlPlaneDigestsKey = "controlPlaneManager"
)

// What the bootstrap keeps in the dhctl state cache between attempts.
const (
	// handoffCacheKey names the handoff material. A second bootstrap attempt has
	// to present the same token to a node that already booted with the first
	// payload, and verify the certificate that payload carried.
	handoffCacheKey = "immutable-control-plane-handoff"

	// collectedKubeconfigCacheKey records that the admin kubeconfig is on disk,
	// and where; a rerun reads it instead of dialing a closed channel. Written
	// before ConfirmCollected: a death in between must not leave no record of
	// the file.
	collectedKubeconfigCacheKey = "immutable-control-plane-collected-kubeconfig"
)
