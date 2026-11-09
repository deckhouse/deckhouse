/*
Copyright 2025 Flant JSC

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

package constant

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
)

const (
	// Host is the in-cluster registry address: the name every image reference, ModuleSource and
	// `deckhouse-registry` in the cluster is built from, and the name containerd keeps a host
	// directory for.
	//
	// A literal, and deliberately not assembled from any port below. It is a key, not an address:
	// nothing dials it — a container runtime is redirected to the node agent by a drop-in, and a
	// process translates it to ProxyHost first — so the `:5001` in it says nothing about where
	// anything listens. What it does say is recorded in every pod spec of every cluster, and a change
	// to it would rename every image the cluster runs: a rollout of everything, and on an air-gapped
	// cluster a pull of every image again, under a name nothing has cached. It used to be derived
	// from the one port constant there was, which made moving the store's listener a change to every
	// image reference as well.
	//
	// The name without the port is dialled by what reads the store with no agent to go through:
	// dhctl in the cluster (terraform-manager's infrastructure auto-converger and its state
	// exporter, in the pod network) and registry-packages-proxy on a master that has no agent yet.
	// They read the store at StorageHost — see there.
	Host = "registry.d8-system.svc:5001"

	// AgentPort is where the node agent answers, on the loopback of every node.
	//
	// Unchanged since the previous implementation's node-side proxy held the same address, and
	// unlike the store's ports it is a contract with processes that update on their own schedule —
	// Deckhouse itself, registry-packages-proxy, the static pod an immutable node runs — so it
	// stays where it is.
	AgentPort = 5001

	// StoragePort is where a replica of the in-cluster store serves on its node's address: what the
	// node agents pull through and what one replica replicates from another. Crosses between nodes,
	// so it lies in the 4200-4299 range the platform documents for host firewalls.
	//
	// Also the port of the registry an installation from a bundle puts on the first master before
	// the store exists. The two share it on purpose: the store's pod declares it, the bootstrap
	// registry's static pod declares it too, and so the scheduler will not place the store on that
	// master until the bootstrap registry is gone — the two would otherwise serve the same directory
	// at once.
	StoragePort = 4281

	// StorageWritePort is the store's second listener, the one that accepts a push: the serving one
	// is a pull-through cache, which refuses writes. Reached by the ingress controller from other
	// nodes, so it lies in the same range.
	StorageWritePort = 4283

	// LegacyStoragePort is where the previous implementation's node services served on a master,
	// and where the store served before it moved to StoragePort. Only that implementation's own
	// code uses it, and the registry controller, which recognizes a replica still running the
	// store's previous pod spec by it. The node agent that kept the number is a different
	// listener on a different address.
	LegacyStoragePort = 5001

	// LegacyStorageWritePort is the write listener of a replica on the store's previous pod spec,
	// the counterpart of LegacyStoragePort as StorageWritePort is of StoragePort.
	LegacyStorageWritePort = 5003

	Path       = "/system/deckhouse"
	PathRegexp = "^system/deckhouse"
	Scheme     = "https"

	// DropInRoot is the directory the container runtime is pointed at through its
	// `config_path`, and which the node agent owns.
	DropInRoot = "/etc/containerd/registry.d"

	// DropInDefaultHost is containerd's fallback directory: consulted for any registry
	// that has no directory of its own, which is how one drop-in covers every registry
	// and makes the node configuration static.
	DropInDefaultHost = "_default"

	// DropInHostsFile is the file name containerd reads inside a host directory.
	DropInHostsFile = "hosts.toml"

	// AuthSecretName is the one Secret holding every credential this module resolved.
	//
	// One object rather than one per registry, because the node agent has to be able to
	// read it: a single name can be granted by resourceNames, so a node's kubelet reaches
	// exactly this and nothing else. The keys are what the credentials belong to —
	// AuthKeyStorage for the in-cluster cache, AuthKeyUpstream for the primary upstream,
	// and the upstream's own name for anything declared as a RegistryUpstream.
	AuthSecretName = "registry-resolved-auth"

	// AuthKeyStorage keys the credentials for the in-cluster cache.
	AuthKeyStorage = "storage"

	// AuthKeyUpstream keys the credentials for the primary upstream.
	AuthKeyUpstream = "upstream"

	// StorePath is where the registry keeps its blobs on a node.
	//
	// A module constant rather than a user setting: the path is part of the
	// contract with the data already on disk. The legacy implementation writes
	// under StorePath/local_data, which matters when adopting an existing store
	// instead of refilling it.
	StorePath = "/opt/deckhouse/registry"

	// StorePathImmutable is where it keeps them on a node whose root filesystem is
	// read-only, which is every node of an Engine cluster.
	//
	// A second constant rather than a change to the first, because the first is a
	// contract with data already on disk: on a bashible node the previous implementation
	// filled StorePath/local_data and the store adopts it rather than refetching every
	// image. There is no such data here — bashible has never run on such a node — so this
	// one is free to be a path that exists, and /var is the writable data partition.
	StorePathImmutable = "/var/lib/deckhouse/registry"

	// AgentPKIPath is where the node agent's certificate material lives on a node.
	//
	// A shared constant because it is a contract between three sides: the bashible step
	// that generates the material, the agent that serves with it, and the Deckhouse pod,
	// which mounts the authority from here to verify the agent it fetches through. The
	// authority is generated on the node and never leaves it, so it cannot travel through
	// the cluster the way a Secret would — every node has its own.
	AgentPKIPath = "/etc/kubernetes/registry-agent/pki"

	// AgentCAFile is the authority that signs what the agent serves.
	AgentCAFile = AgentPKIPath + "/ca.crt"

	// AgentCPURequest and AgentMemoryRequest are what the node agent's static pod requests.
	//
	// Requests and no limits. Without requests the pod is BestEffort, and BestEffort is the
	// smallest CPU weight on the node: under load — a node bootstrapping, a busy worker — the
	// agent got next to no CPU, and every pull on the node waited on it, the pulls that would
	// have relieved the load included. A CPU limit would throttle the same path from the other
	// side, since every byte of every image crosses the agent's TLS. Memory is left unlimited
	// until the agent's peak under parallel pulls is measured; until then the OOM killer reaches
	// it last anyway, through system-node-critical.
	//
	// Shared constants because the manifest exists three times — the bashible step, the
	// module's request for Engine nodes and dhctl's copy of it — and tests pin all of them here.
	AgentCPURequest    = "50m"
	AgentMemoryRequest = "64Mi"

	// AgentMetricsTextfile is where the node agent keeps its metrics for the node exporter.
	//
	// The directory is the node exporter's textfile directory: it mounts this host path and
	// reads every *.prom file in it (monitoring-kubernetes, node-exporter's DaemonSet, the
	// textfile volume and --collector.textfile.directory). That is a contract with another
	// module rather than something this one decides, hence one constant for the three manifests
	// that name it.
	AgentMetricsTextfile = "/var/run/node-exporter-textfile/registry-agent.prom"

	// ImageAddressConfigMapName is the record that the cluster's pull path goes through the
	// node agent: its presence says image references name HostWithPath, and its one key says
	// so in words.
	//
	// A shared constant because it has more than one writer and more than one reader. The
	// module publishes it once every node's agent applies its layout, and the installer
	// publishes it itself on a cluster whose first master comes up with the agent already
	// on it — there the answer is known before any node exists to report it. Everything that
	// has to know which registry the cluster pulls from reads it.
	ImageAddressConfigMapName = "registry-image-address"

	// ImageAddressConfigMapKey is its only key.
	ImageAddressConfigMapKey = "base"
)

var (
	ProxyHost    = fmt.Sprintf("127.0.0.1:%d", AgentPort)
	HostWithPath = fmt.Sprintf("%s/%s", Host, strings.TrimLeft(Path, "/"))

	// StorageHost is the store under the name of its Service, at the port its replicas serve on:
	// what a process with no agent to go through dials to read the store itself, with the store's
	// authority and read-only account. Two do:
	//
	//   - dhctl in the cluster on an air-gapped cluster: terraform-manager's infrastructure
	//     auto-converger and its state exporter, which run in the pod network;
	//   - registry-packages-proxy, in the host network of a master, while that master has no agent
	//     yet. That is the window in which the agent itself is installed, from a package fetched
	//     through this proxy, so it cannot wait for the agent. In practice that is the first master
	//     of an installation with the store, until its agent is installed: a master that joins
	//     later writes the agent's authority (bashible step 053) before its kubelet starts, so no
	//     proxy pod runs on it without one. It keeps this address as a literal, pinned to this one
	//     by its test.
	//
	// Host's port does not get it there. The Service is headless, so its name resolves to the
	// masters' node addresses and a client dials them at whatever port it names; the Service's
	// targetPort applies to none of that. At Host's `:5001` a node has only the agent, on its
	// loopback, and the node address refuses the connection. The serving certificate names the
	// Service, so the name verifies at this port as it did at the old one.
	//
	// Derived from StoragePort, unlike Host: this one is an address, and it moves with the
	// listener. During the update to the current ports a replica still on the previous pod spec
	// serves at LegacyStoragePort and refuses this one. A dialer moves on to the next address the
	// name resolves to, so the read fails only while no replica is on the current spec — on a
	// cluster with one replica, until it is replaced. Nothing falls back to the old port for it:
	// dhctl retries on its next run, and the proxy on the next fetch a node's bashible retries. A
	// running cluster's masters all have an agent, so the proxy reads the store this way only on
	// a fresh installation, where every replica is already on the current spec.
	StorageHost         = fmt.Sprintf("registry.d8-system.svc:%d", StoragePort)
	StorageHostWithPath = fmt.Sprintf("%s/%s", StorageHost, strings.TrimLeft(Path, "/"))

	// ProxyHostWithPath is what a client running on a node fetches through, as opposed to
	// HostWithPath, which is what an image reference is built from.
	//
	// The two name the same agent and differ only in who is asking. A container runtime is
	// handed a drop-in that redirects any registry to the agent, so its references can name
	// a service that nothing dials. A process has no such redirection and has to dial, and
	// the only address that always works is the loopback one: the agent is on every node,
	// while the Service in front of the cache exists only when there is a cache.
	ProxyHostWithPath = fmt.Sprintf("%s/%s", ProxyHost, strings.TrimLeft(Path, "/"))

	// AgentDropInFile is the one file the node agent writes for the container runtime.
	//
	// A shared constant rather than a path spelled out on each side, because it is a
	// contract between three of them: the agent writes it, a bashible step waits for it
	// as the sign that pulls can succeed, and containerd reads it. Spelled out
	// separately, a change to any one of those would be silent in the other two.
	AgentDropInFile = fmt.Sprintf("%s/%s/%s", DropInRoot, DropInDefaultHost, DropInHostsFile)

	SupportedCRI         = []CRIType{CRIContainerdV1, CRIContainerdV2}
	ModesRequiringModule = []ModeType{ModeDirect, ModeLocal}
)

// IsInCluster reports whether a repository names the in-cluster registry.
//
// This is the address everything records — pod image references, a ModuleSource, a status —
// and the one nothing dials: a container runtime is redirected to the node agent by a drop-in,
// while a process has to translate it to the loopback address first.
func IsInCluster(repository string) bool {
	return repository == Host || strings.HasPrefix(repository, Host+"/")
}

// IsLocalAgent reports whether an image repository is served by the node agent on this
// host rather than by a registry somewhere else.
//
// What makes the difference is how the connection is verified: the agent's authority is
// generated on the node and read from its filesystem, while every other registry is
// verified with what the cluster knows about it.
func IsLocalAgent(repository string) bool {
	return repository == ProxyHost || strings.HasPrefix(repository, ProxyHost+"/")
}

// NodeRegistryAddr is the address of the previous implementation's node services on a master.
func NodeRegistryAddr(addr string) string {
	return fmt.Sprintf("%s:%d/%s", addr, LegacyStoragePort, strings.TrimLeft(Path, "/"))
}

// GenerateProxyEndpoints builds the upstream list of the node load balancer from
// the InternalIP addresses of the control plane nodes and the port the registry
// behind it serves on.
//
// The port is the caller's because there are two registries a balancer can front:
// the previous implementation's node services (LegacyStoragePort), and the registry
// an installation from a bundle starts on the first master (StoragePort).
//
// net.JoinHostPort is used rather than string concatenation so that an IPv6
// address is bracketed: `fd00::1:5001` is ambiguous and would be rejected by
// both the NGINX `server` directive and the endpoint validation.
//
// The contract is narrow on purpose: an entry is a bare IP address, and nothing
// else. This function does not validate, it formats -- it joins whatever it is
// given to the port, so an input that is not an address comes out
// looking like an endpoint. Its callers take the value from
// Node.status.addresses, which kubelet fills with a bare address but which the
// API server does not check the format of, so a subject able to patch node
// status decides what arrives here. helpers.ProxyEndpoint is what stands between
// that value and `server <value>;`, and the two must agree on the shape they
// exchange: an already bracketed input such as "[0.0.0.0]" is not an address and
// must not become an acceptable endpoint by passing through here.
func GenerateProxyEndpoints(masterNodesIPs []string, port int) []string {
	proxyEndpoints := make([]string, 0, len(masterNodesIPs))
	for _, ip := range masterNodesIPs {
		proxyEndpoints = append(proxyEndpoints, net.JoinHostPort(ip, strconv.Itoa(port)))
	}
	return proxyEndpoints
}

func IsCRISupported(cri CRIType) bool {
	return slices.Contains(SupportedCRI, cri)
}

func ModuleRequired(mode ModeType) bool {
	return slices.Contains(ModesRequiringModule, mode)
}
