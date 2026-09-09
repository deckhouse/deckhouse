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
	"slices"
	"strings"
)

const (
	Port       = 5001
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
)

var (
	Host         = fmt.Sprintf("registry.d8-system.svc:%d", Port)
	ProxyHost    = fmt.Sprintf("127.0.0.1:%d", Port)
	HostWithPath = fmt.Sprintf("%s/%s", Host, strings.TrimLeft(Path, "/"))

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
	ModesRequiringModule = []ModeType{ModeDirect, ModeLocal, ModeProxy}
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

func NodeRegistryAddr(addr string) string {
	return fmt.Sprintf("%s:%d/%s", addr, Port, strings.TrimLeft(Path, "/"))
}

func GenerateProxyEndpoints(masterNodesIPs []string) []string {
	proxyEndpoints := make([]string, 0, len(masterNodesIPs))
	for _, ip := range masterNodesIPs {
		proxyEndpoints = append(proxyEndpoints, fmt.Sprintf("%s:%d", ip, Port))
	}
	return proxyEndpoints
}

func IsCRISupported(cri CRIType) bool {
	return slices.Contains(SupportedCRI, cri)
}

func ModuleRequired(mode ModeType) bool {
	return slices.Contains(ModesRequiringModule, mode)
}
